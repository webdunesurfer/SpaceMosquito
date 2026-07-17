# ADR-016: SQLite-Only Distribution

- **Status**: Accepted
- **Date**: 2026-07-17
- **Supersedes**: ADR-014 (deleted)
- **Context**: ADR-014 added dockerless SQLite as the default while **keeping Docker + PostgreSQL** as a supported mode. That dual-mode posture increased packaging, config, migrate, and test surface. End-user and developer installs now share one local-binary story; Compose/Postgres packaging and the Postgres store have been removed.

- **Decision**:
  - **SQLite + FTS5** is the only supported database.
  - **Docker Compose / PostgreSQL** are not supported product modes (legacy cleanup via `scripts/cleanup-docker-legacy.sh` only).
  - All state lives under `~/.spacemosquito/` (or `--data-dir` / `SPACEMOSQUITO_DATA_DIR`).
  - Migrations live under `spacemosquito/migrations/sqlite/` (embedded in release builds).
  - API-first scraping with go-rod Chromium fallback remains (unchanged).
  - Former Docker users migrate via on-disk `saved/` + `bootstrap import-saved` (+ optional `reindex --content`), not Postgres dumps.

- **Rationale**:
  - One install path, one config shape, one test surface.
  - Dual drivers and Compose were no longer justified after dockerless became the primary path.
  - FTS5 covers lexical search; vector/semantic search remains out of scope.

- **Alternatives considered**:
  - **Keep Docker optional** — rejected; dual-mode cost outweighed benefit.
  - **Postgres-only** — rejected; too heavy for local single-user installs.
  - **Files-only (no DB)** — rejected; poor fit for search, listing, and incremental crawl.

- **Consequences**:
  - `database.driver: postgres` is rejected at open/migrate time.
  - Docs and ADRs must not present Compose as a supported install.
  - Migration: [README Coming from Docker](../README.md#coming-from-docker); legacy cleanup: `scripts/cleanup-docker-legacy.sh`.

- **Related**: ADR-009 (migrations), ADR-005 (session file), ADR-015 (saved page format)
