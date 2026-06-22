# SpaceMosquito Architecture

SpaceMosquito is a Confluence space scraper, indexer, and search engine with
automated cron scheduling. It uses Confluence REST API for content extraction
(with headless browser fallback), stores pages locally, and indexes content for
BM25/lexical search. It exposes an MCP server for LLM integration, plus
Firefox/Chrome browser extensions for session management and crawl control.

## System Components

Host machine:
  Firefox / Chrome with Pirate Mosquito extension
    - Captures Confluence session cookies
    - Provides popup UI for session/crawl/cron management

Docker (Colima):
  app (Go backend, port 8081)          PostgreSQL + pgvector
  +----------------------------------+ +---------------------+
  | HTTP API / MCP SSE               | | tables:             |
  | Cron scheduler                   | |   spaces | pages    |
  | Scraper (API + browser fallback) | |   + fts index       |
  | Storage (saved-data/)            | |                     |
  | Session (AES-GCM)                | |                     |
  +----------------------------------+ +---------------------+

Volumes:
  config.yaml          -> runtime config (read-only)
  cron-config.json     -> per-space cron overrides
  session.enc          -> encrypted cookies
  saved-data/          -> crawled pages + assets
  pgdata/              -> PostgreSQL data

## Entry Points

### `cmd/server/main.go`

Starts HTTP API + MCP SSE server on port 8081. Initializes cron scheduler,
scraper, session store, and database connection.

### `cmd/cli/main.go`

CLI tool with subcommands: `init`, `save`, `crawl`, `search`, `reindex`,
`stats`, `cron list/config/run-now`, `serve`. Shares `app.Run()` with server
for configuration loading.

## Internal Packages

| Package     | Responsibility                                     |
|-------------|----------------------------------------------------|
| `api`       | HTTP handlers -- session, search, crawl, cron,     |
|             | spaces, MCP routing                                |
| `app`       | Shared app initialization -- config, DB, storage,  |
|             | session store                                      |
| `config`    | YAML config loading with JSON per-space cron       |
|             | overrides                                          |
| `cron`      | gocron/v2 scheduler -- full/incremental crawl jobs,|
|             | per-space interval overrides                       |
| `db`        | PostgreSQL via pgx -- models, migrations, FTS      |
| `embedder`  | (Deferred) Vector embeddings -- ONNX, OpenAI, BGE  |
| `mcp`       | MCP SSE transport -- JSON-RPC 2.0, search_pages,   |
|             | get_page, list_spaces, crawl_space                 |
| `scraper`   | Dual-mode: Confluence REST API (primary) +         |
|             | go-rod browser (fallback)                          |
| `session`   | Cookie capture/management, Cloud vs Server flavor  |
|             | detection                                          |
| `storage`   | File system storage -- HTML, assets, metadata per  |
|             | space/page                                         |

## Data Flow

### Session Capture

  Browser (Confluence page)
    -> cookies.getAll() (extension background)
    -> POST /api/session (REST API)
    -> AES-256-GCM encryption
    -> session.enc (encrypted file, volume-mounted)

Details:
- Uses browser native cookies API (no polyfill)
- Captures all cookies, maps Secure=true to SameSite=None (Atlassian req)
- Writes AES-GCM encrypted JSON via session/store.go
- Flavor auto-detected during validation:
  Cloud:   /wiki/rest/api/... endpoints (tenant.atlassian.net)
  Server:  /rest/api/... endpoints
- Flavor stored in session, used for API URL construction

### Crawl Job

  Extension popup (click Crawl)
    -> POST /api/crawl (REST API)
    -> CrawlJobManager creates in-memory job
    -> CrawlRunner.Run() loads session
    -> discoverSpace() tries API first, falls back to browser:
       API: GET /rest/api/space/{key}/content/page (Cloud)
            GET /rest/api/content?spaceKey={key} (Server)
       Browser: go-rod navigates to space overview, parses sidebar
    -> For each discovered page:
       ScrapePageAPI() -- GET /rest/api/content/{id} (primary)
       If API fails: ScrapePage() -- go-rod browser (fallback)
         -> extractContent() cleans HTML, extracts text,
            downloads images with rate limiting + retries
       -> UpsertPage() saves to PostgreSQL
       -> UpdateSpaceLastCrawled() updates space metadata
    -> Job status: pending -> running -> completed/failed
    -> Progress: GET /api/crawl/status (polling every 3s)

Details:
- Dual-mode scraping: API-first, browser fallback
  Browser only launched on-demand when API fails
- Space discovery dual-mode: API list endpoint first,
  sidebar DOM parsing as fallback
- Confluence flavor detection: Session validation probes
  multiple API endpoints to auto-detect Cloud vs Server/DC
  Flavor stored in session.Flavor
- Rate-limited asset downloads: 5s rate limit, 3 retries
  with exponential backoff
- Queue-based child discovery: Pages in sidebar appended
  to crawl queue for depth-first traversal
- Session used as HTTP headers for API calls, or CDP
  Page.setCookies for browser fallback
- Pages stored in saved-data/{space}/{page}/ as clean
  HTML + raw HTML + metadata JSON
- In-memory job tracking with mutex-protected updates

### Search

  User query
    -> GET /api/search?q=<query>&space=<key>
    -> pgx: SELECT ... WHERE content_vector @@ plainto_tsquery($1)
    -> ORDER BY ts_rank(content_vector, query) DESC
    -> Returns ranked results with snippets

Details:
- PostgreSQL tsvector with GIN index for BM25 lexical search
- FTS triggered on each page save via IndexPageContent()
- No vector embeddings yet (deferred phase)

### Cron Scheduling

  Server startup
    -> load config.yaml + cron-config.json
    -> For each space:
       -> RegisterCrawler(spaceURL, interval)
       -> Creates CrawlJob in memory
       -> Goroutine: Ticker(interval) -> RunJob(ctx)

Details:
- gocron/v2 scheduler with per-space intervals
- cron-config.json stores per-space overrides
- Extension updates intervals via POST /api/cron/space/{key}

## Database Schema

spaces -- tracked Confluence spaces
  id        UUID, PK
  key       VARCHAR(10), UNIQUE   -- e.g. NCHB
  name      TEXT
  url       TEXT
  last_crawled  TIMESTAMP
  created_at    TIMESTAMP

pages -- crawled pages
  id                UUID, PK
  space_id          UUID, FK -> spaces
  confluence_id     INT
  title             TEXT
  confluence_url    TEXT
  parent_confluence_id INT
  content           TEXT, FTS searchable
  html_path         TEXT
  raw_html_path     TEXT
  metadata_path     TEXT
  content_vector    tsvector, GIN indexed
  created_at        TIMESTAMP
  updated_at        TIMESTAMP

crawl_jobs -- in-memory crawl job records
  id            UUID, PK
  space_url     TEXT
  status        string (pending/running/completed/failed)
  total_pages   int
  completed     int
  failed        int
  progress      int (percentage)
  error         string
  created_at    time.Time
  started_at    time.Time
  completed_at  time.Time
  updated_at    time.Time

## Scraping Modes

SpaceMosquito uses two scraping strategies:

1. Confluence REST API (default, fast)
   Cloud:   /wiki/rest/api/content/{id}?expand=body.storage
   Server:  /rest/api/content/{id}?expand=body.storage
   Space discovery: /wiki/rest/api/space/{key}/content/page (Cloud)
                    /rest/api/content?spaceKey={key} (Server)
   Requires valid session cookies and appropriate permissions

2. Headless Browser (fallback, slower)
   go-rod with Chromium, launched on-demand only when API fails
   Navigates to page, extracts HTML via DOM
   Used for space discovery fallback and API failure fallback
   Resilient to permission restrictions but slower

## Security

### Session Encryption

- AES-256-GCM with 32-byte user-generated key
- Key from config.yaml session.encryption_key
- File permissions: 0600 (owner read/write only)
- Key loss = permanent session data loss

### Cookie Security

- Captured via browser.cookies.getAll() (Firefox) or
  chrome.cookies.getAll() (Chrome)
- SameSite=None explicitly set (Atlassian requirement)
- Encrypted on disk, decrypted in memory only

### API Security

- No authentication on HTTP API (MVP)
- Intended to run behind reverse proxy with TLS

## Technology Decisions

### Dual-Mode Scraper (API + Browser)

- Confluence REST API is primary content extraction method --
  lightweight, no browser overhead
- Supports both Cloud (/wiki/rest/api/...) and Server/DC (/rest/api/...)
- go-rod browser fallback activates only when API fails
- Browser launched on-demand per crawl job, not at startup
- Required for space discovery fallback when API unreachable

### Confluence Flavor Detection

- Session validation auto-detects Cloud vs Server/DC by probing:
  /wiki/rest/api/user/current (Cloud)
  /rest/api/latest/myself (Server)
  /rest/api/user/current (Server)
- Flavor stored in session, used for URL construction

### go-rod over chromedp

- chromedp NoSandbox fails in Colima vz driver (EPERM)
- go-rod launcher.NoSandbox(true) with explicit Chromium works
- Browser only used as fallback for API failures

### Extension Architecture

- No content_scripts (Confluence CSP blocks injection)
- Background service worker handles all logic
- Promise-based messaging between popup and background
- Space detection via URL parsing (not content scripts)
- Crawl state persisted via extension storage API
- Parallel Firefox and Chrome implementations

### PostgreSQL over SQLite

- Docker-native, concurrent access
- tsvector + GIN index for BM25-style search
- pgvector extension ready for future embeddings

### File-Based Session Storage

- Simple, portable, volume-mountable
- AES-GCM encryption without external KV store

## Deployment

### Docker Compose

  db    pgvector/pgvector:pg17   PostgreSQL + pgvector
  app   (built from Dockerfile)  Go backend + scraper

### Volume Mounts

  Host File          Container Path       Purpose
  -------------      ----------------     ----------
  config.yaml        /app/config.yaml:ro  Runtime config
  cron-config.json   /app/cron-config.rw  Cron overrides
  session.enc        /app/session.enc:rw  Encrypted cookies
  saved-data/        /app/saved           Crawled pages
  pgdata/            /var/lib/postgresql  PostgreSQL data

### Extensions

Both run on host (not in Docker), communicate with backend at localhost:8081.

Firefox: loaded via about:debugging, built via webpack to firefox-extension/
Chrome:  loaded via chrome://extensions, built via webpack to chrome-extension/

## Known Limitations

1. No vector embeddings -- BM25/lexical only (deferred phase)
2. No API auth -- local/dev use behind reverse proxy
3. Session expiry -- cookies expire (~30 days), must re-capture
4. Confluence CSP -- blocks content script injection
5. In-memory crawl jobs -- not persisted across server restarts
6. Single browser instance -- one go-rod browser per job
7. Browser fallback needed -- some pages fail API, trigger slower browser scrape
