# Guide: Configure MCP

SpaceMosquito exposes an **MCP** (Model Context Protocol) server over **HTTP + SSE**
so agents can search and read crawled Confluence content.

Default URL: `http://127.0.0.1:8081/mcp`

MCP tools read the **local catalog** (SQLite). Capture a session and crawl (or
import) spaces before expecting useful results. MCP does **not** start crawls;
use the CLI, browser extensions, or REST API for that.

## Prerequisites

1. `spacemosquito init` (once)
2. `spacemosquito serve` running
3. At least one space crawled (or `bootstrap import-saved`)

## Cursor

Add a remote MCP server — either:

- **Cursor Settings → Tools & MCP → Add**, or
- Edit `~/.cursor/mcp.json` (global) or `.cursor/mcp.json` (project)

```json
{
  "mcpServers": {
    "spacemosquito": {
      "url": "http://127.0.0.1:8081/mcp"
    }
  }
}
```

## Claude Code

**CLI:**

```sh
claude mcp add --transport sse spacemosquito http://127.0.0.1:8081/mcp
```

**Project** `.mcp.json` or user config (`~/.claude.json`):

```json
{
  "mcpServers": {
    "spacemosquito": {
      "type": "sse",
      "url": "http://127.0.0.1:8081/mcp"
    }
  }
}
```

Docs: [code.claude.com/docs/en/mcp](https://code.claude.com/docs/en/mcp).

## Gemini CLI

Configure under `mcpServers` in `~/.gemini/settings.json` (user) or
`.gemini/settings.json` (project). Use **`url` + `type: "sse"`** for this
server (not streamable-HTTP `httpUrl` alone).

**CLI:**

```sh
gemini mcp add --transport sse spacemosquito http://127.0.0.1:8081/mcp
# optional: -s user  → write to ~/.gemini/settings.json
```

**settings.json:**

```json
{
  "mcpServers": {
    "spacemosquito": {
      "type": "sse",
      "url": "http://127.0.0.1:8081/mcp"
    }
  }
}
```

Docs: [MCP servers with the Gemini CLI](https://google-gemini.github.io/gemini-cli/docs/tools/mcp-server.html).

## Other clients

Any client that supports **remote MCP over SSE** can use:

```text
http://127.0.0.1:8081/mcp
```

Field names vary (`url`, `serverUrl`, `type: "sse"`, etc.). No API key is
required for the default local bind.

## Tools

| Tool                     | Purpose                                                                                                                       |
| ------------------------ | ----------------------------------------------------------------------------------------------------------------------------- |
| `confluence_search`      | BM25/FTS search. Args: `query` (required), optional `space`, `limit` (default 10). Multi-word queries are **AND**.            |
| `confluence_list_spaces` | List crawled spaces.                                                                                                          |
| `confluence_list_space`  | Page summaries in a space (cursor pagination). Args: `space_key`, optional `limit`, `after_confluence_id`, `include_content`. |
| `confluence_get_page`    | Full page by `confluence_id`; optional `space_key` if IDs collide across spaces.                                              |


Typical flow: search → take `confluence_id` → `confluence_get_page`.

Page `content` is **Markdown** (same as CLI / REST).

## Smoke test without an IDE

Keep SSE open in one terminal and note the session path from the `endpoint` event:

```sh
timeout 5 curl -sN http://127.0.0.1:8081/mcp
```

Example output:

```text
event: endpoint
data: /mcp/session/<uuid>
```

Then POST JSON-RPC (replace `<uuid>`):

```sh
curl -s -X POST "http://127.0.0.1:8081/mcp/session/<uuid>" \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}'
```

The HTTP response is `202 Accepted`; the tool list arrives on the SSE stream.
Use `timeout` on the GET so curl does not hang forever (see [DEVELOPMENT.md](../DEVELOPMENT.md)).

## Troubleshooting


| Symptom                 | Check                                                                                    |
| ----------------------- | ---------------------------------------------------------------------------------------- |
| Client cannot connect   | `spacemosquito serve` running? URL host/port match `mcp.host` / `mcp.port`?              |
| Tools empty / no spaces | Crawl or import at least one space; `spacemosquito stats`                                |
| Search returns nothing  | Reindex after upgrade: `spacemosquito reindex` (and `--content` if Markdown looks wrong) |
| SSE drops after idle    | Raise `mcp.session_timeout` or reconnect (clients usually reopen `/mcp`)                 |
| Want LAN access         | Set `mcp.host: "0.0.0.0"` and use the machine’s IP in the client URL — no auth           |




## Related

- [README](../../README.md) — install, crawl, search
- [ARCHITECTURE.md](../ARCHITECTURE.md) — MCP package overview
- [ADR-007](../../adr/007-mcp-transport-sse-http.md) — transport decision

