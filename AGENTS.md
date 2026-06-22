
# Local Development & Build

```bash
cd space-mosquito
go build ./cmd/server
go build ./cmd/cli
```

# Testing with curl

When testing urls that have streaming mode e.g. `http://localhost:8081/mcp` , use `timeout` command to avoid hanging in endless waiting.
