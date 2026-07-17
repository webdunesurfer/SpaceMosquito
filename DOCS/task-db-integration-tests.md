# Task: Database Integration Tests (PostgreSQL)

> **Superseded.** Postgres integration tests are not planned. SpaceMosquito is SQLite-only.
> Use store unit tests (`internal/store/sqlite`) and in-process server integration tests:
> `go test -race -tags=integration ./internal/app/...`
> See [`DOCS/task-server-integration-tests.md`](./task-server-integration-tests.md) and [`DOCS/task-remove-docker-mode.md`](./task-remove-docker-mode.md).

## Objective

Add optional **integration tests** for `internal/db` that run against a real PostgreSQL instance. Complements the unit-test suite in `DOCS/task-go-unit-tests.md`, which deliberately avoids Docker and live databases.

Primary motivation: verify SQL correctness for methods that unit tests cannot exercise — starting with changes from `DOCS/task-mcp-confluence-id.md` (`SearchPages` selecting `confluence_id`, FTS ranking) and other high-value DB paths.

**Out of scope:**

- Docker Compose in CI (optional local dev helper only)
- SQLite integration tests (see `DOCS/task-dockerless-migrations.md` — separate track)
- MCP SSE e2e, browser, or live Confluence
- Integration tests for `expose_internal_ids` debug flag behavior

---

## Current State

| Package | Unit tests | Integration tests |
|---------|------------|-------------------|
| `internal/db` | None | None |
| `internal/mcp` | Validation + protocol (no DB) | None |
| `internal/api` | Handler validation only (nil DB) | None |

CI (`.github/workflows/go-test.yml`) runs `go test -race ./...` — integration tests must **not** run by default.

---

## Approach

### Build tag

All integration tests live behind:

```go
//go:build integration
```

Files: `internal/db/*_integration_test.go`

Default CI:

```bash
go test -race ./...   # skips integration
```

Local / optional CI job:

```bash
TEST_DATABASE_URL="postgres://..." go test -race -tags=integration ./internal/db/...
```

### Database setup

**Option A (recommended for local dev):** Use existing project Postgres (Docker Compose or local install). Tests create isolated data per run:

1. Connect via `TEST_DATABASE_URL` (required env var; skip test if unset)
2. Run migrations (`db.MigrateUp`) once per test package `TestMain`
3. Each test uses transactions rolled back on cleanup, **or** truncates `pages` / `spaces` in `t.Cleanup`

**Option B (future):** `testcontainers-go` spins Postgres in CI — add only if team wants integration in GitHub Actions without a shared service.

### Skip helper

```go
func requireDB(t *testing.T) *db.DB {
    t.Helper()
    dsn := os.Getenv("TEST_DATABASE_URL")
    if dsn == "" {
        t.Skip("TEST_DATABASE_URL not set")
    }
    // connect, return *db.DB
}
```

---

## Test Inventory (Phase 1 — MCP Confluence ID)

After `DOCS/task-mcp-confluence-id.md` lands:

| Test | Setup | Assert |
|------|-------|--------|
| `TestSearchPages_returnsConfluenceID` | Insert space + page with known `confluence_id` and indexed `content` | `SearchResult.ConfluenceID` matches; no internal UUID field in result struct |
| `TestSearchPages_ranking` | Two pages, query matches one more strongly | Higher `similarity` for better match |
| `TestSearchPages_spaceFilter` | Pages in two spaces | `spaceKey` filter excludes other space |
| `TestSearchPages_emptyQuery` | — | Returns nil/empty, no error |
| `TestGetPage_byConfluenceID` | Insert page | `GetPage(space, confluenceID)` returns title, content |
| `TestGetPage_notFound` | — | Error for unknown `(space, confluence_id)` |

FTS setup: call `IndexPageContent` or insert with `content_vector` populated per migration schema.

---

## Test Inventory (Phase 2 — broader coverage)

| Function | Cases |
|----------|-------|
| `UpsertPage` / `CreatePage` | Insert + `ON CONFLICT` version update |
| `ListPages` | Order by `confluence_id`, limit |
| `DeleteStalePages` | Deletes only pages not updated since `crawlStart` |
| `ListSpaces` | Returns crawled spaces |
| `GetPageStats` | Counts after seed data |

Defer vector / embedding tests unless `pgvector` extension is available in test DB (migration already enables it).

---

## Suggested File Layout

```
internal/db/
  models.go
  postgres.go
  integration_test.go      // TestMain, requireDB, helpers
  search_integration_test.go // SearchPages
  pages_integration_test.go  // GetPage, UpsertPage, ListPages
```

Shared seed helpers:

```go
func seedSpaceAndPage(t *testing.T, db *DB, spaceKey string, confluenceID int, title, content string) uuid.UUID
```

---

## CI Strategy

| Job | When | Command |
|-----|------|---------|
| **unit** (existing) | Every push/PR | `go test -race ./...` |
| **integration** (new, optional) | Manual / nightly / label | `go test -race -tags=integration ./internal/db/...` with `TEST_DATABASE_URL` secret |

Do **not** block merge on integration job until a stable Postgres service exists in CI.

---

## Acceptance Criteria

- [ ] Integration tests use `//go:build integration` and skip when `TEST_DATABASE_URL` is unset
- [ ] `go test -race ./...` (no tags) still passes without Postgres
- [ ] `SearchPages` integration tests verify `confluence_id` in results
- [ ] `GetPage` integration test verifies lookup by `(space_key, confluence_id)`
- [ ] `DEVELOPMENT.md` documents how to run integration tests locally
- [ ] `.github/workflows/go-test.yml` unchanged OR optional separate workflow documented

---

## Local Run

```bash
# Start Postgres (project docker-compose or local)
export TEST_DATABASE_URL="postgres://spacemosquito:spacemosquito@localhost:5432/spacemosquito?sslmode=disable"

cd space-mosquito
go test -race -tags=integration -v ./internal/db/...
```

---

## Related Documents

| Doc | Relationship |
|-----|--------------|
| `DOCS/task-mcp-confluence-id.md` | First consumer of fixed `SearchPages` / `GetPage` SQL |
| `DOCS/task-go-unit-tests.md` | Unit tests remain the default CI gate |
| `DOCS/task-dockerless-migrations.md` | Future SQLite may replace Postgres for integration tests in dockerless mode |
