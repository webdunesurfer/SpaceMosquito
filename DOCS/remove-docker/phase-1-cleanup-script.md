# Phase 1 — Local Cleanup Guide + Script

**Parent:** [`DOCS/task-remove-docker-mode.md`](../task-remove-docker-mode.md)  
**Priority:** P0  
**Size:** S  
**Blocks:** Nothing — can ship before Phase 2. Local wipes need not finish before later phases.

## Objective

Ship a **cleanup guide** and optional **script** so developers can wipe leftover Docker Compose state (containers, named volumes, optional images) from machines that used the old stack — without deleting user crawl data by default.

## Deliverables

| Artifact | Path |
|----------|------|
| Script | `scripts/cleanup-docker-legacy.sh` |
| Guide (optional sibling) | `DOCS/guides/cleanup-docker-legacy.md` **or** section linked from README after Phase 5 |

## Script requirements

Idempotent; support `--dry-run`.

1. Detect Compose project / directory (default: repo root).
2. `docker compose down --remove-orphans` when `docker-compose.yml` exists **or** accept `--project-name` / known project name after Compose file is gone (Phase 2).
3. Remove named volume(s) historically used (`pgdata` from compose).
4. Optionally remove unused images matching `spacemosquito` / build tags.
5. Print leftover bind-mount paths (`./saved-data`, `./session.enc`, `./config.yaml`) — **do not delete** by default.
6. Suggest next steps: `spacemosquito init`, copy `saved-data` → `~/.spacemosquito/saved`, `bootstrap import-saved`, `reindex --content`.

Optional flag: `--purge-bind-mounts` (default **off**).

## Manual equivalent (document in guide)

```sh
./scripts/cleanup-docker-legacy.sh --dry-run
./scripts/cleanup-docker-legacy.sh

# Manual:
docker compose down --volumes --remove-orphans
docker volume ls | grep -i spacemosquito
# Copy ./saved-data into ~/.spacemosquito/saved if migrating
```

## Acceptance criteria

- [x] Script exists and is executable
- [x] `--dry-run` prints planned actions without mutating
- [x] Volumes / containers cleaned when Docker is available; graceful message if Docker missing
- [x] Bind mounts not deleted unless `--purge-bind-mounts`
- [x] Documented usage (script header and guide); link from overview README happens in Phase 5

**Shipped:** `scripts/cleanup-docker-legacy.sh`, `DOCS/guides/cleanup-docker-legacy.md`

## Out of scope

- Deleting `Dockerfile` / compose from the repo (Phase 2)
- Postgres → SQLite data dump
