#!/usr/bin/env bash
# cleanup-docker-legacy.sh — wipe leftover SpaceMosquito Docker Compose state.
#
# Usage:
#   ./scripts/cleanup-docker-legacy.sh --dry-run
#   ./scripts/cleanup-docker-legacy.sh
#   ./scripts/cleanup-docker-legacy.sh --project-name SpaceMosquito
#   ./scripts/cleanup-docker-legacy.sh --purge-images
#   ./scripts/cleanup-docker-legacy.sh --purge-bind-mounts   # destructive; opt-in
#
# See DOCS/guides/cleanup-docker-legacy.md
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${REPO_ROOT}"

DRY_RUN=0
PURGE_BIND_MOUNTS=0
PURGE_IMAGES=0
PROJECT_NAME=""

usage() {
  cat <<'EOF'
Usage: ./scripts/cleanup-docker-legacy.sh [options]

  --dry-run              Print actions without changing anything
  --project-name NAME    Compose project name (default: repo directory basename)
  --purge-images         Remove spacemosquito-related Docker images
  --purge-bind-mounts    Also delete host bind mounts (saved-data, session.enc, …)
  -h, --help             Show this help

Guide: DOCS/guides/cleanup-docker-legacy.md
EOF
  exit "${1:-0}"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run) DRY_RUN=1; shift ;;
    --purge-bind-mounts) PURGE_BIND_MOUNTS=1; shift ;;
    --purge-images) PURGE_IMAGES=1; shift ;;
    --project-name)
      if [[ $# -lt 2 ]]; then
        echo "error: --project-name requires a value" >&2
        exit 1
      fi
      PROJECT_NAME="$2"
      shift 2
      ;;
    -h|--help) usage 0 ;;
    *)
      echo "error: unknown argument: $1" >&2
      usage 1
      ;;
  esac
done

run() {
  if [[ "${DRY_RUN}" -eq 1 ]]; then
    echo "[dry-run] $*"
  else
    echo "+ $*"
    "$@"
  fi
}

info() { echo "==> $*"; }
warn() { echo "warning: $*" >&2; }

# Default Compose project name is the directory basename (Compose v2).
if [[ -z "${PROJECT_NAME}" ]]; then
  PROJECT_NAME="$(basename "${REPO_ROOT}")"
fi
PROJECT_NAME_LOWER="$(printf '%s' "${PROJECT_NAME}" | tr '[:upper:]' '[:lower:]')"

COMPOSE_FILE="${REPO_ROOT}/docker-compose.yml"

print_bind_mounts() {
  info "Bind-mount / host leftovers:"
  found=0
  for p in \
    "${REPO_ROOT}/saved-data" \
    "${REPO_ROOT}/saved" \
    "${REPO_ROOT}/session.enc" \
    "${REPO_ROOT}/config.yaml" \
    "${REPO_ROOT}/cron-config.json" \
    "${REPO_ROOT}/.env"
  do
    if [[ -e "$p" ]]; then
      found=1
      echo "  - $p"
    fi
  done
  if [[ "$found" -eq 0 ]]; then
    echo "  (none found under repo root)"
  fi
}

print_next_steps() {
  cat <<'EOF'

Next steps (dockerless):
  1. Build or install spacemosquito
     cd space-mosquito && go build -o spacemosquito ./cmd/spacemosquito
  2. spacemosquito init
  3. Copy saved-data/ (or saved/) into ~/.spacemosquito/saved if migrating
  4. spacemosquito bootstrap import-saved
  5. spacemosquito reindex --content
  6. spacemosquito serve

Guide: DOCS/guides/cleanup-docker-legacy.md
EOF
}

if ! command -v docker >/dev/null 2>&1; then
  warn "docker not found on PATH — skipping container/volume/image cleanup"
  print_bind_mounts
  print_next_steps
  exit 0
fi

if ! docker info >/dev/null 2>&1; then
  warn "docker daemon not reachable — skipping container/volume/image cleanup"
  warn "(start Docker Desktop or Colima, then re-run)"
  print_bind_mounts
  print_next_steps
  exit 0
fi

info "Repo: ${REPO_ROOT}"
info "Compose project name: ${PROJECT_NAME}"
if [[ "${DRY_RUN}" -eq 1 ]]; then
  info "Mode: dry-run (no changes)"
fi

# --- Compose down -----------------------------------------------------------
if [[ -f "${COMPOSE_FILE}" ]]; then
  info "Found docker-compose.yml — bringing stack down"
  if [[ "${DRY_RUN}" -eq 1 ]]; then
    echo "[dry-run] docker compose -f ${COMPOSE_FILE} -p ${PROJECT_NAME} down --remove-orphans"
  else
    docker compose -f "${COMPOSE_FILE}" -p "${PROJECT_NAME}" down --remove-orphans \
      || warn "compose down failed (stack may already be gone)"
  fi
else
  info "No docker-compose.yml (expected after packaging removal) — trying project name only"
  if [[ "${DRY_RUN}" -eq 1 ]]; then
    echo "[dry-run] docker compose -p ${PROJECT_NAME} down --remove-orphans"
  else
    docker compose -p "${PROJECT_NAME}" down --remove-orphans 2>/dev/null \
      || warn "compose down by project name failed (ok if never used Compose here)"
  fi
fi

# --- Named volumes ----------------------------------------------------------
info "Looking for named volumes (pgdata)"

remove_volume_if_exists() {
  vol="$1"
  if docker volume inspect "$vol" >/dev/null 2>&1; then
    info "Removing volume: $vol"
    run docker volume rm "$vol" || warn "failed to remove volume $vol"
    return 0
  fi
  echo "  (not found) $vol"
  return 1
}

remove_volume_if_exists "${PROJECT_NAME}_pgdata" || true
if [[ "${PROJECT_NAME_LOWER}" != "${PROJECT_NAME}" ]]; then
  remove_volume_if_exists "${PROJECT_NAME_LOWER}_pgdata" || true
fi
remove_volume_if_exists "spacemosquito_pgdata" || true
remove_volume_if_exists "SpaceMosquito_pgdata" || true

# Catch leftovers whose names contain both spacemosquito and pgdata (case-insensitive).
while IFS= read -r vol; do
  [[ -z "$vol" ]] && continue
  case "$vol" in
    "${PROJECT_NAME}_pgdata"|"${PROJECT_NAME_LOWER}_pgdata"|spacemosquito_pgdata|SpaceMosquito_pgdata)
      continue
      ;;
  esac
  info "Removing matching volume: $vol"
  run docker volume rm "$vol" || warn "failed to remove volume $vol"
done < <(
  docker volume ls -q 2>/dev/null | while IFS= read -r v; do
    vl=$(printf '%s' "$v" | tr '[:upper:]' '[:lower:]')
    case "$vl" in
      *spacemosquito*pgdata*|*pgdata*spacemosquito*) printf '%s\n' "$v" ;;
    esac
  done
)

# --- Images (optional) ------------------------------------------------------
if [[ "${PURGE_IMAGES}" -eq 1 ]]; then
  info "Removing images matching spacemosquito / ${PROJECT_NAME}"
  while IFS= read -r img; do
    [[ -z "$img" ]] && continue
    info "Removing image: $img"
    run docker image rm "$img" || warn "failed to remove image $img"
  done < <(
    docker images --format '{{.Repository}}:{{.Tag}}' 2>/dev/null | while IFS= read -r line; do
      ll=$(printf '%s' "$line" | tr '[:upper:]' '[:lower:]')
      pl=$(printf '%s' "${PROJECT_NAME}" | tr '[:upper:]' '[:lower:]')
      case "$ll" in
        *spacemosquito*|"${pl}-app"*|"${pl}_app"*) printf '%s\n' "$line" ;;
      esac
    done
  )
else
  info "Skipping image removal (pass --purge-images to remove related images)"
fi

# --- Bind mounts ------------------------------------------------------------
print_bind_mounts
if [[ "${PURGE_BIND_MOUNTS}" -eq 1 ]]; then
  for p in \
    "${REPO_ROOT}/saved-data" \
    "${REPO_ROOT}/saved" \
    "${REPO_ROOT}/session.enc" \
    "${REPO_ROOT}/config.yaml" \
    "${REPO_ROOT}/cron-config.json" \
    "${REPO_ROOT}/.env"
  do
    if [[ -e "$p" ]]; then
      info "Purging bind mount: $p"
      run rm -rf "$p"
    fi
  done
else
  echo
  echo "  Host paths above were NOT deleted."
  echo "  To remove them: re-run with --purge-bind-mounts"
  echo "  Prefer migrating crawl data:"
  echo "    mkdir -p ~/.spacemosquito/saved && cp -R saved-data/* ~/.spacemosquito/saved/"
fi

print_next_steps
info "Done."
