# Task: `confluence_list_space` Pagination

## Objective

Add **cursor-based pagination** to MCP `confluence_list_space` so agents can walk large spaces page-by-page without requesting an enormous `limit` and overflowing the context window.

**Source:** `DOCS/issues/mcp-issues.md` issue #3 (high priority).

**Related (separate task):** issue #4 — `confluence_list_space` returns full `Content` per row. This task adds pagination only; page rows remain full `Page` payloads until #4.

**In scope for this task:**

- REST `GET /api/spaces/{key}/pages?limit=&after_confluence_id=` returning the same `ListSpaceResult` shape

**Out of scope for this task:**

- Titles-only / lightweight list mode (issue #4)
- `confluence_get_page_by_title` (issue #5)
- Changes to `ListPages` callers in cron / `spaces.go` status counts (internal; keep `after_confluence_id = nil`)

---

## Problem Summary

| Today | Impact |
|-------|--------|
| MCP schema: `space_key` + `limit` only (default **50**) | First N pages by `confluence_id` only |
| `db.ListPages`: `ORDER BY p.confluence_id LIMIT $2` | No `OFFSET` or cursor |
| Returns raw `[]db.Page` JSON | Large payloads when `limit` is raised |

An agent that needs page 200 must either guess Confluence IDs or set `limit=200+`, which returns **full `Page` structs including `content`** — multi-megabyte JSON.

```206:207:space-mosquito/internal/db/models.go
		 ORDER BY p.confluence_id
		 LIMIT $2`,
```

```384:393:space-mosquito/internal/mcp/server.go
func (s *Server) toolListSpace(args map[string]interface{}) (interface{}, error) {
	// ...
	return s.db.ListPages(context.Background(), spaceKey, limit)
}
```

---

## Target Behavior

After this task, an agent can list a large space in chunks:

```
1. list_space({ space_key: "PROJ", limit: 50 })
   → pages 1–50, has_more: true, next_after_confluence_id: 12350

2. list_space({ space_key: "PROJ", limit: 50, after_confluence_id: 12350 })
   → pages with confluence_id > 12350, next cursor or has_more: false
```

Requirements:

1. **Stable order** — `ORDER BY p.confluence_id ASC` (unchanged).
2. **Cursor** — `after_confluence_id` (integer, exclusive): return rows where `p.confluence_id > after_confluence_id`.
3. **Response metadata** — `has_more` and `next_after_confluence_id` so agents know how to continue.
4. **Sane defaults** — `limit` default 50; enforce a **max limit** (e.g. 200).
5. **Breaking change acceptable** — wrap list result in a structured object (not a bare array).

---

## Recommended Solution: `after_confluence_id` cursor

Prefer **keyset / cursor pagination** over `OFFSET`:

| | `after_confluence_id` | `offset` |
|---|----------------------|----------|
| Performance on large spaces | Good — uses index on `(space_id, confluence_id)` | Degrades — scans skipped rows |
| Stable under inserts | New pages append at end; cursor semantics clear | Offsets shift when data changes |
| Matches existing sort | Yes — already `ORDER BY confluence_id` | Yes |
| Agent ergonomics | Pass last seen ID from previous response | Pass incrementing offset |

### MCP tool input schema

```json
{
  "type": "object",
  "properties": {
    "space_key": { "type": "string", "description": "Space key" },
    "limit": { "type": "integer", "description": "Max results per page (default 50, max 200)" },
    "after_confluence_id": {
      "type": "integer",
      "description": "Return pages with Confluence ID greater than this (exclusive). Omit for first page."
    }
  },
  "required": ["space_key"]
}
```

### MCP tool response (`ListSpaceResult`)

Wrap results in a structured object (breaking change from bare `[]Page` array). Until issue #4, **`pages[]` still carries full page fields** (`content`, paths, etc.) — same data as today, plus pagination metadata.

```go
type ListSpacePage struct {
    ConfluenceID       int       `json:"confluence_id"`
    Title              string    `json:"title"`
    Version            int       `json:"version,omitempty"`
    ParentConfluenceID *int      `json:"parent_confluence_id,omitempty"`
    Content            string    `json:"content,omitempty"`
    HTMLPath           string    `json:"html_path,omitempty"`
    RawHTMLPath        string    `json:"raw_html_path,omitempty"`
    MetadataPath       string    `json:"metadata_path,omitempty"`
    FileDir            string    `json:"file_dir,omitempty"`
    CreatedAt          time.Time `json:"created_at,omitempty"`
    UpdatedAt          time.Time `json:"updated_at,omitempty"`
    // omit internal UUIDs unless mcp.expose_internal_ids
}

type ListSpaceResult struct {
    SpaceKey              string          `json:"space_key"`
    Pages                 []ListSpacePage `json:"pages"`
    Count                 int             `json:"count"`
    HasMore               bool            `json:"has_more"`
    NextAfterConfluenceID *int            `json:"next_after_confluence_id,omitempty"`
}
```

Issue #4 will slim `ListSpacePage` to summary fields only; this task only adds the wrapper + cursor.

`has_more` detection: request `limit + 1` rows from DB; if more than `limit` returned, trim to `limit` and set `has_more = true`. `next_after_confluence_id` = last row’s `confluence_id` when `has_more`.

### Database (`internal/db/models.go`)

Extend signature:

```go
func (d *DB) ListPages(ctx context.Context, spaceKey string, limit int, afterConfluenceID *int) ([]Page, error)
```

SQL (conceptual):

```sql
SELECT ...
FROM pages p
JOIN spaces s ON s.id = p.space_id
WHERE s.key = $1
  AND ($2::int IS NULL OR p.confluence_id > $2)
ORDER BY p.confluence_id ASC
LIMIT $3
```

Update internal callers with `afterConfluenceID: nil`:

- `internal/mcp/server.go` — `toolListSpace`
- `internal/api/spaces.go` — `getStatus` (count via `len(pages)` when `limit=0` → keep behavior: pass `nil` cursor, existing default limit 100)
- `internal/cron/scheduler.go` — full list with `limit=0`

Optional: add `ListPageSummaries` that selects only needed columns (skips `content`) — **deferred to issue #4**.

### Database migrations

**None required.** Pagination is a query-only change: add `AND p.confluence_id > $cursor` to existing `ListPages` SQL. No new columns or tables.

**Index coverage (existing):**

- `UNIQUE(space_id, confluence_id)` on `pages` — btree supports `WHERE space_id = ? AND confluence_id > ? ORDER BY confluence_id` efficiently once the space row is resolved.
- `idx_pages_space` / `idx_pages_space_id` — `space_id` lookups.

No new migration unless profiling shows a need for a differently ordered composite index (unlikely given the unique constraint).

---

## Implementation Plan

### Phase 1 — DB layer

| Change | Detail |
|--------|--------|
| `ListPages` | Add optional `afterConfluenceID *int`; append `AND p.confluence_id > $n` when set |
| `fetchLimit+1` pattern | Implement in MCP layer or DB helper for `has_more` |

### Phase 2 — MCP + REST

| Change | Detail |
|--------|--------|
| `parseListSpaceArgs` | `space_key`, `limit`, `after_confluence_id`; validate max limit **200** |
| `toolListSpace` | Return `ListSpaceResult`; map `db.Page` → `ListSpacePage` |
| `handleToolsList` | Update `confluence_list_space` schema + description (mention cursor) |
| REST | `GET /api/spaces/{key}/pages?limit=&after_confluence_id=` → same `ListSpaceResult` JSON |
| `pageStore` interface | Extend fake for pagination tests if needed |

### Phase 3 — Tests

| Layer | File | Cases |
|-------|------|-------|
| Arg parsing | `internal/mcp/list_space_test.go` | valid cursor, missing space_key, limit cap, bad types |
| Cursor logic | `internal/mcp/list_space_test.go` or `internal/search` | `has_more` / `next_after_confluence_id` from mock page list |
| Integration | `DOCS/task-db-integration-tests.md` | Seed 5 pages, page with `after_confluence_id`, assert order and exclusivity |

### Phase 4 — Docs

| File | Update |
|------|--------|
| `DOCS/issues/mcp-issues.md` | Mark #3 fixed when done |
| `DEVELOPMENT.md` | Example paginated list flow (optional) |

---

## Testing Strategy (no Docker for merge)

- **Unit tests** for `parseListSpaceArgs` and `buildListSpaceResult` (pure Go, fake rows).
- **Integration tests** optional via `TEST_DATABASE_URL` + `//go:build integration`.
- Default CI: `go test -race ./...` unchanged.

---

## Suggested Implementation Order

1. `parseListSpaceArgs` + `buildListSpaceResult` + unit tests (TDD)
2. Extend `ListPages` SQL with `after_confluence_id`
3. Wire `toolListSpace` + MCP schema
4. Update internal `ListPages` call sites (`nil` cursor)
5. Docs + manual MCP smoke on a space with >50 pages
6. REST handler + route registration + API test

---

## Resolved Decisions

| # | Question | Decision |
|---|----------|----------|
| 1 | Ship with issue #4? | **No** — pagination only; full `content` per row remains until #4 |
| 2 | Max `limit` | **200** (hard cap) |
| 3 | Response shape | **`ListSpaceResult` wrapper** |
| 4 | `after_confluence_id = 0` | **Omit param for first page**; if `0` passed, treat as no cursor |
| 5 | `total_pages` | **Omit** |
| 6 | Reverse paging | **Omit** |
| 7 | REST endpoint | **Yes** — `GET /api/spaces/{key}/pages` |
| — | DB migrations | **None** — query-only; existing `UNIQUE(space_id, confluence_id)` covers cursor scans |

---

## Open Questions

_None — ready to implement._

---

## Acceptance Criteria

- [x] `confluence_list_space` accepts optional `after_confluence_id`
- [x] Results ordered by `confluence_id` ascending; cursor is exclusive (`>`)
- [x] Response includes `has_more` and `next_after_confluence_id` when applicable
- [x] Default `limit` 50; max limit enforced (200)
- [x] Agent can list all pages in a 500+ page space via repeated calls without raising `limit` above max
- [x] `go test -race ./...` passes without Postgres
- [x] Unit tests for arg parsing and result builder
- [x] `GET /api/spaces/{key}/pages` returns `ListSpaceResult` with same pagination semantics
- [x] `DOCS/issues/mcp-issues.md` issue #3 marked addressed

---

## Related Documents

| Doc | Relationship |
|-----|--------------|
| `DOCS/issues/mcp-issues.md` | Source issue #3; issue #4 list payload size |
| `DOCS/task-mcp-confluence-id.md` | `confluence_id` as page identifier |
| `DOCS/task-mcp-search-excerpts.md` | Explicitly deferred list_space work |
| `DOCS/task-db-integration-tests.md` | `ListPages` cursor integration tests |
