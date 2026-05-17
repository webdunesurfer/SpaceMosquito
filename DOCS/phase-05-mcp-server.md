# Phase 5: MCP Server (SSE)

## Objective
Implement the Model Context Protocol server with SSE transport, exposing Confluence search and page retrieval tools to AI agents.

## Deliverables
- MCP JSON-RPC 2.0 server with SSE transport
- MCP endpoints: `/mcp` (session initiation) and `/mcp/session/<id>` (requests)
- Tools: `confluence_search`, `confluence_get_page`, `confluence_list_space`, `confluence_list_spaces`
- Compatible with opencode, Cursor, Gemini CLI

## Tasks

### 5.1 — MCP Server Setup
- `internal/mcp/server.go`:
  - HTTP server on port 8081 (configurable)
  - Session management: each client gets a unique session ID
  - Session storage: in-memory map (sessionID → client session state)
  - Session timeout: 1 hour of inactivity
- SSE transport implementation:
  - `GET /mcp` — not used in SSE mode; clients POST to `/mcp`
  - `POST /mcp` — initiate session, respond with JSON:
    ```json
    {
      "jsonrpc": "2.0",
      "id": 1,
      "result": {
        "url": "http://localhost:8081/mcp/session/<sessionId>"
      }
    }
    ```
  - `POST /mcp/session/<id>` — receive JSON-RPC 2.0 requests
  - Stream responses back via SSE to the session URL

### 5.2 — JSON-RPC 2.0 Handling
- `internal/mcp/server.go`:
  - Parse incoming JSON-RPC requests
  - Support methods: `initialize`, `notifications/initialized`, `tools/list`, `tools/call`, `prompts/list`, `prompts/get`
  - Response format:
    ```json
    {
      "jsonrpc": "2.0",
      "id": <request_id>,
      "result": { ... }
    }
    ```
  - Error responses:
    ```json
    {
      "jsonrpc": "2.0",
      "id": <request_id>,
      "error": { "code": -32601, "message": "Method not found" }
    }
    ```

### 5.3 — Tools Registration
- `internal/mcp/tools.go`:
  - Register tools on server initialization:
    ```go
    tools := []Tool{
        {
            Name:        "confluence_list_spaces",
            Description: "List all crawled Confluence spaces",
            InputSchema: json.RawMessage(`{"type": "object", "properties": {}}`),
        },
        {
            Name:        "confluence_list_space",
            Description: "List all pages in a Confluence space",
            InputSchema: json.RawMessage(`{
                "type": "object",
                "properties": {
                    "space_key": {"type": "string"},
                    "limit": {"type": "integer", "default": 50}
                },
                "required": ["space_key"]
            }`),
        },
        {
            Name:        "confluence_search",
            Description: "Search Confluence pages using semantic and keyword search",
            InputSchema: json.RawMessage(`{
                "type": "object",
                "properties": {
                    "query": {"type": "string"},
                    "space_key": {"type": "string"},
                    "limit": {"type": "integer", "default": 10}
                },
                "required": ["query"]
            }`),
        },
        {
            Name:        "confluence_get_page",
            Description: "Get the full content of a specific Confluence page",
            InputSchema: json.RawMessage(`{
                "type": "object",
                "properties": {
                    "space_key": {"type": "string"},
                    "page_id": {"type": "integer"},
                    "title": {"type": "string"}
                },
                "required": ["space_key"]
            }`),
        },
    }
    ```

### 5.4 — Tool Implementations
- `internal/mcp/tools.go`:

  **confluence_list_spaces:**
  - Query all rows from `spaces` table
  - Return: `[{ key, name, url, last_crawled }]`

  **confluence_list_space:**
  - Join `spaces` and `pages` on space_id
  - Filter by space_key
  - Limit results
  - Return: `[{ id, title, page_id, updated_at, file_path }]`

  **confluence_search:**
  - Generate query embedding using the configured embedder
  - Execute pgvector cosine similarity search
  - Fallback to keyword search (LIKE / ILIKE / tsvector) for pages without embeddings
  - Return: `[{ page_id, title, space_key, excerpt, similarity_score, file_path }]`

  **confluence_get_page:**
  - Query page by (space_key, page_id) or (space_key, title)
  - Return full page data including content, metadata, and local file path
  - Include the local HTML file path for offline viewing

### 5.5 — MCP Prompts (Optional)
- `internal/mcp/prompts.go`:
  - Pre-built prompts for common queries:
    - `confluence_summary` — summarize a space
    - `confluence_compare` — compare content between pages
  - Not required for v1 but useful for agents

### 5.6 — Server Integration
- `cmd/server/main.go`:
  - Start MCP server on port 8081
  - Run alongside API server (port 8080)
  - Both share the same DB and embedder instances
  - Graceful shutdown: close sessions, flush buffers

### 5.7 — Configuration
- `config.yaml`:
  ```yaml
  mcp:
    port: 8081
    host: "0.0.0.0"
    session_timeout: 3600  # seconds
  ```

## Acceptance Criteria
- MCP server starts and responds to `/mcp` POST with a session URL
- `tools/list` returns all 4 registered tools with correct schemas
- `tools/call` for `confluence_list_spaces` returns crawled spaces
- `tools/call` for `confluence_search` returns relevant results
- Compatible with Cursor MCP client (verified by testing)
- Session management works (timeout, cleanup)
