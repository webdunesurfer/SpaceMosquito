#!/bin/bash
set -e

echo "============================================"
echo "Phase 5: MCP Server - Test Suite"
echo "============================================"
echo ""

# Wait for server to be ready
echo "Waiting for server to be ready..."
for i in {1..10}; do
    if curl -s "http://localhost:8081/health" > /dev/null 2>&1; then
        echo "✓ Server is ready"
        break
    fi
    echo "  Waiting... ($i/10)"
    sleep 3
done

# Test 1: MCP Session Init
echo "Test 1: MCP Session Init"
SESSION_INIT=$(curl -s -X POST "http://localhost:8081/mcp" -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","method":"initialize","params":{},"id":1}' 2>/dev/null)
SESSION_ID=$(echo "$SESSION_INIT" | python3 -c "import sys, json; print(json.load(sys.stdin)['result']['session_id'])" 2>/dev/null)
if [ -n "$SESSION_ID" ]; then
    echo "✓ MCP Session Init works (session_id: $SESSION_ID)"
else
    echo "✗ MCP Session Init failed"
    echo "Response: $SESSION_INIT"
    exit 1
fi

# Test 2: MCP tools/list
echo "Test 2: MCP tools/list"
TOOLS_LIST=$(curl -s -X POST "http://localhost:8081/mcp/session/$SESSION_ID" -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","method":"tools/list","params":{},"id":2}' 2>/dev/null)
# Extract JSON from SSE format (data: prefix)
TOOLS_JSON=$(echo "$TOOLS_LIST" | grep "^data:" | sed 's/^data: //' || echo "$TOOLS_LIST")
TOOL_COUNT=$(echo "$TOOLS_JSON" | python3 -c "import sys, json; data = json.load(sys.stdin)['result']['tools']; print(len(data))" 2>/dev/null)
if [ "$TOOL_COUNT" = "4" ]; then
    echo "✓ MCP tools/list works (4 tools registered)"
else
    echo "✗ MCP tools/list failed (expected 4 tools, got $TOOL_COUNT)"
    echo "Response: $TOOLS_LIST"
    exit 1
fi

# Test 3: MCP confluence_list_spaces
echo "Test 3: MCP confluence_list_spaces"
LIST_SPACES=$(curl -s -X POST "http://localhost:8081/mcp/session/$SESSION_ID" -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","method":"tools/call","params":{"tool_name":"confluence_list_spaces","arguments":{}},"id":3}' 2>/dev/null)
LIST_SPACES_JSON=$(echo "$LIST_SPACES" | grep "^data:" | sed 's/^data: //' || echo "$LIST_SPACES")
if echo "$LIST_SPACES_JSON" | python3 -c "import sys, json; data = json.load(sys.stdin); assert 'result' in data and 'content' in data['result']" 2>/dev/null; then
    echo "✓ MCP confluence_list_spaces works"
else
    echo "✗ MCP confluence_list_spaces failed"
    echo "Response: $LIST_SPACES"
    exit 1
fi

# Test 4: MCP confluence_search
echo "Test 4: MCP confluence_search"
SEARCH=$(curl -s -X POST "http://localhost:8081/mcp/session/$SESSION_ID" -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","method":"tools/call","params":{"tool_name":"confluence_search","arguments":{"query":"payroll","limit":5}},"id":4}' 2>/dev/null)
SEARCH_JSON=$(echo "$SEARCH" | grep "^data:" | sed 's/^data: //' || echo "$SEARCH")
if echo "$SEARCH_JSON" | python3 -c "import sys, json; data = json.load(sys.stdin); assert 'result' in data and 'content' in data['result']" 2>/dev/null; then
    echo "✓ MCP confluence_search works"
else
    echo "✗ MCP confluence_search failed"
    echo "Response: $SEARCH"
    exit 1
fi

# Test 5: MCP confluence_list_space
echo "Test 5: MCP confluence_list_space"
LIST_SPACE=$(curl -s -X POST "http://localhost:8081/mcp/session/$SESSION_ID" -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","method":"tools/call","params":{"tool_name":"confluence_list_space","arguments":{"space_key":"NCHB","limit":10}},"id":5}' 2>/dev/null)
LIST_SPACE_JSON=$(echo "$LIST_SPACE" | grep "^data:" | sed 's/^data: //' || echo "$LIST_SPACE")
if echo "$LIST_SPACE_JSON" | python3 -c "import sys, json; data = json.load(sys.stdin); assert 'result' in data and 'content' in data['result']" 2>/dev/null; then
    echo "✓ MCP confluence_list_space works"
else
    echo "✗ MCP confluence_list_space failed"
    echo "Response: $LIST_SPACE"
    exit 1
fi

# Test 6: MCP confluence_get_page
echo "Test 6: MCP confluence_get_page"
GET_PAGE=$(curl -s -X POST "http://localhost:8081/mcp/session/$SESSION_ID" -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","method":"tools/call","params":{"tool_name":"confluence_get_page","arguments":{"space_key":"NCHB","page_id":250347937}},"id":6}' 2>/dev/null)
GET_PAGE_JSON=$(echo "$GET_PAGE" | grep "^data:" | sed 's/^data: //' || echo "$GET_PAGE")
if echo "$GET_PAGE_JSON" | python3 -c "import sys, json; data = json.load(sys.stdin); assert 'result' in data and 'content' in data['result']" 2>/dev/null; then
    echo "✓ MCP confluence_get_page works"
else
    echo "✗ MCP confluence_get_page failed"
    echo "Response: $GET_PAGE"
    exit 1
fi

# Test 7: MCP error handling
echo "Test 7: MCP error handling"
ERROR_TEST=$(curl -s -X POST "http://localhost:8081/mcp/session/nonexistent-session" -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","method":"tools/list","params":{},"id":7}' 2>/dev/null)
if echo "$ERROR_TEST" | python3 -c "import sys, json; data = json.load(sys.stdin); assert 'error' in data" 2>/dev/null; then
    echo "✓ MCP error handling works"
else
    echo "✗ MCP error handling failed"
    echo "Response: $ERROR_TEST"
    exit 1
fi

echo ""
echo "============================================"
echo "Phase 5: All tests PASSED ✓"
echo "============================================"
echo ""
echo "Summary:"
echo "  - MCP Session Init: ✓"
echo "  - MCP tools/list: ✓ (4 tools)"
echo "  - MCP confluence_list_spaces: ✓"
echo "  - MCP confluence_search: ✓"
echo "  - MCP confluence_list_space: ✓"
echo "  - MCP confluence_get_page: ✓"
echo "  - MCP error handling: ✓"
echo ""
echo "Registered Tools:"
echo "  1. confluence_search - Search Confluence pages"
echo "  2. confluence_list_spaces - List all spaces"
echo "  3. confluence_list_space - List pages in a space"
echo "  4. confluence_get_page - Get a specific page"
echo ""
