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

echo "Restarting SpaceMosquito app service..."
if docker compose ps --status running --services | rg -qx "app"; then
  docker compose restart app
else
  docker compose up -d app
fi

echo "SpaceMosquito is ready."
