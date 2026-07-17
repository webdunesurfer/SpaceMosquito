# SpaceMosquito

[![SpaceMosquito](firefox-extension/assets/icon.svg)](https://github.com/webdunesurfer/SpaceMosquito)

Confluence space scraper, indexer, and search engine. Uses the Confluence REST API for content extraction (with headless browser fallback), stores pages locally in **SQLite + FTS5**, and exposes an MCP server plus browser extensions for session management and crawl control.

All state lives under `~/.spacemosquito/` (or a portable `--data-dir`). Docker and PostgreSQL are **not** supported.

## Architecture at a Glance

```
┌──────────────────────────────────────────────────────────────────┐
│                      Host Machine                                │
│                                                                  │
│  Firefox / Chrome + Pirate Mosquito extension                    │
│       │ cookies / crawl UI                                       │
│       ▼                                                          │
│  spacemosquito (binary)                                          │
│    HTTP API + MCP  :8081                                         │
│    Cron · Scraper (API + rod fallback) · Session (AES-GCM)       │
│       │                                                          │
│       ▼                                                          │
│  ~/.spacemosquito/                                               │
│    config.yaml · spacemosquito.db (SQLite+FTS5) · session.enc    │
│    saved/ · browser/ (optional Chromium) · cron-config.json      │
└──────────────────────────────────────────────────────────────────┘
```

See [ARCHITECTURE.md](ARCHITECTURE.md) for details.

## Requirements

- [Go](https://go.dev/dl/) 1.25+ (to build from source)
- Firefox or Chrome (for the Pirate Mosquito extension)
- macOS, Linux, or Windows x64

## Install

### From source

```sh
git clone https://github.com/webdunesurfer/SpaceMosquito.git
cd SpaceMosquito
make build
# binary: build/spacemosquito
```

Or:

```sh
cd space-mosquito
go build -o spacemosquito ./cmd/spacemosquito
```

### Pre-built release (optional)

1. Download the binary for your platform from [GitHub Releases](https://github.com/webdunesurfer/SpaceMosquito/releases).
2. Verify the checksum against `SHA256SUMS` in the release assets.
3. Install onto your `PATH` (example for macOS/Linux):

```sh
chmod +x spacemosquito-darwin-arm64
sudo mv spacemosquito-darwin-arm64 /usr/local/bin/spacemosquito
```

## First run

```sh
spacemosquito init
# Optional: pre-download Chromium (~150 MB) for API-fallback crawls
spacemosquito init --download-browser

spacemosquito serve
```

`init` prints a generated **encryption key** once. Save it — the same value must remain in `config.yaml` for session decryption.

Portable mode:

```sh
spacemosquito init --data-dir ./data
SPACEMOSQUITO_DATA_DIR=./data spacemosquito serve
```

## Capture a Confluence session

1. Start the server: `spacemosquito serve`
2. Load the Pirate Mosquito extension (see below).
3. Open Confluence and use the extension to send cookies to `http://localhost:8081`.

### Firefox (temporary)

```sh
cd firefox-extension && npm install && npm run build
```

Open `about:debugging` → **This Firefox** → **Load Temporary Add-on…** → select `firefox-extension/dist/manifest.json`.

### Chrome (temporary)

```sh
cd chrome-extension && npm install && npm run build
```

Open `chrome://extensions` → Developer mode → **Load unpacked** → select `chrome-extension/dist/`.

## Crawl a space

```sh
spacemosquito crawl "https://your-domain.atlassian.net/wiki/spaces/SPACEKEY"
```

Or trigger a crawl via the extension or MCP at `http://localhost:8081/mcp`.

## Search

Multi-word queries match **all** terms (AND). Title matches rank above body-only hits. Default: 10 results.

Page **content** is stored as Markdown (`content.md` on disk, `pages.content` in the DB). After upgrading, regenerate existing pages:

```sh
spacemosquito reindex --content
```

```sh
spacemosquito search "your query"
spacemosquito search "your query" SPACEKEY
spacemosquito search "your query" --limit 50
```

REST and MCP also accept `limit` (`GET /api/search?q=...&limit=50`, MCP `confluence_search` `limit` field).

## Get a page by Confluence ID

```sh
spacemosquito get-page 250347937
spacemosquito get-page 42 TST

curl -s http://localhost:8081/api/pages/250347937
curl -s "http://localhost:8081/api/pages/42?space_key=TST"
```

## Commands

| Command | Description |
|---------|-------------|
| `init` | Create data directory, config, migrations |
| `bootstrap import-saved` | Rebuild SQLite catalog from existing `saved/` files |
| `serve` | Start API + MCP server |
| `crawl <url>` | Crawl a Confluence space |
| `search <query>` | Full-text search (`--limit N`; multi-word AND) |
| `get-page <id>` | Get page by Confluence ID (optional space key) |
| `reindex` | Rebuild FTS indexes (`--content` regenerates Markdown from saved HTML) |
| `stats` | Database statistics |
| `version` | Print build version |

Run `spacemosquito` with no arguments for the full command list.

## Coming from Docker?

Docker Compose / PostgreSQL mode has been removed. To keep crawl artifacts without recrawling:

1. Wipe leftover containers/volumes (optional): [`scripts/cleanup-docker-legacy.sh`](scripts/cleanup-docker-legacy.sh) — see [`DOCS/guides/cleanup-docker-legacy.md`](DOCS/guides/cleanup-docker-legacy.md)
2. `spacemosquito init`
3. Copy your old Compose bind-mount `saved-data/` (or `./saved`) → `~/.spacemosquito/saved/`
4. `spacemosquito bootstrap import-saved`
5. `spacemosquito reindex --content`
6. Point the extension at `http://localhost:8081` and run `spacemosquito serve`

Useful flags:

```sh
spacemosquito bootstrap import-saved --from /path/to/saved
spacemosquito bootstrap import-saved --force
spacemosquito bootstrap import-saved --dry-run
```

Import does **not** read PostgreSQL. If you only have a Postgres volume and no `saved/` tree, recrawl instead.

Full removal overview: [`DOCS/task-remove-docker-mode.md`](DOCS/task-remove-docker-mode.md).

## Environment

| Variable | Purpose |
|----------|---------|
| `SPACEMOSQUITO_DATA_DIR` | Data directory (default `~/.spacemosquito`) |
| `CONFIG_PATH` | Config file path |
| `CHROMIUM_PATH` | Override browser executable |

## Development

See [DEVELOPMENT.md](DEVELOPMENT.md). Quick checks:

```sh
make test
cd space-mosquito && go test -race -tags=integration ./internal/app/...
```
