# Phase 10: Polish

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
- CLI uses structured logging with `--verbose` flag for DEBUG level
- Error types include structured fields for machine parsing
- All logging uses `logging.Sugar` for consistent API

## Tasks

### 10.1 — Asset Download Improvements
- `internal/storage/asset.go`:
  - Handle Confluence CDN URLs (`https://confluence-attachments...`)
  - Handle attachment URLs (`/download/attachments/...`)
  - Handle inline images in wiki markup (`!image.png!`)
  - Download images at original resolution
  - Save attachments with original filenames (hash if duplicate)
  - Track all assets in `metadata.json`
  - **Already has logging from Phase 1 logging refactor**

### 10.2 — Clean HTML Conversion
- `internal/storage/writer.go`:
  - URL rewriting for offline navigation
  - Preserve readability (CSS, code blocks, tables)
  - Generate `README.html` at space root with navigation index
  - **Already has logging from Phase 1 logging refactor**

### 10.3 — Error Handling Throughout
- Scraper:
  - Retry failed pages (3 attempts, exponential backoff)
  - Skip rate-limited pages (429 → wait and retry)
  - Log all errors with context (page URL, error message, stack trace)
  - Failed pages listed in crawl status
  - **Log retry attempts with backoff duration, rate limit headers, skip reasons**
- Extension:
  - Toast notifications for all user-facing errors
  - "Retry" buttons for failed operations
  - Session expiry detection and re-auth prompt
  - **Log extension errors in console with error_type, timestamp, operation**
- Backend:
  - HTTP error responses with descriptive messages
  - Structured error types (AuthError, NotFoundError, ValidationError)
  - Request logging with correlation IDs (via `api.LoggingMiddleware`)
  - **Already has request logging from Phase 1 logging refactor**

### 10.4 — CLI Polish
- `cmd/cli/main.go`:
  - Cobra or urfave/cli for subcommands
  - Flags: `--config`, `--output`, `--verbose`
  - Help text for all commands
  - Version output: `space-mosquito --version`
  - **Use structured logging; `--verbose` enables DEBUG level via logger config**

### 10.5 — README
- Project overview and architecture diagram
- Installation, configuration, usage examples
- Troubleshooting: common issues and solutions
  - Include logging troubleshooting: how to enable verbose logging, where to find structured logs

### 10.6 — Example Configs
- `.env.example`
- `config.yaml.example` with all options and defaults
  - Add `logging.level` and `logging.format` options

### 10.7 — Makefile
- Build, run, test, dev-extension, build-extension, docker, migrate, lint targets
  - Add `logs` target: `tail -f spacemosquito.log` for development

### 10.8 — .gitignore
- Extension build artifacts, saved data, secrets, logs

## Acceptance Criteria
- All assets (images, attachments) are downloaded and linked correctly
- Clean HTML is navigable offline in any browser
- Error handling is comprehensive and user-friendly
- CLI commands are well-documented and functional
- README covers all setup and usage scenarios
- Example configs are provided
- Makefile targets work correctly
- All errors logged with structured fields (error_type, context, stack)
- All API requests logged with correlation IDs and request/response timing
- CLI supports `--verbose` for DEBUG level logging
- All logging uses consistent `logging.Sugar` API
