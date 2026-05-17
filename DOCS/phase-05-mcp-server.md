# Phase 5: MCP Server (SSE)

## Objective
Implement the Model Context Protocol server with SSE transport, exposing Confluence search and page retrieval tools to AI agents.

## Deliverables
- MCP JSON-RPC 2.0 server with SSE transport
- MCP endpoints: `/mcp` (session initiation) and `/mcp/session/<id>` (requests)
- Tools: `confluence_search`, `confluence_get_page`, `confluence_list_space`, `confluence_list_spaces`
- Compatible with opencode, Cursor, Gemini CLI
- Structured logging for all MCP operations

## Logging Strategy
- Use `logging.Sugar` injected via MCP server constructor
- Log session lifecycle (create, timeout, cleanup) with session_id
- Log all tool calls: method name, input params (sanitized), duration, result count
- Log errors with tool name and error details
- Include correlation_id from HTTP request in all MCP session logs

## Tasks

### 5.1 — MCP Server Setup
- `internal/mcp/server.go`:
  - HTTP server on port 8081 (configurable)
  - Session management: each client gets a unique session ID
  - Session storage: in-memory map (sessionID → client session state)
  - Session timeout: 1 hour of inactivity
  - SSE transport implementation:
    - `GET /mcp` — not used in SSE mode; clients POST to `/mcp`
    - `POST /mcp` — initiate session, respond with JSON
    - `POST /mcp/session/<id>` — receive JSON-RPC 2.0 requests
    - Stream responses back via SSE to the session URL
  - **Log session creation/deletion/timeout with session_id, client IP, user_agent**

### 5.2 — JSON-RPC 2.0 Handling
- `internal/mcp/server.go`:
  - Parse incoming JSON-RPC requests
  - Support methods: `initialize`, `notifications/initialized`, `tools/list`, `tools/call`, `prompts/list`, `prompts/get`
  - Response format and error responses per MCP spec
  - **Log all JSON-RPC requests and responses with method, request_id, session_id, duration_ms, error (if any)**

### 5.3 — Tools Registration
- `internal/mcp/tools.go`:
  - Register tools on server initialization
  - Tool struct with name, description, inputSchema
  - **Log tools registration with tool count and names**

### 5.4 — Tool Implementations
- `internal/mcp/tools.go`:

  **confluence_list_spaces:**
  - Query all rows from `spaces` table
  - Return: `[{ key, name, url, last_crawled }]`
  - **Log execution with space count, query duration**

  **confluence_list_space:**
  - Join `spaces` and `pages` on space_id
  - Filter by space_key, limit results
  - Return: `[{ id, title, page_id, updated_at, file_path }]`
  - **Log execution with space_key, page count, query duration**

  **confluence_search:**
  - Generate query embedding using the configured embedder
  - Execute pgvector cosine similarity search
  - Fallback to keyword search for pages without embeddings
  - Return: `[{ page_id, title, space_key, excerpt, similarity_score, file_path }]`
  - **Log embedding generation time, search duration, result count, fallback usage**

  **confluence_get_page:**
  - Query page by (space_key, page_id) or (space_key, title)
  - Return full page data including content, metadata, local file path
  - **Log execution with page lookup time, file size**

### 5.5 — MCP Prompts (Optional)
- `internal/mcp/prompts.go`:
  - Pre-built prompts for common queries
  - **Log prompt selection and usage**

### 5.6 — Server Integration
- `cmd/server/main.go`:
  - Start MCP server on port 8081
  - Run alongside API server (port 8080)
  - Both share the same DB and embedder instances
  - Graceful shutdown: close sessions, flush buffers
  - **Log MCP server startup/shutdown, session cleanup on shutdown**

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
- All MCP operations logged with structured fields (session_id, method, duration, tool_name)
