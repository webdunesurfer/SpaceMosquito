# Phase 2 — Delete Docker Packaging

**Parent:** [`DOCS/task-remove-docker-mode.md`](../task-remove-docker-mode.md)  
**Priority:** P0  
**Size:** S  
**Depends on:** None (can run in parallel with Phase 1). **Decision:** hard delete — no `archive/docker/`.

## Objective

Remove all Docker / Compose packaging and Docker-centric helper scripts from the repository so the tree no longer implies a container install path.

## Inventory (delete)

| Artifact | Role |
|----------|------|
| `Dockerfile` | Builds `/app/server` + `/app/cli` with Chromium |
| `docker-compose.yml` | `db` (pgvector/pg17) + `app` |
| `app-start.sh` | Container entrypoint → `/app/server` |
| `.env.example` | Compose env (if Compose-only) |
| `scripts/rebuild-redeploy-restart-spacemosquito.sh` | Colima + `docker compose build/up` |
| `scripts/restart-spacemosquito.sh` | Docker restart helper (if Docker-only) |

## Makefile

- [ ] Remove targets: `docker-up`, `docker-down`, `docker-logs`, `docker-build`, `docker-migrate`, `serve-docker`, `crawl-docker`
- [ ] Fix or remove broken `config-example` heredoc (parse failure)
- [ ] Leave SQLite-oriented targets for Phase 3 rewrite if needed (`build`, `test`, `lint`, extension) — or minimal stub until Phase 3

## Config examples

- [ ] Delete root Postgres-oriented `config.yaml.example` if duplicated
- [ ] Keep **one** SQLite-first example (`space-mosquito/config.yaml.example` or data-dir docs) — final polish in Phase 5

## Acceptance criteria

- [ ] Listed packaging files hard-deleted
- [ ] No Docker Compose scripts remain (except Phase 1 `cleanup-docker-legacy.sh`)
- [ ] Makefile has no `docker-*` / `serve-docker` / `crawl-docker` targets
- [ ] `make` parses successfully (if Makefile retained)

## Out of scope

- Removing `internal/db` / Postgres migrations (Phase 3)
- README rewrite (Phase 5)
