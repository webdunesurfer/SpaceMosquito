# Guide: Cleanup Legacy Docker State

Wipe leftover **Docker Compose** state from machines that ran SpaceMosquito with containers + PostgreSQL. Crawl data on the host (`saved-data/`, `session.enc`) is **not** deleted unless you opt in.

**Parent:** [`DOCS/remove-docker/phase-1-cleanup-script.md`](../remove-docker/phase-1-cleanup-script.md)  
**Overview:** [`DOCS/task-remove-docker-mode.md`](../task-remove-docker-mode.md)

## Prerequisites

- Docker CLI available (`docker` on `PATH`)
- Daemon running (Docker Desktop or Colima)

If Docker is missing or the daemon is down, the script prints host leftovers and migration steps, then exits successfully.

## Quick start

From the repo root:

```sh
chmod +x scripts/cleanup-docker-legacy.sh   # once

./scripts/cleanup-docker-legacy.sh --dry-run
./scripts/cleanup-docker-legacy.sh
```

## Options

| Flag | Effect |
|------|--------|
| `--dry-run` | Print actions only |
| `--project-name NAME` | Compose project name (default: repo directory basename, e.g. `SpaceMosquito`) |
| `--purge-images` | Remove images whose names match `spacemosquito` / `{project}-app` |
| `--purge-bind-mounts` | **Destructive:** delete host `saved-data/`, `saved/`, `session.enc`, `config.yaml`, `cron-config.json`, `.env` under the repo |

## What it does

1. `docker compose down --remove-orphans` (uses `docker-compose.yml` if present, else project name only).
2. Removes named volumes such as `{project}_pgdata`, `spacemosquito_pgdata`.
3. Optionally removes related images (`--purge-images`).
4. Lists bind-mount leftovers; deletes them only with `--purge-bind-mounts`.

## Manual equivalent

```sh
cd /path/to/SpaceMosquito
docker compose down --volumes --remove-orphans
docker volume ls | grep -i spacemosquito
# docker volume rm <name>   # if any remain
```

## Migrate to dockerless (after cleanup)

Postgres volume data is **not** imported. Rebuild the catalog from saved HTML:

```sh
cd space-mosquito
go build -o spacemosquito ./cmd/spacemosquito

./spacemosquito init
mkdir -p ~/.spacemosquito/saved
# If you still have the Compose bind mount:
cp -R ../saved-data/* ~/.spacemosquito/saved/   # adjust paths as needed

./spacemosquito bootstrap import-saved
./spacemosquito reindex --content
./spacemosquito serve
```

Point the Pirate Mosquito extension at `http://localhost:8081`.

## After packaging removal

Once `docker-compose.yml` is deleted from the repo (Phase 2), keep using the same script with `--project-name` if your Compose project name differs from the directory basename:

```sh
./scripts/cleanup-docker-legacy.sh --project-name SpaceMosquito --dry-run
```
