# Phase 3 — Deep Code Cleanup (SQLite-Only)

**Parent:** [`DOCS/task-remove-docker-mode.md`](../task-remove-docker-mode.md)  
**Priority:** P0  
**Size:** M  
**Status:** Done  
**Depends on:** Phase 2 preferred (packaging gone) but not strictly required for code deletion.

## Objective

Make the Go codebase **SQLite-only**: remove the Postgres store, dual-driver switches, and unused deps. Default database driver is **sqlite**.

## Inventory (Postgres path)

| Path | Role |
|------|------|
| `space-mosquito/internal/db/` | `pgx` pool, Postgres `Store` |
| `space-mosquito/migrations/postgres/` | SQL migrations |
| `internal/datastore/open.go` | `switch` sqlite / postgres |
| `internal/datastore/migrate.go` | Dual migrate paths |
| `internal/store/migrate.go` | Postgres migrate helpers + `postgres` migrate driver |
| `internal/config` | host/port/user/password/dbname; `DriverName()` defaults to **postgres** today |
| `go.mod` | `jackc/pgx/v5`, migrate postgres driver |

## Work items

### Store / migrate / config

- [x] Delete package `space-mosquito/internal/db/` (`postgres.go`, `models.go`, `types_alias.go`, …)
- [x] Delete `space-mosquito/migrations/postgres/`
- [x] `datastore.Open` — SQLite only; **error** if `driver: postgres`
- [x] Migrate helpers — SQLite only; drop postgres migrate driver import
- [x] `DatabaseConfig` — drop host/port/user/password/dbname/sslmode **or** deprecate temporarily; **default driver = sqlite**
- [x] Remove `DATABASE_URL` Postgres DSN path if Compose-only
- [x] `go.mod` / `go.sum` — drop unused `pgx` / migrate postgres
- [x] Grep: `postgres`, `pgx`, `pgvector`, `DriverName`, `DATABASE_URL` — zero production hits (except clear error strings)

### CLI / binaries

- [x] `init`, `serve`, `migrate-down`, `reindex` assume SQLite paths
- [x] Prefer documenting `cmd/spacemosquito`; **keep** `cmd/server` + `cmd/cli` as thin aliases (overview decision)

### Makefile (complete rewrite if not done in Phase 2)

- [x] `build` → `spacemosquito`
- [x] `test` → `go test -race ./...`
- [x] Optional `test-integration`, `lint`, extension targets — **no Docker**

## Acceptance criteria

- [x] No `internal/db` package; no `migrations/postgres`
- [x] Open/migrate paths SQLite-only; default driver sqlite
- [x] No unused `pgx` / postgres migrate deps in `go.mod`
- [x] `go test -race ./...` passes (full test green may land with Phase 4)

## Out of scope

- Shell smoke test deletion (Phase 4)
- Doc / ADR rewrites (Phases 5–6)
