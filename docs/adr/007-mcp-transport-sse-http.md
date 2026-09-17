# ADR-007: MCP Transport — SSE over HTTP

- **Status:** Superseded by [ADR-017](017-mcp-transport-streamable-http.md)
- **Date:** 2025-01-17

## Context

The MCP server needs to expose tools (search, get_page, list_spaces) to AI agent clients like opencode, Cursor, and Gemini CLI.

## Decision

Use Server-Sent Events (SSE) transport over HTTP for the MCP protocol (protocol
`2024-11-05` HTTP+SSE).

This decision is **superseded**: MCP deprecated HTTP+SSE in favor of Streamable
HTTP. See ADR-017.

## Alternatives considered

| Option | Why not (at the time) |
|--------|------------------------|
| Stdio transport | Only works for local processes |
| WebSocket | More complex connection management |
| HTTP POST only (no SSE) | Less efficient for streaming (pre–Streamable HTTP) |

## Consequences

Historical: in-memory SSE sessions, `/mcp` GET + `/mcp/session/<id>` POST.
Removed in the Streamable HTTP migration.
