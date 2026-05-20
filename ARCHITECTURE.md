# SpaceMosquito Architecture

## Overview

SpaceMosquito is a Confluence space scraper, indexer, and search engine with automated cron scheduling. It captures pages via a headless browser, stores them locally, and indexes content for BM25/lexical search. It exposes an MCP server for LLM integration and a Firefox extension for interactive session management and crawl control.

## System Components

```
┌──────────────────────────────────────────────────────────────────┐
│                      Host Machine                                │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │              Firefox (Browser)                             │  │
│  │  ┌──────────────────────────────────────────────────────┐  │  │
│  │  │  Pirate Mosquito (Web Extension)                     │  │  │
│  │  │  ┌──────────┐ ┌──────────┐ ┌───────────┐            │  │  │
│  │  │  │          │ │ Background│ │  Popup UI │            │  │  │
│  │  │  │          │ │  Worker  │ │  (Session │            │  │  │
│  │  │  │          │ │          │ │  /Crawl/  │            │  │  │
│  │  │  │          │ │          │ │  Cron)    │            │  │  │
│  │  │  └──────────┘ └──────────┘ └───────────┘            │  │  │
│  │  └──────────────────────────────────────────────────────┘  │  │
│  └────────────────────────────────────────────────────────────┘  │
│                                                                  │
├──────────────────────────────────────────────────────────────────┤
│                          Docker (Colima)                         │
│                                                                  │
│  ┌──────────────────────┐    ┌───────────────────────────────┐  │
│  │   app (Go Backend)   │    │     PostgreSQL + pgvector     │  │
│  │                      │    │                               │  │
│  │  HTTP API  :8080     │    │  ┌──────────┐  ┌──────────┐  │  │
│  │  MCP/SSE :8081       │◄───┤  │  spaces  │  │  pages   │  │  │
│  │                      │    │  │          │  │  +fts     │  │  │
│  │  Cron Scheduler      │    │  │  crawl   │  │          │  │  │
│  │  Headless Scraper    │    │  │  jobs    │  │          │  │  │
│  │  Storage (disk)      │    │  └──────────┘  └──────────┘  │  │
│  │  Session (AES-GCM)   │    └───────────────────────────────┘  │
│  └──────────────────────┘                                       │
│                                                                  │
│  Volumes:                                                      │
│    config.yaml          → runtime config                        │
│    cron-config.json     → per-space cron overrides              │
│    session.enc          → encrypted cookies                     │
│    saved-data/          → crawled pages + assets                │
│    pgdata/              → PostgreSQL data                       │
└──────────────────────────────────────────────────────────────────┘
```

## Entry Points

### `cmd/server/main.go` — HTTP API + MCP Server

- Starts the HTTP API server on port 8080
- Starts the MCP (SSE) server on port 8081
- Initializes the cron scheduler
- Initializes the headless scraper (go-rod)
- Loads session from encrypted file

### `cmd/cli/main.go` — CLI Tool

- Subcommands: `init`, `save`, `crawl`, `search`, `reindex`, `stats`, `cron list/config/run-now`, `serve`
- Shares `app.Run()` with the server for configuration loading

## Internal Package Structure

| Package | Responsibility |
|---------|---------------|
| `api` | HTTP handlers — session, search, crawl, cron, spaces, MCP routing |
| `app` | Shared app initialization — loads config, creates DB, storage, session store |
| `config` | YAML config loading with JSON per-space overrides for cron |
| `cron` | gocron/v2 scheduler — full/incremental crawl jobs, per-space interval overrides |
| `db` | PostgreSQL access via pgx — models, migrations, FTS indexing |
| `embedder` | (Deferred) Vector embeddings — ONNX runtime, OpenAI, BGE models |
| `mcp` | MCP (SSE) transport — JSON-RPC 2.0, tools: `search_pages`, `get_page`, `list_spaces`, `crawl_space` |
| `scraper` | go-rod headless browser — discovery, page scraping, content extraction |
| `session` | Cookie capture and management — `tenant.session.token` extraction, CDP cookie injection |
| `storage` | File system storage — HTML, assets, metadata per space/page |

## Data Flow

### 1. Session Capture

```
Firefox (Confluence page)
  → browser.storage.cookies.get() (extension background)
  → POST /api/session (REST API)
  → AES-256-GCM encryption
  → session.enc (encrypted file, volume-mounted)
```

Key details:
- Uses Firefox native `browser` API (no polyfill)
- Captures all cookies, maps `Secure=true` cookies to `SameSite=None` (Atlassian requirement)
- Writes AES-GCM encrypted JSON to `session.enc` via `session/store.go`

### 2. Crawl Job

```
Extension popup (click Crawl)
  → POST /api/crawl (REST API)
  → CrawlJobManager creates in-memory job
  → CrawlRunner.Run() spawns go-rod browser
  → discoverSpace() parses space overview page
  → ScrapePage() for each page:
    → go-rod navigates to page
    → MustWaitStable() waits for network idle + DOM stability
    → extractContent() cleans HTML, extracts text, downloads images
    → UpsertPage() saves to PostgreSQL
    → UpdateSpaceLastCrawled() updates space metadata
  → Job status: pending → running → completed/failed
  → Progress reported via GET /api/crawl/status (polling every 3s)
```

Key details:
- go-rod over chromedp (chromedp `NoSandbox` fails in Colima vz driver)
- Session loaded from `session.enc` and injected via CDP `Page.setCookies`
- Pages stored as clean HTML + raw HTML + metadata JSON in `saved-data/{space}/{page}/`
- In-memory job tracking with mutex-protected progress updates

### 3. Search

```
User query
  → GET /api/search?q=<query>&space=<key>
  → pgx query: SELECT * FROM pages WHERE content_vector @@ plainto_tsquery($1)
  → ORDER BY ts_rank(content_vector, query) DESC
  → Returns ranked results with snippets
```

Key details:
- PostgreSQL `tsvector` with GIN index for BM25-style lexical search
- FTS triggered on each page save via `IndexPageContent()`
- No vector embeddings yet (deferred phase)

### 4. Cron Scheduling

```
Server startup
  → load config.yaml + cron-config.json
  → For each space in config:
    → CrawlerJobManager.RegisterCrawler(spaceURL, interval)
    → Creates CrawlJob in memory
    → Starts goroutine: time.NewTicker(interval) → RunJob(ctx, job)
```

Key details:
- gocron/v2 scheduler with per-space intervals
- `cron-config.json` (volume-mounted) stores per-space overrides
- Extension can update intervals via `POST /api/cron/space/{key}`

## Database Schema

### `spaces` — Tracked Confluence spaces

| Column | Type | Description |
|--------|------|-------------|
| `id` | UUID, PK | Internal ID |
| `key` | VARCHAR(10), UNIQUE | Space key (e.g., `NCHB`) |
| `name` | TEXT | Space display name |
| `url` | TEXT | Space overview URL |
| `last_crawled` | TIMESTAMP | Last successful crawl time |
| `created_at` | TIMESTAMP | Space creation time |

### `pages` — Crawled pages

| Column | Type | Description |
|--------|------|-------------|
| `id` | UUID, PK | Internal ID |
| `space_id` | UUID, FK → spaces | Associated space |
| `confluence_id` | INT | Confluence page ID |
| `title` | TEXT | Page title |
| `confluence_url` | TEXT | Original Confluence URL |
| `parent_confluence_id` | INT | Parent page ID |
| `content` | TEXT | Extracted plain text (FTS searchable) |
| `html_path` | TEXT | Local clean HTML file path |
| `raw_html_path` | TEXT | Local raw HTML file path |
| `metadata_path` | TEXT | Local metadata JSON file path |
| `content_vector` | tsvector, GIN | Full-text search vector |
| `created_at` | TIMESTAMP | Page creation time |
| `updated_at` | TIMESTAMP | Page update time |

### In-Memory Crawl Jobs

| Field | Type | Description |
|-------|------|-------------|
| `id` | UUID | Job ID |
| `space_url` | TEXT | Space overview URL |
| `status` | string | pending/running/completed/failed/cancelled |
| `total_pages` | int | Total pages discovered |
| `completed` | int | Pages successfully crawled |
| `failed` | int | Pages that failed |
| `progress` | int | Percentage (completed/total * 100) |
| `error` | string | Error message if failed |
| `created_at` | time.Time | Job creation time |
| `started_at` | time.Time | Job start time |
| `completed_at` | time.Time | Job completion time |
| `updated_at` | time.Time | Last update time |

## Security

### Session Encryption

- AES-256-GCM with 32-byte user-generated key
- Key from `config.yaml` `session.encryption_key`
- File permissions: `0600` (owner read/write only)
- Key loss = permanent session data loss

### Cookie Security

- `tenant.session.token` captured via `browser.cookies.getAll()`
- `SameSite=None` explicitly set for session token (Atlassian requirement)
- Cookies encrypted on disk, decrypted in memory only

### API Security

- No authentication on HTTP API (MVP)
- Intended to run behind reverse proxy with TLS and access controls

## Technology Decisions

### go-rod over chromedp

- chromedp `NoSandbox` fails in Colima vz driver (EPERM)
- go-rod `launcher.NoSandbox(true)` with explicit Chromium binary works
- `MustWaitStable()` improved with timeout to prevent indefinite hangs

### Firefox Extension Architecture

- No content_scripts (Confluence CSP blocks all injection)
- Background service worker handles all logic
- Popup communicates with background via `browser.runtime.sendMessage`
- Promise-based messaging for reliable async communication
- Space detection via URL parsing (not content scripts)
- Crawl state persisted via `browser.storage.local`

### PostgreSQL over SQLite

- Docker-native, concurrent access
- `tsvector` + GIN index for BM25-style search
- pgvector extension ready for future embeddings

### File-Based Session Storage

- Simple, portable, volume-mountable
- AES-GCM encryption provides security without external KV store

## Deployment

### Docker Compose

| Service | Image | Purpose |
|---------|-------|---------|
| `db` | `pgvector/pgvector:pg17` | PostgreSQL + pgvector |
| `app` | Built from Dockerfile | Go backend + scraper |

### Volume Mounts

| Host File | Container Path | Purpose |
|-----------|----------------|---------|
| `config.yaml` | `/app/config.yaml:ro` | Runtime config |
| `cron-config.json` | `/app/cron-config.json:rw` | Per-space cron overrides |
| `session.enc` | `/app/session.enc:rw` | Encrypted cookies |
| `saved-data/` | `/app/saved` | Crawled pages + assets |
| `pgdata/` | `/var/lib/postgresql/data` | PostgreSQL data |

### Firefox Extension

- Runs on host (not in Docker)
- Communicates with Docker backend via `http://localhost:8080`
- Loaded as temporary add-on via `about:debugging`
- Built via webpack → `firefox-extension/dist/`

## Known Limitations

1. **No vector embeddings** — Search uses BM25/lexical only (deferred phase)
2. **No API auth** — Intended for local/dev use behind reverse proxy
3. **Session expiry** — Cookies expire (~30 days), must re-capture
4. **Confluence CSP** — Blocks all content script injection, limits extension capabilities
5. **In-memory crawl jobs** — Jobs are not persisted across server restarts
6. **Single browser instance** — One go-rod browser per crawl job (no connection pooling)
