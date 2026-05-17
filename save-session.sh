#!/bin/bash
# Helper script to save session cookies to the encrypted session file
# Usage: save-session.sh <cookie_name> <cookie_value> [cookie_name] [cookie_value] ...

set -e

COOKIES="$@"

# Build JSON cookies array
JSON="["
FIRST=true
while [ $# -gt 0 ]; do
  name="$1"
  value="$2"
  shift 2
  
  if [ "$FIRST" = true ]; then
    FIRST=false
  else
    JSON+=","
  fi
  
  # Escape value for JSON
  escaped_value=$(echo "$value" | sed 's/"/\\"/g')
  
  JSON+="{\"name\":\"$name\",\"value\":\"$escaped_value\",\"domain\":\".atlassian.net\",\"path\":\"/\",\"secure\":true,\"httpOnly\":true}"
done
JSON+="]"

echo "Sending session to API..."
curl -s -X POST http://localhost:8080/api/session \
  -H "Content-Type: application/json" \
  -d "{\"confluence_url\":\"https://teamnetconomy.atlassian.net/wiki/spaces/NCHB\",\"cookies\":$JSON}" | python3 -m json.tool
