# Task: `confluence_list_space` Lightweight Rows (No Content by Default)

## Objective

Make **`confluence_list_space` and `GET /api/spaces/{key}/pages` return summary rows by default** — enough to browse a space and call `confluence_get_page` for full text, without shipping megabytes of `content` per list call.

**Source:** `DOCS/issues/mcp-issues.md` issue #4 (high priority).

**Prerequisite:** issue #3 (pagination) — **done** (`DOCS/task-mcp-list-space-pagination.md`). Agents can already page through spaces; this task fixes per-row payload size.

**Out of scope for this task:**

- Pagination changes (cursor, `ListSpaceResult` wrapper — already shipped)
- `confluence_get_page_by_title` (issue #5)
- Content cleanup on `confluence_get_page` response (separate ergonomics task)
- Changes to internal `db.ListPages` callers (cron incremental scan, space status counts — still need full rows)
- Browser extension client changes (`chrome-extension`, `firefox-extension`) — extensions do not call list-pages today; no client updates in this task
- Docker Compose e2e or live MCP SSE tests

---

## Problem Summary

Pagination (#3) limits how many rows are returned per call, but **each row still includes the full page body and internal file paths**.

| Today | Impact |
|-------|--------|
| `ListSpacePage` includes `content`, `html_path`, `raw_html_path`, `metadata_path`, `file_dir` | ~3 KB+ per page typical; 200 pages × ~3 KB ≈ **600 KB–1.5 MB** per list page |
| `db.ListPages` always `SELECT … content, html_path, …` | DB reads and transfers full TEXT for every list row |
| Agent workflow | List is for **discovery**; `confluence_get_page` is for **reading** — list duplicates get_page's job |

```82:100:space-mosquito/internal/search/list_space.go
func ToListSpacePage(page *db.Page, exposeInternalIDs bool) ListSpacePage {
	row := ListSpacePage{
		// ...
		Content:            page.Content,
		HTMLPath:           page.HTMLPath,
		RawHTMLPath:        page.RawHTMLPath,
		MetadataPath:       page.MetadataPath,
		FileDir:            page.FileDir,
		// ...
	}
```

**Reported impact:** For a 500-page space, listing with a high limit produced ~1.5 MB JSON — mostly unused `content` the agent did not need to triage titles.

**Consumers affected (same code path):**

- MCP `confluence_list_space` → `ListSpaceResult.pages[]`
- REST `GET /api/spaces/{key}/pages` → same JSON shape

---

## Target Behavior

After this task:

1. **Default list rows are summaries** — `confluence_id`, `title`, `version`, `parent_confluence_id`, `created_at`, `updated_at` (and optional `internal_id` when configured).
2. **No `content` in default responses** — agents use `confluence_get_page` for body text.
3. **No internal storage paths** in list responses (`html_path`, `raw_html_path`, `metadata_path`, `file_dir`) — not useful to MCP agents; mirrors `PageDetail` which already omits paths.
4. **Optional opt-in for bulk export** — `include_content: true` adds `content` to each row when explicitly requested (REST query param + MCP tool arg).
5. **DB reads match response** — summary lists do not `SELECT content` or path columns (new query or mode).
6. **Pagination unchanged** — `after_confluence_id`, `has_more`, `next_after_confluence_id` behavior stays as in #3.

Example default response:

```json
{
  "space_key": "PROJ",
  "pages": [
    {
      "confluence_id": 12345,
      "title": "Invoice Filtering Logics",
      "version": 3,
      "parent_confluence_id": 10000,
      "created_at": "2025-01-15T08:00:00Z",
      "updated_at": "2026-06-01T12:00:00Z"
    }
  ],
  "count": 1,
  "has_more": true,
  "next_after_confluence_id": 12345
}
```

Agent workflow:

```
list_space({ space_key: "PROJ", limit: 50 })
  → skim titles
get_page({ space_key: "PROJ", confluence_id: 12345 })
  → read full content for chosen page
```

---

## Recommended Solution

### Default summary, opt-in `include_content`

Prefer **secure-by-default / lean-by-default** over a `titles_only` flag that defaults to false:

| Approach | Verdict |
|----------|---------|
| `titles_only: true` opt-in | Agents must remember to set it; easy to blow context window |
| **`include_content: false` default** (opt-in `true`) | **Chosen** — matches issue priority table (“no Content **by default**”); list stays safe |
| `fields` array | Flexible but harder for MCP tool schema and agents |

When `include_content: true`, enforce a **lower max limit** than summary mode:

| Mode | Max `limit` |
|------|-------------|
| Default (summary) | **200** (unchanged from #3) |
| `include_content: true` | **50** (hard cap) |

Constants: `ListSpaceMaxLimit = 200`, `ListSpaceMaxLimitWithContent = 50`. Apply in `ClampListSpaceLimit(limit, includeContent bool)`.

### Slim `ListSpacePage` DTO

Update `internal/search/list_space.go`:

```go
type ListSpacePage struct {
    ConfluenceID       int       `json:"confluence_id"`
    Title              string    `json:"title"`
    Version            int       `json:"version,omitempty"`
    ParentConfluenceID *int      `json:"parent_confluence_id,omitempty"`
    CreatedAt          time.Time `json:"created_at,omitempty"`
    UpdatedAt          time.Time `json:"updated_at,omitempty"`
    Content            string    `json:"content,omitempty"` // only when include_content
    InternalID         string    `json:"internal_id,omitempty"`
}
```

Remove path fields from the list DTO entirely (breaking change from current post-#3 shape).

### MCP tool input

Add to existing `confluence_list_space` schema:

```json
{
  "include_content": {
    "type": "boolean",
    "description": "Include full page content in each row (default false). Prefer confluence_get_page for reading."
  }
}
```

### REST query param

```
GET /api/spaces/{key}/pages?limit=50&include_content=true
```

Parse via extended `ParseListSpaceQuery` or sibling `ParseListSpaceOptions`.

### Database layer

Add a summary query — do **not** overload internal `ListPages` used by cron:

```go
// ListPageSummaries returns lightweight rows for MCP/REST listing.
func (d *DB) ListPageSummaries(ctx context.Context, spaceKey string, limit int, afterConfluenceID *int) ([]PageSummary, error)
```

```sql
SELECT p.id, p.confluence_id, p.version, p.title, p.parent_confluence_id, p.created_at, p.updated_at
FROM pages p
JOIN spaces s ON s.id = p.space_id
WHERE s.key = $1
  AND ($2::int IS NULL OR p.confluence_id > $2)
ORDER BY p.confluence_id ASC
LIMIT $3
```

`PageSummary` struct in `internal/db` (or reuse slim fields on a new type).

When `include_content: true`, either:

- **Option A (recommended):** call existing `ListPages` (full SELECT) — rare path, simpler
- **Option B:** single query with optional content column — one code path, more complex SQL

Wire MCP `toolListSpace` and REST `SpacePagesHandler` to pick query based on flag.

### Mapping helpers

```go
func ToListSpacePageSummary(s db.PageSummary, exposeInternalIDs bool) ListSpacePage
func ToListSpacePageFull(page *db.Page, exposeInternalIDs bool) ListSpacePage // include_content path
func BuildListSpaceResult(...) // accept []PageSummary or interface; or two builders
```

Keep `BuildListSpaceResult` signature stable where possible; add `includeContent bool` parameter.

---

## Implementation Plan

### Phase 1 — DB + types

| Change | Detail |
|--------|--------|
| `PageSummary` type | `id`, `confluence_id`, `version`, `title`, `parent_confluence_id`, `created_at`, `updated_at` |
| `ListPageSummaries` | Cursor pagination same as `ListPages`; no `content`/paths in SELECT |
| `ListPages` | Unchanged for cron / status |

### Phase 2 — Search DTO layer

| Change | Detail |
|--------|--------|
| Slim `ListSpacePage` | Drop path fields; `content` only when requested |
| `ToListSpacePage` | Split summary vs full mappers |
| `BuildListSpaceResult` | Summary path by default |
| `ParseListSpaceQuery` | Add `include_content` bool parsing |

### Phase 3 — MCP + REST

| Change | Detail |
|--------|--------|
| `parseListSpaceArgs` | `include_content` bool, default `false`; cap limit at 50 when true |
| `toolListSpace` | `ListPageSummaries` unless `include_content` |
| MCP schema | Document `include_content` and lower max limit when set |
| `SpacePagesHandler` | `?include_content=true`; same defaults as MCP |

### Phase 4 — Tests

| Layer | Cases |
|-------|-------|
| `internal/search` | Summary row JSON omits `content`/paths; `include_content` includes body |
| `internal/mcp` | Arg parsing default false; true selects full path; limit capped at 50 |
| `internal/api` | Query param validation; `include_content` cap |
| Integration (optional) | `ListPageSummaries` order + cursor via `TEST_DATABASE_URL` |

### Phase 5 — Docs

| File | Update |
|------|--------|
| `DOCS/issues/mcp-issues.md` | Mark #4 fixed |
| `DOCS/task-mcp-list-space-pagination.md` | Cross-link (already notes #4 as follow-up) |

---

## Testing Strategy (no Docker for merge)

- **Unit tests** for DTO mapping, arg/query parsing, `BuildListSpaceResult` with summaries.
- **JSON assertions** — default marshal must not contain `"content"` or `"html_path"`.
- Default CI: `go test -race ./...` unchanged.

---

## Suggested Implementation Order

1. `PageSummary` + `ListPageSummaries` SQL
2. Slim `ListSpacePage` + `ToListSpacePageSummary` + tests (TDD)
3. `include_content` flag in MCP/REST parsers
4. Wire handlers; full-content path uses `ListPages` when flag set
5. Docs + manual smoke: list 50 pages → small JSON; one `get_page` for content

---

## Resolved Decisions

| # | Question | Decision |
|---|----------|----------|
| 1 | `include_content` at all? | **Yes** — opt-in `include_content: true` remains for bulk export |
| 2 | Cap when `include_content=true`? | **Yes** — max limit **50** (vs 200 for summaries) |
| 3 | `created_at` in summary? | **Include** |
| 4 | REST default | **Same as MCP** — `include_content=false` |
| 5 | Extension clients | **No changes** — extensions don't use list-pages API today; backend-only task |

---

## Open Questions

_None — ready to implement._

## Acceptance Criteria

- [x] Default `confluence_list_space` / `GET /api/spaces/{key}/pages` rows omit `content` and storage paths
- [x] Summary rows include `confluence_id`, `title`, `version`, `parent_confluence_id`, `created_at`, `updated_at`
- [x] `include_content: true` (MCP) / `?include_content=true` (REST) restores full `content` per row
- [x] `include_content` mode caps `limit` at **50**; summary mode max **200**
- [x] Summary list uses DB query that does not `SELECT content` or path columns
- [x] Pagination semantics unchanged (`after_confluence_id`, `has_more`, `next_after_confluence_id`)
- [x] Internal `ListPages` callers (cron, space status) unchanged
- [x] `go test -race ./...` passes without Postgres
- [x] `DOCS/issues/mcp-issues.md` issue #4 marked addressed

---

## Related Documents

| Doc | Relationship |
|-----|--------------|
| `DOCS/issues/mcp-issues.md` | Source issue #4 |
| `DOCS/task-mcp-list-space-pagination.md` | Prerequisite (#3); deferred slim rows to this task |
| `DOCS/task-mcp-confluence-id.md` | `confluence_id` + `get_page` workflow after list |
| `DOCS/task-db-integration-tests.md` | Optional `ListPageSummaries` integration tests |
