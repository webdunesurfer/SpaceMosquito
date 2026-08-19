# ADR-011: Go Backend Language Choice

- **Status:** Accepted
- **Date:** 2025-01-17

## Context

We need a backend for the SpaceMosquito project that handles API serving, database operations, MCP server, and cron scheduling.

## Decision

Use Go for all backend logic.

- Matches user preference and strong language fit for the required components:
  - Single binary deployment
  - Built-in HTTP server (API + MCP)
  - Strong SQLite ecosystem (modernc) plus FTS5
  - Strong concurrency model for parallel scraping and batch processing
  - Fast compilation and low memory footprint
  - No external runtime dependencies
- `golang-migrate` for migrations
- `go-rod` for headless scraping fallback
- `gocron` for cron scheduling
- Standard library SSE support for MCP server

## Alternatives considered

| Option | Why not |
|--------|---------|
| Python | Rich ML/embedding ecosystem but slower, heavier, and less ideal for HTTP servers |
| Node.js | Good for the extension but not ideal for database/ML workloads |
| Rust | Excellent performance but longer development time, less mature Playwright bindings |
| TypeScript (Node) | Embedding generation would require Python/ONNX runtime integration anyway |

## Consequences

- All backend services (API, MCP, scraper, cron) in one Go codebase
- One local binary process; no container runtime required
- Go's static typing helps maintain large codebases over time
