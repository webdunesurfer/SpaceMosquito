# Phase 4 — Deep Test Cleanup

**Parent:** [`DOCS/task-remove-docker-mode.md`](../task-remove-docker-mode.md)  
**Priority:** P0  
**Size:** S–M  
**Depends on:** Phase 3 (Postgres store gone).

## Objective

Remove Docker/Postgres-dependent tests and harnesses. SQLite unit tests and in-process integration tests are the only supported automated paths.

## Inventory

| Item | Role |
|------|------|
| `tests/test_phase4_fts.sh`, `tests/test_phase5_mcp.sh` | Manual smoke against running Docker server |
| `DOCS/task-db-integration-tests.md` | Postgres integration test plan |
| `TEST_DATABASE_URL` / Postgres harness stubs | Obsolete |
| SQLite store + `internal/app` integration tests | **Keep** — sole path |

## Work items

- [ ] Delete or rewrite Docker smoke scripts — point to `go test -race -tags=integration ./internal/app/...`
- [ ] Mark `DOCS/task-db-integration-tests.md` **superseded** (banner) or delete
- [ ] Remove `TEST_DATABASE_URL` / Postgres harness code if any
- [ ] Confirm all store tests use SQLite temp DBs
- [ ] CI (`.github/workflows/go-test.yml`) — no Compose service; SQLite-only (already true; verify)

## Acceptance criteria

- [ ] No test docs or scripts require Docker Compose or live Postgres
- [ ] `go test -race ./...` passes
- [ ] `go test -race -tags=integration ./internal/app/...` passes

## Out of scope

- README / architecture rewrite (Phase 5)
- ADR deletions (Phase 6)
