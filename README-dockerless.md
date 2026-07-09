# SpaceMosquito — Dockerless Install

Run SpaceMosquito locally without Docker. All state lives under `~/.spacemosquito/` (or a portable `--data-dir`).

## Requirements

- [Go](https://go.dev/dl/) 1.25+ (for building from source)
- Confluence access via the **Pirate Mosquito** Firefox extension (sideload)
- macOS, Linux, or Windows x64

## Install

### From source (recommended for local development)

```sh
git clone https://github.com/vkh/spacemosquito.git
cd spacemosquito/space-mosquito
go build -o spacemosquito ./cmd/spacemosquito
```

Run from the repo (no install step):

```sh
./spacemosquito init
./spacemosquito serve
```

Or install onto your `PATH`:

```sh
# macOS / Linux
sudo mv spacemosquito /usr/local/bin/

# Windows (PowerShell, from space-mosquito/)
go build -o spacemosquito.exe ./cmd/spacemosquito
# add the directory containing spacemosquito.exe to PATH
```

### Pre-built release (optional)

1. Download the binary for your platform from [GitHub Releases](https://github.com/vkh/spacemosquito/releases).
2. Verify the checksum against `SHA256SUMS` in the release assets.
3. Make it executable (macOS/Linux):

```sh
chmod +x spacemosquito-darwin-arm64   # or your platform artifact
sudo mv spacemosquito-darwin-arm64 /usr/local/bin/spacemosquito
```

On Windows, rename `spacemosquito-windows-amd64.exe` to `spacemosquito.exe` and add it to your `PATH`.

## First run

```sh
# Create ~/.spacemosquito/, config.yaml, SQLite DB, and session file
spacemosquito init

# Optional: pre-download Chromium (~150 MB) for offline API-fallback crawls
spacemosquito init --download-browser
```

`init` prints a generated **encryption key** once. Save it — you need the same value in `config.yaml` for session decryption.

Portable mode:

```sh
spacemosquito init --data-dir ./data
SPACEMOSQUITO_DATA_DIR=./data spacemosquito serve
```

## Capture a Confluence session

1. Start the server: `spacemosquito serve`
2. Open Confluence in Firefox with the Pirate Mosquito extension installed.
3. Use the extension to send cookies to `http://localhost:8081`.

## Crawl a space

```sh
spacemosquito crawl "https://your-domain.atlassian.net/wiki/spaces/SPACEKEY"
```

Or trigger a crawl via MCP at `http://localhost:8081/mcp`.

## Search

```sh
spacemosquito search "your query"
spacemosquito search "your query" SPACEKEY
```

## Commands

| Command | Description |
|---------|-------------|
| `init` | Create data directory, config, migrations |
| `bootstrap import-saved` | Rebuild SQLite catalog from existing `saved/` files |
| `serve` | Start API + MCP server |
| `crawl <url>` | Crawl a Confluence space |
| `search <query>` | Full-text search |
| `stats` | Database statistics |
| `version` | Print build version |

Run `spacemosquito` with no arguments for the full command list.

## Migrate from Docker (import saved files)

If you already crawled with Docker, you can reuse the `saved/` bind-mount instead of recrawling. Copy your saved pages into the dockerless data directory, then import:

```sh
# 1. Initialize dockerless layout (creates ~/.spacemosquito and SQLite DB)
spacemosquito init

# 2. Copy your existing crawl artifacts (example: from Docker volume mount)
cp -R /path/to/docker/saved/* ~/.spacemosquito/saved/

# 3. Import metadata + HTML into SQLite and rebuild FTS
spacemosquito bootstrap import-saved

# 4. Verify and serve
spacemosquito stats
spacemosquito search "your query"
spacemosquito serve
```

Useful flags:

```sh
# Import from a non-default location
spacemosquito bootstrap import-saved --from /path/to/saved

# DB already has pages (re-import)
spacemosquito bootstrap import-saved --force

# Scan and report only (no DB writes)
spacemosquito bootstrap import-saved --dry-run
```

You can also run import during init:

```sh
spacemosquito init --bootstrap-mode=import_saved --from ~/.spacemosquito/saved
```

Import writes a machine-readable report to `~/.spacemosquito/reports/bootstrap-import-<timestamp>.json`.

**Note:** This imports from on-disk `saved/` files only. It does not read directly from PostgreSQL. If you only have a Postgres volume and no `saved/` tree, use `recrawl` instead.

## Environment

| Variable | Purpose |
|----------|---------|
| `SPACEMOSQUITO_DATA_DIR` | Data directory (default `~/.spacemosquito`) |
| `CONFIG_PATH` | Config file path |
| `CHROMIUM_PATH` | Override browser executable |

## Docker / Postgres mode

For developers who prefer Docker Compose and PostgreSQL, see the main [README.md](README.md).
