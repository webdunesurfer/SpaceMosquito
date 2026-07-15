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

## Testing with curl

When testing urls that have streaming mode e.g. `http://localhost:8081/mcp` , use `timeout` command to avoid hanging in endless waiting.

Get a page by Confluence ID (REST):

```sh
curl -s http://localhost:8081/api/pages/42
curl -s "http://localhost:8081/api/pages/42?space_key=TST"
```

