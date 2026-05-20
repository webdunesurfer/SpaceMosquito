#!/bin/bash
set -e

echo "============================================"
echo "Phase 4: BM25/FTS Search - Test Suite"
echo "============================================"
echo ""

# Test 1: Check FTS column exists
echo "Test 1: Checking FTS content_vector column..."
FTS_CHECK=$(docker-compose exec db psql -U spacemosquito -d spacemosquito -t -c \
    "SELECT column_name FROM information_schema.columns WHERE table_name = 'pages' AND column_name = 'content_vector'" 2>/dev/null)

if [[ "$FTS_CHECK" == *"content_vector"* ]]; then
    echo "✓ FTS content_vector column exists"
else
    echo "✗ FTS content_vector column NOT found"
    exit 1
fi

# Test 2: Check FTS index exists
echo "Test 2: Checking FTS GIN index..."
INDEX_CHECK=$(docker-compose exec db psql -U spacemosquito -d spacemosquito -t -c \
    "SELECT indexname FROM pg_indexes WHERE tablename = 'pages' AND indexname = 'idx_pages_content_vector'" 2>/dev/null)

if [[ "$INDEX_CHECK" == *"idx_pages_content_vector"* ]]; then
    echo "✓ FTS GIN index exists"
else
    echo "✗ FTS GIN index NOT found"
    exit 1
fi

# Test 3: Check pages are indexed
echo "Test 3: Checking indexed pages..."
INDEXED_COUNT=$(docker-compose exec db psql -U spacemosquito -d spacemosquito -t -c \
    "SELECT COUNT(*) FROM pages WHERE content_vector IS NOT NULL" 2>/dev/null | tr -d ' ')

echo "  Indexed pages: $INDEXED_COUNT"
if [ "$INDEXED_COUNT" -gt 0 ] 2>/dev/null; then
    echo "✓ Pages are indexed"
else
    echo "✗ No pages are indexed"
    exit 1
fi

# Test 4: Test search API
echo "Test 4: Testing search API..."
sleep 5
SEARCH_RESPONSE=$(curl -s "http://localhost:8081/api/search?q=payroll&limit=5" 2>/dev/null || echo '{"error":"server not ready"}')
if echo "$SEARCH_RESPONSE" | python3 -c "import sys, json; data = json.load(sys.stdin); assert 'results' in data and len(data['results']) > 0" 2>/dev/null; then
    echo "✓ Search API works"
else
    echo "✗ Search API failed"
    echo "Response: $SEARCH_RESPONSE"
    exit 1
fi

# Test 5: Test stats API
echo "Test 5: Testing stats API..."
STATS_RESPONSE=$(curl -s "http://localhost:8081/api/search/stats" 2>/dev/null || echo '{"error":"server not ready"}')
if echo "$STATS_RESPONSE" | python3 -c "import sys, json; data = json.load(sys.stdin); assert 'TotalPages' in data and data['TotalPages'] > 0" 2>/dev/null; then
    echo "✓ Stats API works"
else
    echo "✗ Stats API failed"
    echo "Response: $STATS_RESPONSE"
    exit 1
fi

# Test 6: Test reindex API
echo "Test 6: Testing reindex API..."
REINDEX_RESPONSE=$(curl -s -X POST "http://localhost:8081/api/search/reindex" 2>/dev/null || echo '{"error":"server not ready"}')
if echo "$REINDEX_RESPONSE" | python3 -c "import sys, json; data = json.load(sys.stdin); assert 'message' in data" 2>/dev/null; then
    echo "✓ Reindex API works"
else
    echo "✗ Reindex API failed"
    echo "Response: $REINDEX_RESPONSE"
    exit 1
fi

# Test 7: Test CLI search command
echo "Test 7: Testing CLI search command..."
CLI_SEARCH=$(docker-compose exec app /app/cli search "payroll" 2>&1 | grep -A 5 "Search Results" || echo "Failed")
if echo "$CLI_SEARCH" | grep -q "Payroll"; then
    echo "✓ CLI search command works"
else
    echo "✗ CLI search command failed"
    echo "Output: $CLI_SEARCH"
    exit 1
fi

# Test 8: Test CLI stats command
echo "Test 8: Testing CLI stats command..."
CLI_STATS=$(docker-compose exec app /app/cli stats 2>&1 | grep "Total Pages:" || echo "Failed")
if echo "$CLI_STATS" | grep -q "Total Pages:"; then
    echo "✓ CLI stats command works"
else
    echo "✗ CLI stats command failed"
    echo "Output: $CLI_STATS"
    exit 1
fi

echo ""
echo "============================================"
echo "Phase 4: All tests PASSED ✓"
echo "============================================"
echo ""
echo "Summary:"
echo "  - FTS content_vector column: ✓"
echo "  - FTS GIN index: ✓"
echo "  - Pages indexed: $INDEXED_COUNT"
echo "  - Search API: ✓"
echo "  - Stats API: ✓"
echo "  - Reindex API: ✓"
echo "  - CLI search: ✓"
echo "  - CLI stats: ✓"
echo ""
