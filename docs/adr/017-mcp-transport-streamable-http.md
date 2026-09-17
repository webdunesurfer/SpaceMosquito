# ADR-017: MCP Transport — Streamable HTTP

- **Status:** Accepted
- **Date:** 2026-09-16
- **Related:** Supersedes [ADR-007](007-mcp-transport-sse-http.md)

## Context

MCP remote transport moved from HTTP+SSE (`2024-11-05`) to **Streamable HTTP**
(`2025-03-26+`). The old long-lived SSE + `/mcp/session/<id>` design is
deprecated. SpaceMosquito still documented and implemented HTTP+SSE.

## Decision

Serve MCP **only** via Streamable HTTP on a single `POST /mcp` endpoint:

| Topic | Choice |
|-------|--------|
| Spec wire shape | `2026-07-28` style: POST-only, no protocol sessions, no standalone GET stream |
| Responses | `application/json` JSON-RPC body (request-scoped SSE deferred) |
| Sessions | Stateless — no `MCP-Session-Id` |
| Security | Reject non-loopback `Origin` when the header is present |
| Compat | No HTTP+SSE dual-stack |

Advertised `protocolVersion` in `initialize`: `2025-03-26` (Streamable HTTP era;
clients negotiate via `MCP-Protocol-Version`).

## Alternatives considered

| Option | Why not |
|--------|---------|
| Keep HTTP+SSE + dual-stack | Extra surface; deprecated; clients already prefer Streamable HTTP |
| Request-scoped `text/event-stream` | Not needed for sync catalog tools; defer |
| Protocol sessions | Local tools are request-scoped; unnecessary state |
| stdio transport | Does not fit remote/`serve` model |

## Consequences

- Clients must use Streamable HTTP (`type: http` / streamable-http), not `--transport sse`.
- `/mcp/session/*` and SSE `endpoint` events are gone (breaking).
- `mcp.session_timeout` is unused for MCP transport (legacy YAML key ignored).
- Docs: [Configure MCP](../guides/configure-mcp.md).
