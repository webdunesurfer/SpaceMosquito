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

## Tasks

### 10.1 — Asset Download Improvements
- `internal/storage/asset.go`:
  - Handle Confluence CDN URLs (`https://confluence-attachments...`)
  - Handle attachment URLs (`/download/attachments/...`)
  - Handle inline images in wiki markup (`!image.png!`)
  - Download images at original resolution
  - Save attachments with original filenames (hash if duplicate)
  - Track all assets in `metadata.json`:
    ```json
    {
      "assets": {
        "images": [
          {"original_url": "...", "local_path": "assets/images/abc123.png"}
        ],
        "attachments": [
          {"original_url": "...", "local_path": "assets/attachments/manual.pdf"}
        ]
      }
    }
    ```

### 10.2 — Clean HTML Conversion
- `internal/storage/writer.go`:
  - URL rewriting for offline navigation:
    - Internal links (`/wiki/display/PROJ/Page`) → `../another-page/index.html`
    - Image URLs → `assets/images/hash.png`
    - Attachment URLs → `assets/attachments/filename.pdf`
  - Preserve readability:
    - Keep essential CSS (tables, code blocks, headings)
    - Remove Confluence-specific layout classes
    - Inline critical styles (optional)
  - Generate `README.html` at space root with navigation index

### 10.3 — Error Handling Throughout
- Scraper:
  - Retry failed pages (3 attempts, exponential backoff)
  - Skip rate-limited pages (429 → wait and retry)
  - Log all errors with context (page URL, error message, stack trace)
  - Failed pages listed in crawl status
- Extension:
  - Toast notifications for all user-facing errors
  - "Retry" buttons for failed operations
  - Session expiry detection and re-auth prompt
- Backend:
  - HTTP error responses with descriptive messages
  - Structured error types (AuthError, NotFoundError, ValidationError)
  - Request logging with correlation IDs

### 10.4 — CLI Polish
- `cmd/cli/main.go`:
  - Cobra or urfave/cli for subcommands:
    ```
    space-mosquito serve          # Start API + MCP server
    space-mosquito crawl <url>    # Crawl a space
    space-mosquito search <query> # Semantic search
    space-mosquito list           # List crawled spaces/pages
    space-mosquito init           # Run migrations
    space-mosquito cron list      # List cron jobs
    ```
  - Flags: `--config`, `--output`, `--verbose`
  - Help text for all commands
  - Version output: `space-mosquito --version`

### 10.5 — README
- Project overview and architecture diagram
- Installation:
  - Local development (Go + PostgreSQL + Firefox extension)
  - Docker Compose (full stack with noVNC)
- Configuration: explain all config.yaml options
- Usage:
  - CLI commands with examples
  - Firefox extension setup
  - Docker setup and noVNC access
  - MCP server connection (Cursor, opencode, Gemini CLI)
- Troubleshooting: common issues and solutions

### 10.6 — Example Configs
- `.env.example`:
  ```
  SESSION_ENCRYPTION_KEY=your-32-character-encryption-key-here
  OPENAI_API_KEY=your-openai-key-if-using-openai-embeddings
  ```
- `config.yaml.example`:
  - All config options with comments and default values

### 10.7 — Makefile
- `Makefile` at project root:
  ```makefile
  build:              # Build Go backend
  run:                # Run Go backend locally
  test:               # Run Go tests
  dev-extension:      # Run Firefox extension in dev mode
  build-extension:    # Build extension bundle
  docker-up:          # docker compose up
  docker-down:        # docker compose down
  migrate-up:         # Run database migrations
  migrate-down:       # Rollback migrations
  lint:               # Lint Go and TypeScript
  ```

### 10.8 — .gitignore
- `firefox-extension/node_modules/`
- `firefox-extension/dist/`
- `space-mosquito/tmp/`
- `saved/`
- `*.enc`
- `.env`
- `config.yaml` (contains secrets)
- `*.log`

## Acceptance Criteria
- All assets (images, attachments) are downloaded and linked correctly
- Clean HTML is navigable offline in any browser
- Error handling is comprehensive and user-friendly
- CLI commands are well-documented and functional
- README covers all setup and usage scenarios
- Example configs are provided
- Makefile targets work correctly
