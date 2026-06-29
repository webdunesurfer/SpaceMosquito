# Development

## Ground rules

- Breaking changes in API are acceptable.

## Local Development & Build

```sh
cd space-mosquito
go build ./cmd/server
go build ./cmd/cli
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

## Testing with curl

When testing urls that have streaming mode e.g. `http://localhost:8081/mcp` , use `timeout` command to avoid hanging in endless waiting.
