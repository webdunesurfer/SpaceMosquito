# Task: Fix MCP SSE Compliance

## Objective
The current MCP SSE transport implementation is not strictly compliant with the official Model Context Protocol. While it opens an SSE stream, it fails to handle `initialize` correctly because it expects the POST request to return the JSON-RPC response in its HTTP body, whereas the standard dictates that POST requests should immediately return `202 Accepted` and all JSON-RPC responses must be sent via the SSE stream. This causes external agents (like Gemini CLI) to hang.

## Proposed Solution
1. **Refactor ClientSession**: Add a `SendChan chan []byte` to the `ClientSession` struct.
2. **Refactor GET /mcp**: 
   - Establish the SSE connection.
   - Run a loop that `select`s on `session.SendChan`, sending any received bytes as `event: message\ndata: ...\n\n`.
3. **Refactor POST /mcp/session/{id}**:
   - Immediately return `202 Accepted`.
   - Asynchronously process the JSON-RPC request (including `initialize`, `tools/list`, etc.).
   - Pass the resulting JSON-RPC responses to the `session.SendChan`.

## User Action Required
Once implemented, users can run:
`gemini mcp add --transport sse spacemosquito http://localhost:8081/mcp`
and the agent will correctly connect and discover tools.