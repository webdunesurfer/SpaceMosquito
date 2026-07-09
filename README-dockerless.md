# SpaceMosquito — Dockerless Install

Run SpaceMosquito locally without Docker. All state lives under `~/.spacemosquito/` (or a portable `--data-dir`).

## Requirements

- Confluence access via the **Pirate Mosquito** Firefox extension (sideload)
- macOS, Linux, or Windows x64

## Install

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
| `serve` | Start API + MCP server |
| `crawl <url>` | Crawl a Confluence space |
| `search <query>` | Full-text search |
| `stats` | Database statistics |
| `version` | Print build version |

Run `spacemosquito` with no arguments for the full command list.

## Environment

| Variable | Purpose |
|----------|---------|
| `SPACEMOSQUITO_DATA_DIR` | Data directory (default `~/.spacemosquito`) |
| `CONFIG_PATH` | Config file path |
| `CHROMIUM_PATH` | Override browser executable |

## Docker / Postgres mode

For developers who prefer Docker Compose and PostgreSQL, see the main [README.md](README.md).
