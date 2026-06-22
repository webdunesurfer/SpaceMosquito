# SpaceMosquito

[![SpaceMosquito](firefox-extension/assets/icon.svg)](https://github.com/webdunesurfer/SpaceMosquito)

Confluence space scraper, indexer, and search engine with automated cron scheduling. Captures pages via a headless browser, stores them locally, and indexes content for semantic (BM25) and lexical search. Exposes an MCP server for LLM integration and a Firefox extension for interactive session management and crawl control.

## Architecture at a Glance

```
┌──────────────────────────────────────────────────────────────────┐
│                      Host Machine                                │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │              Firefox (Browser)                             │  │
│  │  ┌──────────────────────────────────────────────────────┐  │  │
│  │  │  Pirate Mosquito (Web Extension)                     │  │  │
│  │  │  ┌──────────┐ ┌───────────┐ ┌───────────┐            │  │  │
│  │  │  │          | |           | |           |            │  │  │
│  │  │  | Session  │ │ Background│ │  Popup UI │            │  │  │
│  │  │  │ Handler  │ │  Worker   │ │  (Session │            │  │  │
│  │  │  │          │ │           │ │  /Crawl/  │            │  │  │
│  │  │  │          │ │           │ │  Cron)    │            │  │  │
│  │  │  └──────────┘ └───────────┘ └───────────┘            │  │  │
│  │  └──────────────────────────────────────────────────────┘  │  │
│  └────────────────────────────────────────────────────────────┘  │
│                                                                  │
├──────────────────────────────────────────────────────────────────┤
│                          Docker (Colima)                         │
│                                                                  │
│  ┌──────────────────────┐    ┌───────────────────────────────┐   │
│  │   app (Go Backend)   │    │     PostgreSQL + pgvector     │   │
│  │                      │    │                               │   │
│  │  HTTP API  :8081     │    │  ┌──────────┐  ┌──────────┐   │   │
│  │  MCP/SSE :8081       │◄───┤  │  spaces  │  │  pages   │   │   │
│  │                      │    │  │          │  │  +fts    │   │   │
│  │  Cron Scheduler      │    │  │  crawl   │  │          │   │   │
│  │  Headless Scraper    │    │  │  jobs    │  │          │   │   │
│  │  Storage (disk)      │    │  └──────────┘  └──────────┘   │   │
│  │  Session (AES-GCM)   │    └───────────────────────────────┘   │
│  └──────────────────────┘                                        │
│                                                                  │
│  Volumes:                                                        │
│    config.yaml          → runtime config                         │
│    cron-config.json     → per-space cron overrides               │
│    session.enc          → encrypted cookies                      │
│    saved-data/          → crawled pages + assets                 │
│    pgdata/              → PostgreSQL data                        │
└──────────────────────────────────────────────────────────────────┘
```

## Quick Start

### Prerequisites

- [Docker Desktop](https://docs.docker.com/desktop/) or [Colima](https://github.com/abiosoft/colima) (Apple Silicon)
- [Firefox](https://firefox.com) (for the browser extension)
- [git](https://git-scm.com/)

### 1. Clone and Build

```bash
git clone git@github.com:webdunesurfer/SpaceMosquito.git
cd SpaceMosquito
```

### 2. Configure

Copy the example config and edit for your environment:

```bash
cp config.yaml.example config.yaml
```

Edit `config.yaml`:

```yaml
database:
  host: db            # "db" inside Docker, "localhost" for local dev
  port: 5432
  user: spacemosquito
  password: spacemosquito
  dbname: spacemosquito
  sslmode: disable

storage:
  base_path: /app/saved   # Where crawled pages are stored

session:
  encryption_key: your-32-byte-key-here!  # AES-256, must be exactly 32 chars
  file_path: /app/session.enc

mcp:
  port: 8081
  host: "0.0.0.0"
  session_timeout: 3600

cron:
  full_crawl:
    enabled: false          # Set to true to enable scheduled full crawls
    interval: "24h"
    max_duration: "4h"
    spaces:
      - "https://your-company.atlassian.net/wiki/spaces/PROJ"
  incremental:
    enabled: false          # Set to true to enable scheduled incremental scans
    interval: "2h"
    max_duration: "30m"
    detection: "dom"        # "dom" | "api"
    spaces:
      - "https://your-company.atlassian.net/wiki/spaces/PROJ"
```

### 3. Start Infrastructure

```bash
docker compose up db -d
```

### 4. Run Migrations

```bash
docker compose exec app /app/cli init
```

This runs all database migrations against PostgreSQL.

### 5. Capture Session Cookies

1. Load the **Pirate Mosquito** extension in Firefox:
   - Open `about:debugging` in Firefox
   - Click **"This Firefox"** → **"Load Temporary Add-on..."**
   - Select `firefox-extension/dist/manifest.json`

2. Navigate to your Confluence space (e.g., `https://company.atlassian.net/wiki/spaces/PROJ/overview`)

3. Log in to Confluence if prompted

4. Open the extension popup (click the pirate mosquito icon) and go to the **Session** tab:
   - Click **"Capture Session"** — this grabs your Confluence cookies
   - Click **"Validate"** — posts the cookies to the backend for verification

5. The cookies are stored encrypted in `session.enc` (volume-mounted into the Docker container)

### 6. Start the Backend

```bash
docker compose up app -d
```

The server exposes:
- **HTTP API** on port `8081` (localhost)
- **MCP (SSE)** on port `8081` (localhost)

Verify: `curl http://localhost:8081/health` → `ok`

### 7. Run Your First Crawl

Via the extension popup (Crawl tab):
- Click **"Crawl Now"** to start an immediate crawl of the current space

Via CLI:
```bash
docker compose exec app /app/cli crawl "https://company.atlassian.net/wiki/spaces/PROJ"
```

Via API:
```bash
curl -X POST http://localhost:8081/api/crawl \
  -H "Content-Type: application/json" \
  -d '{"space_url": "https://company.atlassian.net/wiki/spaces/PROJ"}'
```

### 8. Search

Via CLI:
```bash
docker compose exec app /app/cli search "your query" [space-key]
```

Via API:
```bash
curl "http://localhost:8081/api/search?q=your+query&space_key=PROJ"
```

### 9. Connect an MCP Client

Configure your MCP client (opencode, Cursor, Gemini CLI, etc.) to connect to:

```
http://localhost:8081/mcp
```

The server provides tools for searching pages, retrieving page content, listing spaces, and triggering crawls.

## Firefox Extension

### Installation

#### Quick Install (Temporary)

1. Build the extension:
   ```bash
   cd firefox-extension
   npm install
   npm run build
   ```

2. Open `about:debugging` in Firefox

3. Click **"This Firefox"** → **"Load Temporary Add-on..."**

4. Select `firefox-extension/dist/manifest.json`

5. Pin the Pirate Mosquito icon to your toolbar

6. Open the popup to access Session, Crawl, Spaces, and Cron tabs

#### Persistent Install (Advanced)

To install as a permanent extension:

1. Build the extension as above

2. Edit `firefox-extension/manifest.json` and change `"update_url"` to your server URL

3. Install via Firefox Add-ons page (about:addons) → Gear icon → "Install Add-on From File..."

> Note: The extension must be loaded from the same machine as the backend (localhost:8081).

### Development

```bash
cd firefox-extension
npm install
npm run dev        # Watch mode, rebuilds on changes
npm run dev:firefox  # Auto-reload in Firefox via web-ext
```

### Popup Tabs

| Tab | Function |
|-----|----------|
| **Session** | Capture cookies from the current Confluence session, validate, and delete |
| **Crawl** | Trigger immediate crawls of the current space, view progress |
| **Spaces** | Add/remove Confluence spaces for automated crawling |
| **Cron** | Configure per-space crawl intervals and scheduling |

### Architecture

- **Background worker**: Service worker handling all logic (no content scripts due to Confluence CSP)
- **Popup UI**: Displays session status, crawl progress, space list, and cron config
- **Messaging**: Promise-based `browser.runtime.sendMessage` between popup and background
- **Storage**: `browser.storage.local` for extension state (active crawl ID, settings)

## Backend CLI

```bash
# Run inside Docker:
docker compose exec app /app/cli <command> [args]

# Or build locally:
cd space-mosquito && go build -o cli ./cmd/cli && ./cli <command> [args]
```

| Command | Description |
|---------|-------------|
| `init` | Run database migrations |
| `save <url>` | Save a single Confluence page |
| `crawl <url>` | Crawl a full Confluence space |
| `search <query> [space-key]` | Search pages (optionally filter by space) |
| `reindex` | Rebuild FTS indexes for all pages |
| `stats` | Show database statistics |
| `cron list` | List scheduled crawl jobs |
| `cron config` | Show cron configuration |
| `cron run-now` | Trigger all cron jobs immediately |
| `serve` | Start the API + MCP server |

## Configuration Reference

### config.yaml

All configuration options:

| Section | Key | Default | Description |
|---------|-----|---------|-------------|
| `database` | `host` | `localhost` | PostgreSQL host |
| | `port` | `5432` | PostgreSQL port |
| | `user` | `spacemosquito` | Database user |
| | `password` | `spacemosquito` | Database password |
| | `dbname` | `spacemosquito` | Database name |
| | `sslmode` | `disable` | SSL mode |
| `storage` | `base_path` | `./saved` | Directory for crawled pages |
| `session` | `encryption_key` | _(required)_ | AES-256 key (32 bytes) |
| | `file_path` | `~/.config/spacemosquito/session.enc` | Path to encrypted session file |
| `embedder` | `model` | `nomic-embed-text` | Embedding model (local ONNX) |
| | `openai.api_key` | _(empty)_ | OpenAI API key (optional) |
| | `openai.model` | `text-embedding-3-small` | OpenAI model name |
| | `bge.model_path` | `./models/bge-m3` | BGE model path (optional) |
| `mcp` | `port` | `8081` | MCP/HTTP port |
| | `host` | `0.0.0.0` | MCP/HTTP bind address |
| | `session_timeout` | `3600` | MCP session timeout (seconds) |
| `cron` | `full_crawl.enabled` | `false` | Enable full crawl scheduler |
| | `full_crawl.interval` | `24h` | Crawl interval (Go duration) |
| | `full_crawl.max_duration` | `4h` | Max crawl duration before timeout |
| | `full_crawl.spaces` | `[]` | List of space overview URLs |
| | `incremental.enabled` | `false` | Enable incremental scan scheduler |
| | `incremental.interval` | `2h` | Scan interval |
| | `incremental.max_duration` | `30m` | Max scan duration |
| | `incremental.detection` | `dom` | Change detection: `dom` (DOM diff) or `api` (Confluence API) |
| | `incremental.spaces` | `[]` | List of space overview URLs |

### Cron Overrides (cron-config.json)

Per-space interval overrides are stored in `cron-config.json` (volume-mounted from the host). This allows the Firefox extension to adjust crawl intervals without restarting the server.

Format:
```json
{
  "NCHB": {
    "interval": "6h",
    "type": "full"
  }
}
```

Keys are space keys. Values override the global cron config for that space.

## Docker Setup

### Compose Services

| Service | Image | Ports | Purpose |
|---------|-------|-------|---------|
| `db` | `pgvector/pgvector:pg17` | `5432` | PostgreSQL with vector extension |
| `app` | _(built from Dockerfile)_ | `8081, 8081` | Go backend + headless scraper |

### Volumes

| Volume | Mount | Host Path | Description |
|--------|-------|-----------|-------------|
| `config.yaml` | `/app/config.yaml:ro` | `./config.yaml` | Runtime config (read-only) |
| `cron-config.json` | `/app/cron-config.json:rw` | `./cron-config.json` | Per-space cron overrides |
| `session.enc` | `/app/session.enc:rw` | `./session.enc` | Encrypted session cookies |
| `saved-data` | `/app/saved` | _(Docker volume)_ | Crawled pages and assets |
| `pgdata` | `/var/lib/postgresql/data` | _(Docker volume)_ | PostgreSQL data |

### Building Locally

```bash
docker compose up --build app
```

### Local Development (without Docker)

```bash
# 1. Start PostgreSQL
docker compose up db -d

# 2. Run migrations
cd space-mosquito && go run ./cmd/cli init

# 3. Start the server
go run ./cmd/server

# 4. The extension communicates with http://localhost:8081
```

## API Reference

### Health

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |

### Session Management

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/session` | Capture and store session cookies |
| DELETE | `/api/session` | Delete stored session |
| GET | `/api/session/status` | Check session validity |
| POST | `/api/session/validate` | Validate existing session against Confluence |

### Search

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/search?q=<query>&space=<key>` | Search pages (BM25/lexical) |
| POST | `/api/search/reindex` | Rebuild FTS indexes |
| GET | `/api/search/stats` | Index statistics |

### Crawl

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/crawl` | Start a crawl job (`{space_url: "..."}`) |
| GET | `/api/crawl` | List all crawl jobs |
| GET | `/api/crawl/status?job_id=<id>` | Get job status and progress |
| POST | `/api/crawl/cancel` | Cancel a running job |
| POST | `/api/crawl/cleanup` | Remove completed/failed jobs |

### Cron

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/cron` | List active cron jobs |
| POST | `/api/cron/start` | Trigger all jobs immediately |
| GET | `/api/cron/config` | Get full cron configuration |
| POST | `/api/cron/config` | Update cron configuration |
| POST | `/api/cron/reload` | Reload and restart scheduler |

### Per-Space Cron

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/cron/space/{key}` | Set per-space cron override |
| GET | `/api/cron/space/{key}` | Get per-space cron override |
| DELETE | `/api/cron/space/{key}` | Remove per-space override |

### Spaces

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/spaces` | List all tracked spaces |
| POST | `/api/spaces` | Add a new space (`{url: "..."}`) |
| GET | `/api/spaces/{key}` | Get space details |
| DELETE | `/api/spaces/{key}` | Delete a space |

### MCP (Model Context Protocol)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/mcp` | Initiate session, receive SSE URL |
| POST | `/mcp/session/{id}` | JSON-RPC 2.0 requests with tool calls |

Available tools: `search_pages`, `get_page`, `list_spaces`, `crawl_space`.

## Troubleshooting

### Session Validation Fails

- Make sure you're logged into Confluence in Firefox before capturing cookies
- The `tenant.session.token` cookie must be present and not expired
- Check that `SameSite=None` is set on the session token (Atlassian requires this)
- Verify the encryption key in `config.yaml` matches the one used to encrypt `session.enc`

### Cron Jobs Not Running

- Ensure `full_crawl.enabled` or `incremental.enabled` is set to `true` in `config.yaml`
- Verify the space URLs in the `spaces` list are correct overview URLs
- Check the cron config file: `curl http://localhost:8081/api/cron`
- Trigger manually: `curl -X POST http://localhost:8081/api/cron/start`

### Crawl Hangs or Times Out

- Check the crawl job status: `curl "http://localhost:8081/api/crawl/status?job_id=<id>"`
- Increase `max_duration` in the cron config for large spaces
- Verify the session is valid: `curl http://localhost:8081/api/session/status`
- Check Docker logs: `docker compose logs app`

### Extension Can't Connect to Backend

- Ensure the backend is running: `curl http://localhost:8081/health`
- Check CORS is enabled — the backend includes a CORS middleware for extension requests
- In Firefox, open `about:debugging` and check the background service worker for errors

### PostgreSQL Connection Refused

- Make sure the `db` container is healthy: `docker compose ps db`
- Check Colima/Docker is running
- For local dev, ensure PostgreSQL is running on `localhost:5432`

### Storage Disk Space

- Crawled pages and assets accumulate in the `saved-data` volume
- Check usage: `docker volume inspect spacemosquito_saved-data`
- Clean up old pages via the API: `curl -X POST http://localhost:8081/api/crawl/cleanup`

## Database Schema

```
spaces          → tracked Confluence spaces
  ├─ id (UUID, PK)
  ├─ key (VARCHAR, UNIQUE)
  ├─ name (TEXT)
  ├─ url (TEXT)
  ├─ last_crawled (TIMESTAMP)
  └─ created_at (TIMESTAMP)

pages           → crawled pages
  ├─ id (UUID, PK)
  ├─ space_id (FK → spaces)
  ├─ confluence_id (INT)
  ├─ title (TEXT)
  ├─ confluence_url (TEXT)
  ├─ parent_confluence_id (INT)
  ├─ content (TEXT, FTS searchable)
  ├─ html_path (TEXT)
  ├─ raw_html_path (TEXT)
  ├─ metadata_path (TEXT)
  ├─ content_vector (tsvector, GIN indexed)  ← BM25/lexical search
  ├─ pages_crawled (INT)                       ← dynamic count from pages table
  └─ created_at / updated_at

crawl_jobs      → async crawl job records (in-memory)
  ├─ id (UUID, PK)
  ├─ space_url (TEXT)
  ├─ status (pending/running/completed/failed/cancelled)
  ├─ total_pages / completed / failed / progress
  ├─ error
  └─ created_at / started_at / completed_at / updated_at
```

\* Vector embeddings (`page_embeddings` table with IVFFlat index) are deferred to a later phase. Current search uses PostgreSQL `tsvector` with GIN indexing (BM25-style lexical search).

## Security

### Secrets Policy

The following files **must never be committed** to the repository:

| File | What it contains | .gitignore |
|------|-----------------|------------|
| `config.yaml` | Real encryption key, DB credentials, Confluence URLs | Yes |
| `cron-config.json` | Per-space cron overrides with real space URLs | Yes |
| `session.enc` | AES-256 encrypted Confluence cookies | Yes |
| `session.enc.bak` | Encrypted session backup | Yes |
| `.env` | Environment secrets | Yes |

Template files with safe defaults are provided:

| File | Purpose |
|------|---------|
| `.env.example` | Environment variable template — copy to `.env` and fill in values |
| `config.yaml.example` | Config template with empty encryption key — copy to `config.yaml` and customize |

### Encryption

- Session cookies are encrypted with **AES-256-GCM** using a 32-byte key from `config.yaml`
- The encryption key is user-generated — never use a default or predictable value
- File permissions are set to `0600` (owner read/write only)
- If you lose the encryption key, you cannot decrypt the stored session

### Generated Key

```bash
# Generate a cryptographically secure 32-byte key
openssl rand -base64 32
# or
head -c 32 /dev/urandom | base64
```

Use the output as your `session.encryption_key` in `config.yaml`.

### API Security

The HTTP API has no authentication. This is intentional for the MVP but should be addressed before production exposure behind a reverse proxy with TLS and access controls.

## License

Private / Internal use only.
