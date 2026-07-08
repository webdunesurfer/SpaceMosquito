#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${REPO_ROOT}"

echo "Checking Colima..."
if ! colima status 2>/dev/null | rg -qi "running"; then
  echo "Colima is not running. Starting Colima..."
  colima start
else
  echo "Colima is already running."
fi

echo "Rebuilding SpaceMosquito app image..."
docker compose build app

echo "Redeploying SpaceMosquito app container..."
docker compose up -d app

echo "Restarting SpaceMosquito app service..."
docker compose restart app

echo "SpaceMosquito rebuild/redeploy/restart completed."
