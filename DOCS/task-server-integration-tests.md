# Task: Server Integration Tests (REST + MCP, Seeded SQLite)

## Objective

Add **in-process integration tests** that wire the real application stack (migrations → store → HTTP mux → handlers), seed known fake data into SQLite, and exercise **REST** and **MCP** endpoints over HTTP. Tests must assert **HTTP 200** (or documented success codes) and that **response bodies contain the seeded data**, not merely that handlers return a non-error shape.

This closes the gap between:

- **Store tests** (`internal/store/sqlite/sqlite_test.go`) — DB only, no HTTP
- **Handler/MCP unit tests** — handlers in isolation, often `nil` DB
- Former Docker shell smoke scripts under `tests/` — **removed**; use this Go suite instead

**Parent references:**

- `DOCS/epic-dockerless-mode.md` — Tier 3 (`TestServerBoot_SQLite`, `TestMCP_SearchRoundTrip`)
- `DOCS/task-go-unit-tests.md` — unit scope explicitly defers this work
- `DOCS/task-db-integration-tests.md` — **superseded** (Postgres path removed)

**Out of scope (v1):**

- Building/running the release binary as a subprocess (optional follow-up smoke test)
- Docker Compose or PostgreSQL (use SQLite + embedded migrations)
- Browser, crawl jobs, cron execution, live Confluence
- Session extension / encrypted session round-trip (unless needed for a specific endpoint)
- SSE streaming stress tests or MCP session TTL expiry
- Manual curl smoke against a long-lived `serve` process (optional; not required for CI)

---

## Current State

| Layer | What exists | Gap |
|-------|-------------|-----|
| Store | `TestSQLiteStoreCRUDAndSearch` — migrate, seed, search in-process | No HTTP |
| API | `handler_test.go` — `httptest.NewRecorder`, nil DB for search | No seeded data, no full mux |
| MCP | `server_test.go` — `processMessage` in-process, fakes | No HTTP, no DB seed |
| Shell | ~~`test_phase5_mcp.sh`~~ | **Removed** — use this Go suite |

Default CI (`.github/workflows/go-test.yml`) runs `go test -race ./...` only — integration tests **must not** run on every PR unless we later promote a fast subset.

---

## Target Architecture

```
t.TempDir()  →  ~/.spacemosquito-like layout (config, sqlite db, saved/)
       ↓
paths.SetDataDir + test config (sqlite, random port)
       ↓
app.NewTestServer(cfg)  →  setupComponents (real wiring)
       ↓
testutil.SeedFixtures(store)  →  1 space, 2–3 pages, FTS-indexed content
       ↓
httptest.NewServer(mux)  →  real HTTP client
       ↓
REST GET /api/...  +  MCP POST /mcp + /mcp/session/{id}
       ↓
assert status + JSON contains seeded confluence_id / title / counts
```

**Design choice — in-process vs subprocess:**

| Approach | Pros | Cons |
|----------|------|------|
| **In-process `httptest.Server` (v1)** | Fast, no port races, same code path as `serve`, easy seed/cleanup | Does not catch link-time / packaging issues |
| **Subprocess `go build` + exec (v2)** | Validates release artifact | Slower, flaky ports, harder cleanup |

**v1 uses in-process only.** Document v2 as optional `TestReleaseBinary_Smoke` behind the same `integration` tag.

---

## Prerequisites (small code changes)

These are part of this task but are thin refactors, not feature work:

| Change | Why |
|--------|-----|
| Export test harness from `internal/app` | `setupComponents` is private; tests need the real `http.ServeMux` and `store.Store` |
| `internal/testutil` package | Shared seed data, HTTP helpers, MCP SSE JSON extractor |
| Disable cron in test config | Avoid background goroutines during tests (`cron.*.enabled: false` or skip `cronScheduler.Start`) |

### Suggested `app` test API

```go
// internal/app/testserver.go  (or testserver_integration.go with build tag)
type TestServer struct {
    URL    string
    Store  store.Store
    Config *config.Config
    Close  func()
}

func NewTestServer(t *testing.T, cfg *config.Config) *TestServer
```

Implementation:

1. Call existing `setupComponents(cfg, log)` (export or duplicate mux wiring — prefer single source).
2. Wrap `api.CORSMiddleware(api.LoggingMiddleware(mux, ...))` same as `Start()`.
3. `httptest.NewServer(handler)`.
4. `t.Cleanup` → `Close()` database, cron, httptest server.

**Cron:** Do **not** call `cronScheduler.Start` in test server — matches post–Phase 3 lazy behavior and avoids flaky timers.

---

## Build Tag & CI

```go
//go:build integration

package app_test
```

**Default:**

```bash
go test -race ./...   # skips integration
```

**Local / pre-release:**

```bash
go test -race -tags=integration ./internal/app/...
# or broader:
go test -race -tags=integration ./...
```

**CI (optional — add when stable):** nightly or `workflow_dispatch` job in `.github/workflows/integration.yml`; not required to merge PRs.

---

## Test Fixtures (`internal/testutil`)

### `SeedData` struct

Stable identifiers used across REST and MCP assertions:

| Field | Example | Purpose |
|-------|---------|---------|
| `SpaceKey` | `"TST"` | List/search filter |
| `SpaceName` | `"Test Space"` | `confluence_list_spaces` |
| `SpaceURL` | `https://example.atlassian.net/wiki/spaces/TST` | Space metadata |
| `Pages` | 3 entries | Distinct `confluence_id`, `title`, `content` |
| `SearchTerm` | `"mosquito"` | Appears in one page body only |
| `SearchPageID` | `42` | Page that must rank first for `SearchTerm` |

### `SeedFixtures(ctx, store.Store) (*SeedData, error)`

1. `CreateSpace`
2. `UpsertPage` for each page (with `Version`, `Content`, paths optional)
3. SQLite FTS: triggers should index on insert; if not visible in search, call `IndexAllPageContents` once and document why

### HTTP helpers

```go
func GETJSON(t, url string, dest any) int   // returns status code
func POSTJSON(t, url string, body, dest any) int
```

### MCP helpers

Port / session flow (historical shell smoke; now covered by Go tests):

1. `POST /mcp` → `initialize` → parse `session_id` from JSON body (non-SSE first response).
2. `POST /mcp/session/{id}` → parse SSE `data: {...}` lines into `MCPResponse`.
3. `MCPToolCall(t, baseURL, sessionID, toolName, args) (result, error)`

---

## Test Inventory

### Phase 1 — Boot & health

| Test | Assert |
|------|--------|
| `TestServerBoot_SQLite` | Temp data dir, migrations applied, `GET /health` → `200`, body `ok` |
| `TestServerBoot_embeddedMigrations` | No on-disk `migrations/` dir; `SPACEMOSQUITO_MIGRATIONS_DIR` unset; still boots (validates release path) |

### Phase 2 — REST with seeded data

| Test | Request | Assert |
|------|---------|--------|
| `TestREST_Search_returnsSeededPage` | `GET /api/search?q=mosquito` | `200`; JSON hits contain `confluence_id: 42`, title, non-empty excerpt |
| `TestREST_Search_spaceFilter` | `GET /api/search?q=mosquito&space_key=TST` | `200`; hits only from `TST` |
| `TestREST_Search_unknownSpace` | `GET /api/search?q=mosquito&space_key=NOPE` | `200`; empty hits |
| `TestREST_Stats_matchSeed` | `GET /api/search/stats` | `200`; `total_spaces >= 1`, `total_pages >= 3` |
| `TestREST_ListSpaces` | `GET /api/spaces` | `200`; array contains `space_key: TST`, `pages_crawled >= 3` |
| `TestREST_SpacePages` | `GET /api/spaces/TST/pages` | `200`; pages include seeded `confluence_id`s, ordered ASC |
| `TestREST_SpacePages_pagination` | `GET .../pages?limit=2&after_confluence_id=...` | `200`; cursor semantics match seed order |
| `TestREST_SpaceByKey` | `GET /api/spaces/TST` | `200`; `pages_crawled >= 3` |

Use table-driven subtests where multiple cases share the same server fixture (`TestMain` or `t.Parallel` with isolated temp dirs — prefer **one server per test** for isolation until perf becomes an issue).

### Phase 3 — MCP with seeded data

| Test | Tool / method | Assert |
|------|---------------|--------|
| `TestMCP_Initialize` | `POST /mcp` initialize | `200`; `session_id` present |
| `TestMCP_ToolsList` | `tools/list` | 4 tools registered (names match production) |
| `TestMCP_ListSpaces` | `confluence_list_spaces` | Result mentions `TST` |
| `TestMCP_ListSpace` | `confluence_list_space` `{space_key: TST}` | Page rows include seeded IDs/titles |
| `TestMCP_Search` | `confluence_search` `{query: mosquito}` | Hit contains `confluence_id: 42` |
| `TestMCP_GetPage` | `confluence_get_page` `{space_key: TST, page_id: 42}` | Title/content match seed |
| `TestMCP_GetPage_notFound` | unknown `page_id` | MCP error response (not HTTP 500) |
| `TestMCP_InvalidSession` | `POST /mcp/session/bogus` | JSON error (mirror shell test 7) |

**Important:** MCP assertions must validate **tool result content** (parsed JSON inside `result.content`), not only HTTP 200.

### Phase 4 — Optional follow-ups

| Test | Notes |
|------|-------|
| `TestREST_Reindex` | `POST /api/search/reindex` → `200`; search still works |
| `TestReleaseBinary_Smoke` | `go build -o $TMP/spacemosquito ./cmd/spacemosquito`; exec `serve` on random port; `GET /health` only |
| ~~Postgres variant~~ | **Dropped** — SQLite-only |

---

## Test Config Template

Written to `t.TempDir()/config.yaml` or built in memory:

```yaml
database:
  driver: sqlite
  path: spacemosquito.db
storage:
  base_path: ./saved
session:
  encryption_key: "test-key-32-bytes-minimum-padded!!"
  file_path: ./session.enc
mcp:
  port: 0   # ignored when using httptest.Server URL
  host: "127.0.0.1"
  expose_internal_ids: false
cron:
  full_crawl:
    enabled: false
  incremental:
    enabled: false
```

Set `paths.SetDataDir(tempDir)` before `paths.NormalizeConfig`.

---

## Files to Add / Change

| File | Action |
|------|--------|
| `internal/app/testserver.go` | `NewTestServer` harness (build-tagged or always compiled with unexported test hook) |
| `internal/app/server_integration_test.go` | `//go:build integration` — main test cases |
| `internal/testutil/seed.go` | Fixture seeding |
| `internal/testutil/http.go` | GET/POST JSON helpers |
| `internal/testutil/mcp.go` | Initialize + SSE parse + tool call |
| `DEVELOPMENT.md` | Document `go test -tags=integration` |
| `.github/workflows/integration.yml` | Optional nightly job |

**Prefer** `app_test` package (external test) to avoid exporting production internals beyond `NewTestServer`.

---

## Acceptance Criteria

- [ ] `go test -race ./...` stays green (integration excluded by default)
- [ ] `go test -race -tags=integration ./internal/app/...` passes locally without Docker or Postgres
- [ ] Tests use real `setupComponents` mux (not a hand-wired mini-mux)
- [ ] SQLite DB is seeded with known `confluence_id`s and searchable content
- [ ] At least **3 REST** endpoints assert response **data** matches seed (search, list spaces, list pages)
- [ ] At least **3 MCP tools** assert tool **result content** matches seed (search, list_space, get_page)
- [ ] `GET /health` returns `200` before and after seed
- [ ] No manual server startup required
- [ ] `DEVELOPMENT.md` documents how to run integration tests

---

## Open Questions

1. **Package location:** `internal/app` vs top-level `test/integration` — prefer `internal/app` to stay close to wiring; revisit if import cycles appear.
2. **Parallelism:** start with `t.Parallel()` disabled (SQLite + shared globals like `mcp.ServerInstance`); enable later if isolated.
3. **Promote to PR CI?** Only if runtime stays under ~30s on GitHub runners; otherwise nightly/release only.
4. **Shell scripts:** Deleted in remove-docker Phase 4; Go integration tests are the only automated path.

---

## Related Docs

| Doc | Relationship |
|-----|----------------|
| `DOCS/task-db-integration-tests.md` | Superseded — Postgres integration not planned |
| `DOCS/task-dockerless-migrations.md` | Embedded migrations validated in `TestServerBoot_embeddedMigrations` |
| `DOCS/task-mcp-list-space-pagination.md` | Pagination cases for `TestREST_SpacePages_pagination` |
| `DOCS/task-mcp-search-excerpts.md` | Excerpt shape in search assertions |
