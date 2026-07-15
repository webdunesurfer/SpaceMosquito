# Development

## Ground rules

- Breaking changes in API are acceptable.

## Local Development & Build

```sh
cd space-mosquito
go build ./cmd/server
go build ./cmd/cli
go build ./cmd/spacemosquito
```

Release binaries (cross-compile, embedded SQLite migrations):

```sh
cd space-mosquito
./scripts/build-release.sh v0.1.0
ls dist/
```

## Run unit tests

```sh
cd space-mosquito
go test ./...
```

With the race detector (same as CI):

```sh
cd space-mosquito
go test -race ./...
```

## Integration tests (REST + MCP, in-process)

Requires the `integration` build tag. Boots a real SQLite DB with embedded migrations, seeds fixtures, and exercises HTTP + MCP SSE.

```sh
cd space-mosquito
go test -race -tags=integration ./internal/app/...
```

Not run in CI by default. See `DOCS/task-server-integration-tests.md`.

## Search (SQLite / dockerless)

- Multi-word queries use **AND** — all terms must appear in title or body.
- **Title is weighted 10×** over body in BM25 ranking.
- Default result limit is **10** (CLI: `--limit N`; REST: `?limit=N`; MCP: `limit` field).
- If FTS returns no rows, search falls back to case-insensitive **title substring** match.

## Page content (Markdown)

Crawls and imports store page body text as **Markdown** (`content.md` on disk, `pages.content` in the DB) using HTML→Markdown conversion — not flat `doc.Text()` extraction. This preserves paragraph boundaries and improves search/MCP readability.

```
index.html  →  contentmd.HTMLToMarkdown()  →  content.md + pages.content  →  FTS
```

Regenerate existing catalogs after upgrade:

```sh
spacemosquito reindex --content
```

## Testing with curl

When testing urls that have streaming mode e.g. `http://localhost:8081/mcp` , use `timeout` command to avoid hanging in endless waiting.

Get a page by Confluence ID (REST):

```sh
curl -s http://localhost:8081/api/pages/42
curl -s "http://localhost:8081/api/pages/42?space_key=TST"
```

