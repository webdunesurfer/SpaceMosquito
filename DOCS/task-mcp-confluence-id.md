# Task: Align MCP Search and `confluence_get_page` on Confluence ID

## Objective

Fix the MCP workflow break where **`confluence_search` returns an internal UUID** (`pages.id`) but **`confluence_get_page` expects a Confluence integer ID** (`pages.confluence_id`). Agents must be able to search, then fetch the full page using **the same Confluence-native identifier** — without bridging two ID systems.

**Canonical ID for MCP:** `(space_key, confluence_id)` — matches Confluence URLs, REST API, and how users refer to pages.

**Out of scope for this task:**

- Search excerpt quality (issue #2 in `DOCS/issues/mcp-issues.md`)
- `confluence_list_space` pagination or titles-only mode (issues #3–#4)
- `confluence_get_page_by_title` (issue #5)
- Slimming `confluence_get_page` response payload (content cleanup / MCP ergonomics — separate task)
- **`POST /api/search/reindex`** — renamed to `confluence_id` + `space_key` (see implementation)
- Docker Compose, live MCP SSE e2e, or browser-based tests

---

## Problem Summary

| Tool | ID today | Type | Meaning |
|------|----------|------|---------|
| `confluence_search` | `PageID` (JSON) | `uuid.UUID` | Internal Postgres `pages.id` — **wrong field for agents** |
| `confluence_get_page` | `page_id` (arg) | `integer` | Confluence `pages.confluence_id` — **correct semantics, wrong name** |

Search SQL aliases the internal primary key:

```sql
SELECT p.id AS page_id, ...
```

`confluence_get_page` calls `GetPage(spaceKey, int(page_id))` on `p.confluence_id`.

**Root cause:** search exposes an implementation detail (DB row UUID) instead of the Confluence page ID agents already know from URLs (`/pages/12345`).

**Schema note:** `confluence_id` is unique per space (`UNIQUE(space_id, confluence_id)`), so `space_key` is always required alongside it.

**Reported impact:** Critical blocker for MCP agents (see `DOCS/issues/mcp-issues.md` issue #1).

---

## Target Behavior

After this task:

1. **One primary ID for agents:** Confluence integer `confluence_id` + `space_key`.
2. **`confluence_search` returns a streamlined result** with snake_case JSON; **`confluence_id` is the fetch key**.
3. **`confluence_get_page` takes `confluence_id` (integer)** — same field returned by search.
4. **Search → get page works in one hop:**

   ```
   search → { "confluence_id": 12345, "space_key": "PROJ", … }
   get_page({ "space_key": "PROJ", "confluence_id": 12345 }) → full page
   ```

5. **Internal UUID is not part of the agent API** (optional `internal_id` on search for debugging only, omitted by default).

**Breaking change is acceptable.** Drop misleading `page_id` naming, PascalCase JSON, and UUID-as-page-id entirely.

---

## Streamlined API Shape

### Design principles

| Principle | Decision |
|-----------|----------|
| Canonical ID | **`confluence_id`** (integer) + **`space_key`** — matches Confluence URLs and REST API |
| JSON field names | **snake_case** via `json` struct tags on MCP-facing types |
| Internal UUID | **Not exposed** to agents by default; optional `internal_id` on search hits for ops/debug |
| `page_id` param | **Removed** from MCP tools — was ambiguous (meant Confluence id, sounded like internal id) |

### `confluence_search` result

Introduce an MCP response DTO — do not marshal untagged `db.SearchResult` directly:

```go
type SearchHit struct {
    ConfluenceID int     `json:"confluence_id"`
    SpaceKey     string  `json:"space_key"`
    Title        string  `json:"title"`
    Excerpt      string  `json:"excerpt"`
    Similarity   float64 `json:"similarity"`
    FilePath     string  `json:"file_path,omitempty"`
    InternalID   string  `json:"internal_id,omitempty"` // only when mcp.expose_internal_ids: true
}
```

Example response:

```json
{
  "confluence_id": 12345,
  "space_key": "PROJ",
  "title": "Invoice Filtering Logics",
  "excerpt": "…",
  "similarity": 0.42,
  "file_path": "saved/PROJ/Invoice…/index.html"
}
```

DB change in `SearchPages`:

```sql
SELECT p.confluence_id, s.key AS space_key, p.title,
       LEFT(p.content, 200) AS excerpt,
       ts_rank(...) AS similarity,
       p.html_path AS file_path
FROM pages p ...
```

Stop selecting `p.id` for MCP search results (keep in DB layer only if needed for `internal_id` debug mode).

### `confluence_get_page` input schema

**Single lookup path** — Confluence ID from search or URL:

```json
{
  "type": "object",
  "properties": {
    "space_key": {
      "type": "string",
      "description": "Space key"
    },
    "confluence_id": {
      "type": "integer",
      "description": "Confluence page ID (from search or /pages/{id} URL)"
    }
  },
  "required": ["space_key", "confluence_id"]
}
```

Validation rules in `toolGetPage`:

| Case | Result |
|------|--------|
| `space_key` + `confluence_id` | `GetPage(ctx, spaceKey, confluenceID)` — existing DB method |
| Missing `space_key` or `confluence_id` | Error: both required |
| Non-integer `confluence_id` | Error: invalid `confluence_id` |
| Not found | Error: page not found |

No dual lookup (`page_id` vs `confluence_id`). No `GetPageByID` needed for this task.

### `confluence_get_page` response (`PageDetail`)

Tagged DTO — snake_case, agent-oriented (no filesystem paths unless needed later):

```go
type PageDetail struct {
    ConfluenceID int       `json:"confluence_id"`
    SpaceKey     string    `json:"space_key"`
    Title        string    `json:"title"`
    Version      int       `json:"version"`
    Content      string    `json:"content"`
    UpdatedAt    time.Time `json:"updated_at"`
    InternalID   string    `json:"internal_id,omitempty"` // when mcp.expose_internal_ids
}
```

Map from `db.Page` + space key in shared package. Omit `html_path`, `raw_html_path`, `file_dir`, and internal UUIDs by default.

---

## REST vs MCP — why both change

Both surfaces read the same database but serve different clients and wrap results differently.

| | **MCP** (`confluence_search`) | **REST** (`GET /api/search`) |
|---|-------------------------------|------------------------------|
| **Transport** | JSON-RPC over SSE (`GET /mcp`) + POST to `/mcp/session/{id}` | Plain HTTP `GET /api/search?q=…&space=…` |
| **Consumer** | AI agents (Cursor, Claude Desktop, etc.) | curl, browser extensions, scripts |
| **Response envelope** | MCP tool result: `{ content: [{ type: "text", text: "<json>" }] }` | `{ query, count, results: [...] }` |
| **Code path** | `internal/mcp/server.go` → `toolSearch` | `internal/api/search.go` → `SearchHandler.Search` |
| **Shared DB call** | `db.SearchPages(...)` | `db.SearchPages(...)` |

**Why not MCP-only?** If only MCP maps to `SearchHit`, REST would keep returning PascalCase `PageID` (internal UUID) while MCP returns `confluence_id` — same backend, inconsistent contracts. CLI `search` would also stay opaque (today it prints title only, no ID at all).

**Decision:** Same PR, shared `SearchHit` / `PageDetail` mappers used by MCP, REST `results`, and CLI output.

---

## Implementation Plan

### Phase 1 — Database layer

**File:** `internal/db/models.go`

| Change | Detail |
|--------|--------|
| `SearchResult` | Replace `PageID uuid.UUID` with `ConfluenceID int`; add `json` tags or map to `SearchHit` in MCP |
| `SearchPages` | Select `p.confluence_id` instead of `p.id AS page_id` |

`GetPage(ctx, spaceKey, confluenceID)` — **unchanged**; already the correct lookup.

No migration file needed — schema unchanged.

### Phase 2 — MCP tool layer

**Files:** `internal/mcp/server.go`, `internal/mcp/types.go` (new, optional)

| Change | Detail |
|--------|--------|
| `SearchHit` / `PageDetail` | Tagged DTOs in shared package (`internal/search` or similar) |
| `handleToolsList` | `confluence_get_page`: require `space_key` + `confluence_id`; remove `page_id` |
| `toolGetPage` | Parse `confluence_id` (JSON number → `int`); call `GetPage`; return `PageDetail` (not raw `db.Page`) |
| `toolSearch` | Map via shared `ToSearchHits(results, cfg.MCP.ExposeInternalIDs)` |
| `MCPConfig` | Add `ExposeInternalIDs bool` — gates `internal_id` on search hits |
| Tool descriptions | Document search → get_page using `confluence_id` |

**Refactor for testability (recommended):**

```go
func parseGetPageArgs(args map[string]interface{}) (spaceKey string, confluenceID int, err error)
```

Optional small interface for tests:

```go
type pageStore interface {
    GetPage(ctx context.Context, spaceKey string, confluenceID int) (*db.Page, error)
}
```

`*db.DB` satisfies this; tests inject a fake.

### Phase 3 — REST / CLI alignment (same PR)

All three consumers call `db.SearchPages` today. Use a **shared mapper** (e.g. `internal/search/dto.go`) so MCP, REST, and CLI expose the same `SearchHit` / `PageDetail` shapes. See [REST vs MCP](#rest-vs-mcp-why-both-change) below.

| Area | Change |
|------|--------|
| REST `GET /api/search` | **Approved:** map `results` through shared `ToSearchHits(...)`; each hit has `confluence_id` (not internal UUID). Keep outer envelope `{ query, count, results }` |
| REST `POST /api/search/reindex` | **Done (follow-up):** `?space_key=<key>&confluence_id=<int>` for single page; bulk unchanged |
| CLI `search` command | Print `confluence_id` per hit |
| Browser extensions | Update `searchPages` types in `chrome-extension/lib/api.ts` and `firefox-extension/lib/api.ts` to match `SearchHit` (`confluence_id`, wrapped `{ results }` if needed) |
| `config.yaml` | `mcp.expose_internal_ids: false` (default) — when true, populate `internal_id` on search hits |

### Phase 4 — Documentation

| File | Update |
|------|--------|
| `DOCS/issues/mcp-issues.md` | Mark issue #1 resolved / link this task |
| `DEVELOPMENT.md` or MCP docs | Document search → get_page flow with example JSON |

---

## Testing Strategy (No Docker Required)

**Yes — this task can be fully verified without Docker.** Simpler than a dual-ID design: one parse path, one DB method.

### What can be unit-tested without a database

| Layer | Test file | What to verify |
|-------|-----------|----------------|
| **Arg parsing** | `internal/mcp/page_lookup_test.go` (new) | `parseGetPageArgs`: valid pair, missing `space_key`, missing `confluence_id`, non-integer id |
| **Search mapping** | `internal/mcp/server_test.go` | `SearchHit` JSON is snake_case; includes `confluence_id`; no `PageID` / UUID in output |
| **MCP tool dispatch** | `internal/mcp/server_test.go` (extend) | `toolGetPage` with fake `pageStore` calls `GetPage(space, id)`; errors as `isError` |
| **Schema / tools list** | `internal/mcp/server_test.go` | `confluence_get_page` schema requires `confluence_id`, not `page_id` |

Example table-driven cases for `parseGetPageArgs`:

| `space_key` | `confluence_id` | Expected |
|-------------|-----------------|----------|
| `"PROJ"` | `12345` | OK |
| `""` | `12345` | Error |
| `"PROJ"` | missing | Error |
| `"PROJ"` | `"abc"` | Error |

Fake store:

```go
type fakePageStore struct {
    getPage func(ctx context.Context, spaceKey string, id int) (*db.Page, error)
}
```

### What cannot be tested without Postgres (defer)

| Layer | Reason |
|-------|--------|
| `db.SearchPages` selecting `confluence_id` | Requires live `pages` + FTS data |

Verify SQL manually or via optional `//go:build integration` + `TEST_DATABASE_URL` later.

### Manual smoke test (optional)

```bash
cd space-mosquito
go build ./cmd/server
# MCP search → get_page with confluence_id from result
```

Use `timeout` for SSE MCP calls per `AGENTS.md`.

---

## Suggested Implementation Order

1. Add `parseGetPageArgs` + table tests (TDD)
2. Update `SearchPages` SQL and `SearchResult` struct
3. Add `SearchHit` + wire `toolSearch` mapping
4. Update `toolGetPage` + tool schema (`confluence_id` required)
5. Align REST/CLI if in scope; update docs; `go test -race ./...`

---

## Acceptance Criteria

- [ ] `confluence_search` returns snake_case JSON with **`confluence_id`** as the page identifier
- [ ] Search results do **not** expose internal UUID as `page_id` / `PageID`
- [ ] `confluence_get_page` requires **`space_key`** + **`confluence_id`** (integer)
- [ ] `page_id` parameter is **removed** from MCP tool schema
- [ ] Agent workflow search → get_page works using `confluence_id` from search results
- [ ] `go test -race ./...` passes **without Docker or a running database**
- [ ] New tests cover argument parsing, search JSON shape, and MCP dispatch (fake store)
- [ ] `confluence_get_page` returns tagged `PageDetail` (snake_case), not raw `db.Page`
- [ ] REST `GET /api/search` `results[]` use `SearchHit` with **`confluence_id`** (approved breaking change)
- [ ] `POST /api/search/reindex` single-page mode uses **`space_key`** + **`confluence_id`**
- [ ] CLI search and MCP use the same `SearchHit` mapper as REST `GET /api/search`
- [ ] `mcp.expose_internal_ids` defaults to `false`; when true, `internal_id` appears on hits
- [ ] No unit tests required for `expose_internal_ids: true` behavior
- [ ] `DOCS/issues/mcp-issues.md` issue #1 annotated as addressed

---

## Resolved Decisions

| # | Question | Decision |
|---|----------|----------|
| 1 | `confluence_get_page` response shape | **Tagged `PageDetail` DTO** — snake_case, content + metadata; omit filesystem paths |
| 2 | `internal_id` on search | **`mcp.expose_internal_ids` config flag** (default `false`). No extra unit tests when flag is on |
| 3 | DB integration tests | **Follow-up task** — `DOCS/task-db-integration-tests.md` |
| 4 | REST vs MCP scope | **Both in same PR** via shared mapper — see [REST vs MCP](#rest-vs-mcp-why-both-change) |
| 5 | `POST /api/search/reindex` | **`confluence_id` + `space_key`** query params (renamed from `page_id` + `space`) |
| 6 | `GET /api/search` | **Approved** — return `confluence_id` in `results[]` via `SearchHit`; remove internal UUID from response |

### Config addition

```yaml
mcp:
  port: 8081
  expose_internal_ids: false   # when true, search hits include internal_id (pages.id UUID)
```

---

## Related Documents

| Doc | Relationship |
|-----|--------------|
| `DOCS/issues/mcp-issues.md` | Source issue #1 |
| `DOCS/task-go-unit-tests.md` | Unit-test conventions; MCP fake-store pattern |
| `DOCS/task-db-integration-tests.md` | Postgres integration tests (follow-up) |
| `DOCS/task-dockerless-migrations.md` | Future: SQLite may simplify integration tests without Docker |
