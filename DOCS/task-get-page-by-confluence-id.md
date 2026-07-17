# Task: Get Page by `confluence_id` (Optional `space_key`)

## Objective

Add a **first-class “get page by Confluence ID”** path across **REST**, **MCP**, and **CLI**. **`space_key` is optional**: callers can fetch with just `confluence_id` when the ID is unambiguous in the local catalog.

**Resolution rule:**

| `space_key` | Matches in DB | Result |
|-------------|---------------|--------|
| provided | 0 | `404` not found |
| provided | 1 | `200` page |
| omitted | 0 | `404` not found |
| omitted | 1 | `200` page (most common case) |
| omitted | 2+ | `409` ambiguous — caller must pass `space_key` |

This closes the gap where agents and humans must know both keys, or work around missing REST/CLI with list+filter.

**Parent references:**

- `DOCS/task-mcp-confluence-id.md` — canonical `(space_key, confluence_id)` workflow (this task relaxes the *required* part of `space_key`)
- `DOCS/task-mcp-list-space-summaries.md` — list for discovery; get-page for reading
- `DOCS/task-server-integration-tests.md` — extend integration coverage after implementation

**Out of scope (v1):**

- `confluence_get_page_by_title` (separate issue)
- Content cleanup / excerpt shaping on get-page response
- Fetching live content from Confluence API (local DB only)
- Changing DB uniqueness (`UNIQUE(space_id, confluence_id)` stays)
- Pagination when ambiguous (return full candidate list in error body; no “pick first” heuristic)

---

## Problem Summary

Today:

| Surface | Get full page? | `space_key` required? |
|---------|----------------|----------------------|
| MCP `confluence_get_page` | Yes | **Yes** (`parseGetPageArgs` rejects empty) |
| REST | No dedicated endpoint | N/A — workaround: `GET /api/spaces/{key}/pages?include_content=true` + client filter |
| CLI | No `get-page` command | N/A — only `search` (hits, not full body) |

Schema: `confluence_id` is unique **per space**, not globally:

```sql
UNIQUE(space_id, confluence_id)
```

In practice, a given integer ID usually appears in **one** crawled space. Requiring `space_key` for every fetch is friction for agents that only have a URL fragment `/pages/12345`.

---

## Target Behavior

### Shared lookup semantics

Store method (SQLite):

```go
// GetPageByConfluenceID resolves a page by Confluence integer ID.
// spaceKey may be empty: unique match succeeds; multiple matches return ErrAmbiguousPage.
func (d *DB) GetPageByConfluenceID(ctx context.Context, confluenceID int, spaceKey string) (*store.Page, string /*resolved space_key*/, error)
```

**Algorithm:**

1. Validate `confluenceID > 0`.
2. If `spaceKey != ""` → delegate to existing `GetPage(ctx, spaceKey, confluenceID)`; map `sql.ErrNoRows` → not found.
3. If `spaceKey == ""`:
   ```sql
   SELECT p.*, s.key
   FROM pages p
   JOIN spaces s ON s.id = p.space_id
   WHERE p.confluence_id = ?
   LIMIT 2   -- only need to know 0 / 1 / 2+
   ```
   - 0 rows → not found
   - 1 row → return page + `s.key`
   - 2+ rows → `ErrAmbiguousPage` with candidate `space_key`s

**Typed errors** (new package `internal/search` or `internal/store`):

```go
var ErrPageNotFound = errors.New("page not found")

type AmbiguousPageError struct {
    ConfluenceID int
    SpaceKeys    []string
}
func (e *AmbiguousPageError) Error() string { ... }
```

HTTP mapping:

| Error | HTTP | MCP tool result |
|-------|------|-----------------|
| not found | `404` | `isError: true`, message |
| ambiguous | `409` | `isError: true`, list `space_keys` in message/body |
| invalid id | `400` | parse error |

Response shape reuses existing `search.PageDetail` (already includes `space_key` in output even when omitted on input).

---

## API Surfaces

### REST (new)

```
GET /api/pages/{confluence_id}
GET /api/pages/{confluence_id}?space_key=PROJ
```

**Examples:**

```sh
# Unambiguous (one page with id 42 in catalog)
curl -s http://localhost:8081/api/pages/42

# Disambiguate when needed
curl -s "http://localhost:8081/api/pages/42?space_key=TST"
```

**Success `200`:**

```json
{
  "confluence_id": 42,
  "space_key": "TST",
  "title": "Mosquito Notes",
  "version": 1,
  "content": "The space mosquito lives in integration tests.",
  "updated_at": "2026-07-09T12:00:00Z"
}
```

**Ambiguous `409`:**

```json
{
  "error": "multiple pages share confluence_id 42",
  "confluence_id": 42,
  "space_keys": ["TST", "PROJ"]
}
```

**Handler location:** `internal/api/page.go` — `PageByConfluenceIDHandler`.

**Route registration** in `internal/app/server.go`:

```go
mux.HandleFunc("GET /api/pages/{confluence_id}", api.PageByConfluenceIDHandler(database, cfg, log))
```

Use `r.PathValue("confluence_id")` + `r.URL.Query().Get("space_key")`.

---

### MCP (extend existing tool)

Update **`confluence_get_page`** — do **not** add a second tool name.

**Input schema change:**

```json
{
  "type": "object",
  "properties": {
    "confluence_id": { "type": "integer", "description": "Confluence page ID" },
    "space_key": { "type": "string", "description": "Optional. Required only when multiple spaces contain the same confluence_id." }
  },
  "required": ["confluence_id"]
}
```

**`parseGetPageArgs`** (`internal/mcp/page_lookup.go`):

- `confluence_id` required (unchanged validation)
- `space_key` optional (empty string allowed)

**Tool implementation** (`toolGetPage`): call `GetPageByConfluenceID`; map ambiguity to tool error text:

```
Error: ambiguous confluence_id 42 in spaces: TST, PROJ — pass space_key
```

---

### CLI (new command)

```sh
spacemosquito get-page <confluence_id> [space_key]
```

**Examples:**

```sh
spacemosquito get-page 250347937
spacemosquito get-page 42 TST
```

**Output (default):** JSON to stdout (`search.PageDetail`), same as REST.

**Flags (optional v1):**

- `--content-only` — print body text only (handy for pipes)
- `--json` — explicit JSON (default)

**Exit codes:**

| Code | Meaning |
|------|---------|
| 0 | found |
| 1 | not found / DB error |
| 2 | ambiguous (print `space_keys` to stderr) |

Wire in `internal/cliapp/run.go` + `printUsage()`.

---

## Implementation Plan

### Phase 1 — Store layer

| File | Change |
|------|--------|
| `internal/store/store.go` | Add `GetPageByConfluenceID` to `Store` interface |
| `internal/store/sqlite/sqlite.go` | Implement lookup + ambiguity |
| ~~`internal/db/models.go`~~ | Removed with Postgres store |
| `internal/store/errors.go` (new) | `ErrPageNotFound`, `AmbiguousPageError` |

Add index if needed for lookup-by-id-only:

```sql
CREATE INDEX idx_pages_confluence_id ON pages(confluence_id);
```

Migration under `migrations/sqlite/` (Postgres tree removed).

### Phase 2 — API + MCP + CLI

| File | Change |
|------|--------|
| `internal/api/page.go` (new) | REST handler |
| `internal/app/server.go` | Register route |
| `internal/mcp/page_lookup.go` | Optional `space_key` |
| `internal/mcp/server.go` | Update tool schema + `toolGetPage` |
| `internal/cliapp/run.go` | `get-page` command |

Shared helper (avoid duplication):

```go
// internal/search/page_lookup.go
func GetPageDetail(ctx context.Context, db store.Store, confluenceID int, spaceKey string, exposeInternalIDs bool) (PageDetail, error)
```

Maps store errors → API errors; builds `PageDetail` via existing `ToPageDetail`.

### Phase 3 — Tests

| Test | Package |
|------|---------|
| `TestGetPageByConfluenceID_uniqueWithoutSpaceKey` | `internal/store/sqlite` |
| `TestGetPageByConfluenceID_ambiguous` | `internal/store/sqlite` |
| `TestGetPageByConfluenceID_withSpaceKey` | `internal/store/sqlite` |
| `TestPageByConfluenceIDHandler_200/404/409` | `internal/api` |
| `TestParseGetPageArgs_optionalSpaceKey` | `internal/mcp` |
| `TestMCP_GetPage_withoutSpaceKey` | `internal/app` (integration tag) |
| `TestMCP_GetPage_ambiguous` | `internal/app` (integration tag) |

Fixture: seed two spaces with the **same** `confluence_id` to exercise `409`.

### Phase 4 — Docs

| Doc | Update |
|-----|--------|
| `README.md` | REST table: `GET /api/pages/{confluence_id}` |
| `README.md` | CLI `get-page` example |
| `DEVELOPMENT.md` | curl example |

---

## Acceptance Criteria

- [ ] `GET /api/pages/{id}` returns full `PageDetail` when match is unique
- [ ] `GET /api/pages/{id}?space_key=X` disambiguates and matches existing `GetPage` behavior
- [ ] `GET /api/pages/{id}` returns `409` with `space_keys` when multiple rows share `confluence_id`
- [ ] MCP `confluence_get_page` accepts omitted `space_key` with same semantics
- [ ] CLI `spacemosquito get-page <id> [space_key]` works against local SQLite
- [ ] Ambiguous and not-found cases have clear error messages on all three surfaces
- [ ] `go test -race ./...` passes
- [ ] Integration tests cover REST + MCP happy path and ambiguity (`-tags=integration`)

---

## Design Decisions (resolved)

| Question | Decision |
|----------|----------|
| New tool vs extend `confluence_get_page`? | **Extend** existing tool (one name for agents) |
| Ambiguity handling | **Fail with 409** + candidate `space_keys`; never guess |
| Global uniqueness constraint? | **No** — keep per-space uniqueness in DB |
| REST path | **`GET /api/pages/{confluence_id}`** (not nested under `/api/spaces/`) — ID-first ergonomics |
| Response shape | Reuse **`search.PageDetail`** everywhere |
| `space_key` in response | **Always present** on success (resolved from DB) |

---

## Open Questions

1. Should ambiguous `409` include titles per candidate space for easier disambiguation? **Recommend yes** — `{space_key, title}` pairs in error body.
2. Postgres integration tests — **N/A** (SQLite-only).
3. Rate-limit or cap candidate list size in ambiguity error? **Defer** — typical collision count is 2–3.

---

## Related Files

| File | Role |
|------|------|
| `internal/mcp/page_lookup.go` | Current required `space_key` validation |
| `internal/mcp/server.go` | Tool schema + `toolGetPage` |
| `internal/store/store.go` | `GetPage` today |
| `internal/search/dto.go` | `PageDetail` response type |
| `internal/app/server.go` | Route registration |
| `internal/app/server_integration_test.go` | MCP integration tests to extend |

---

## Estimated Effort

| Phase | Size |
|-------|------|
| Store + errors + migration | S |
| REST + MCP + shared helper | S |
| CLI | S |
| Tests + docs | M |

**Total:** ~0.5–1 day.
