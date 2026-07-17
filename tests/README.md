# Tests

Automated tests live in Go under `space-mosquito/`. There are no Docker Compose or live Postgres smoke scripts.

## Unit tests (CI default)

```sh
cd space-mosquito
go test -race ./...
```

## Integration tests (REST + MCP, in-process SQLite)

```sh
cd space-mosquito
go test -race -tags=integration ./internal/app/...
```

See `DEVELOPMENT.md` and `DOCS/task-server-integration-tests.md`.
