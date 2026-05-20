#!/bin/bash
set -e

echo "============================================"
echo "Running Phase 4 and Phase 5 Tests"
echo "============================================"
echo ""

echo "Phase 4: BM25/FTS Search"
echo "========================="
cd /Users/vkh/dev/SpaceMosquito && ./tests/test_phase4_fts.sh 2>&1

echo ""
echo "Phase 5: MCP Server"
echo "==================="
cd /Users/vkh/dev/SpaceMosquito && ./tests/test_phase5_mcp.sh 2>&1

echo ""
echo "============================================"
echo "All Phases Passed ✓"
echo "============================================"
