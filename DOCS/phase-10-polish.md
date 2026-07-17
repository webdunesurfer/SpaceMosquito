# Phase 10: Polish

> **Historical.** This phase document describes work from the Docker/Postgres era.
> Docker mode has been removed; SpaceMosquito is SQLite-only.
> See [`DOCS/task-remove-docker-mode.md`](./task-remove-docker-mode.md).

> **Status**: Completed (Phase 11 hardening in progress). See `.opencode/plans/phase-11-production-hardening.md`.

## Objective
Finalize the project with asset handling, clean HTML conversion, error handling, CLI polish, and documentation.

## Deliverables
- Robust asset download (images, attachments, embedded files)
- Clean HTML with rewritten URLs for offline navigation
- Comprehensive error handling throughout the stack
- CLI polish: help text, subcommands, flags
- README with setup documentation
- `.env.example` and `config.yaml.example`
- Complete structured logging across all components

## Logging Strategy
- All error handling uses structured `zap` logging with context (URL, space_key, page_id, error, stack)
- HTTP request logging middleware logs all API requests with correlation IDs
- CLI uses structured logging with component-based naming via `logging.Sugar`
- Error types include structured fields for machine parsing
- All logging uses `logging.Sugar` for consistent API

## Task Status

### 10.1 — Asset Download Improvements ✅
- `internal/storage/asset.go`: Confluence CDN URLs, attachment URLs, inline images handled
- Download images at original resolution, save with hash-based filenames
- Assets tracked in `metadata.json`
- Structured logging throughout

### 10.2 — Clean HTML Conversion ✅
- `internal/storage/writer.go`: URL rewriting, CSS preservation
- HTML stored at `saved/{space_key}/{title}/page.html`
- Structured logging throughout

### 10.3 — Error Handling Throughout ✅
- Scraper: structured error logging with context (page URL, error, stack)
- Extension: error handling in popup with user-facing feedback
- Backend: HTTP error responses, request logging via `api.LoggingMiddleware`
- Async crawl jobs: error tracked in `crawl_jobs` table

### 10.4 — CLI Polish ✅
- `cmd/cli/main.go`: subcommands (`init`, `save`, `crawl`, `search`, `reindex`, `stats`, `cron`, `serve`)
- Help text via `printUsage()`
- Structured logging with `logging.Sugar`
- `runServe` wired to full server startup (Phase 11)

### 10.5 — README ✅
- Project overview, architecture diagram, quick start guide
- Configuration reference, API reference, troubleshooting
- Security section with secrets policy

### 10.6 — Example Configs ✅
- `.env.example` with environment variable templates
- `config.yaml.example` with all options and defaults
- Security warning comments added

### 10.7 — Makefile ✅
- All targets: `build`, `run`, `test`, `migrate-up`, `migrate-down`, `docker-up`, `docker-down`, `docker-logs`, `docker-build`, `docker-migrate`, `serve-docker`, `crawl-docker`, `lint`, `dev-extension`, `build-extension`, `clean`, `config-example`

### 10.8 — .gitignore ✅
- All sensitive files excluded: `*.enc`, `config.yaml`, `cron-config.json`, `*.log`, `session-data/`, `saved/`, build artifacts, IDE files, OS artifacts
- Extension artifacts: `node_modules/`, `dist/`, `*.xpi`

## Remaining (Phase 11)
- `runServe` CLI command fully implemented
- Migration numbering consistency
- ADR/DOCS updates for go-rod
- Full build/test verification
