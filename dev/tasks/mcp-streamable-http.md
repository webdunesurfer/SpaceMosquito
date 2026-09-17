# MCP — drop HTTP+SSE, switch to Streamable HTTP

- **Task ID:** `mcp-streamable-http`
- **Status:** done
- **Parent:** —
- **Blocked by:** —

## Goal

**Break** old HTTP+SSE. Serve MCP **only** via Streamable HTTP on `POST /mcp`.

## Decisions (locked)

| # | Decision |
|---|----------|
| 1 | Drop HTTP+SSE entirely |
| 2 | Spec wire `2026-07-28` (POST-only, no sessions/GET stream) |
| 3 | `application/json` responses; SSE streams deferred |
| 4 | Stateless |
| 5 | Origin validation (loopback only when present) |
| 6 | Docs rewritten for Streamable HTTP |
| 7 | No release in-task — CHANGELOG Unreleased; major later |

## Shipped

- `internal/mcp`: Streamable HTTP `HandleRequest`; removed session map / SSE.
- App routing: `/mcp` only (no `/mcp/session/`).
- Tests + `testutil.ConnectMCP` rewritten.
- ADR-017; ADR-007 superseded; configure-mcp / README / ARCHITECTURE / CHANGELOG.

## Done when

- [x] No HTTP+SSE session lifecycle
- [x] `POST /mcp` initialize + tools/list + tool calls (JSON)
- [x] Unit + integration MCP tests green
- [x] Docs/ADR rewritten
- [x] CHANGELOG `[Unreleased]` breaking note
