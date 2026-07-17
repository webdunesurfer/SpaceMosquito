#!/usr/bin/env bash
# cleanup-docker-legacy.sh — wipe leftover SpaceMosquito Docker Compose state.
#
# Usage:
#   ./scripts/cleanup-docker-legacy.sh --dry-run
#   ./scripts/cleanup-docker-legacy.sh
#   ./scripts/cleanup-docker-legacy.sh --project-name SpaceMosquito
#   ./scripts/cleanup-docker-legacy.sh --purge-images
#   ./scripts/cleanup-docker-legacy.sh --purge-bind-mounts
#
# See docs/guides/cleanup-docker-legacy.md
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${REPO_ROOT}"

DRY_RUN=0
PURGE_BIND_MOUNTS=0
PURGE_IMAGES=0
PROJECT_NAME=""
VOLUMES_TO_REMOVE=""

usage() {
  cat <<'EOF'
Usage: ./scripts/cleanup-docker-legacy.sh [options]

  --dry-run              Print actions without changing anything
  --project-name NAME    Compose project name (default: repo directory basename)
  --purge-images         Remove spacemosquito-related Docker images
  --purge-bind-mounts    Also delete host bind mounts (saved-data, session.enc, ...)
  -h, --help             Show this help

Guide: docs/guides/cleanup-docker-legacy.md
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

# Compose project names must be lowercase [a-z0-9_-], starting with alnum.
# Use separate sed -E calls for macOS/BSD sed compatibility.
normalize_project_name() {
  printf '%s' "$1" \
    | tr '[:upper:]' '[:lower:]' \
    | sed -E 's/[^a-z0-9_-]+/-/g' \
    | sed -E 's/^-+//' \
    | sed -E 's/-+$//' \
    | sed -E 's/^[^a-z0-9]+//'
}

if [[ -z "${PROJECT_NAME}" ]]; then
  PROJECT_NAME="$(basename "${REPO_ROOT}")"
fi
RAW_PROJECT_NAME="${PROJECT_NAME}"
PROJECT_NAME="$(normalize_project_name "${PROJECT_NAME}")"
if [[ -z "${PROJECT_NAME}" ]]; then
  PROJECT_NAME="spacemosquito"
fi

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

Guide: docs/guides/cleanup-docker-legacy.md
EOF
}

volume_already_queued() {
  needle="$1"
  for v in ${VOLUMES_TO_REMOVE}; do
    [[ "$v" == "$needle" ]] && return 0
  done
  return 1
}

queue_volume() {
  vol="$1"
  if volume_already_queued "$vol"; then
    return 0
  fi
  if [[ -z "${VOLUMES_TO_REMOVE}" ]]; then
    VOLUMES_TO_REMOVE="$vol"
  else
    VOLUMES_TO_REMOVE="${VOLUMES_TO_REMOVE} ${vol}"
  fi
}

# Remove containers attached to a volume, then remove the volume.
remove_volume() {
  vol="$1"
  if ! docker volume inspect "$vol" >/dev/null 2>&1; then
    echo "  (not found) $vol"
    return 0
  fi

  info "Freeing containers using volume: $vol"
  ids="$(docker ps -aq --filter "volume=${vol}" 2>/dev/null || true)"
  if [[ -n "${ids}" ]]; then
    if [[ "${DRY_RUN}" -eq 1 ]]; then
      echo "[dry-run] docker rm -f ${ids}"
    else
      echo "+ docker rm -f (containers using ${vol})"
      # shellcheck disable=SC2086
      docker rm -f ${ids} || warn "failed to remove some containers using ${vol}"
    fi
  else
    echo "  (no containers via --filter volume=)"
  fi

  info "Removing volume: $vol"
  if [[ "${DRY_RUN}" -eq 1 ]]; then
    echo "[dry-run] docker volume rm ${vol}"
    return 0
  fi

  echo "+ docker volume rm ${vol}"
  if docker volume rm "$vol" 2>/dev/null; then
    return 0
  fi

  warn "volume still in use — scanning all containers for mount ${vol}"
  docker ps -aq 2>/dev/null | while IFS= read -r cid; do
    [[ -z "$cid" ]] && continue
    mounts="$(docker inspect "$cid" --format '{{range .Mounts}}{{.Name}} {{end}}' 2>/dev/null || true)"
    case " ${mounts} " in
      *" ${vol} "*)
        echo "+ docker rm -f ${cid}"
        docker rm -f "$cid" || true
        ;;
    esac
  done

  if docker volume rm "$vol"; then
    return 0
  fi
  warn "still failed to remove volume ${vol}"
  return 1
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

if [[ "${RAW_PROJECT_NAME}" != "${PROJECT_NAME}" ]]; then
  info "Normalized Compose project name: ${RAW_PROJECT_NAME} -> ${PROJECT_NAME}"
fi
info "Repo: ${REPO_ROOT}"
info "Compose project name: ${PROJECT_NAME}"
if [[ "${DRY_RUN}" -eq 1 ]]; then
  info "Mode: dry-run (no changes)"
fi

# --- Compose down ------------------------------------------------------------
if [[ -f "${COMPOSE_FILE}" ]]; then
  info "Found docker-compose.yml — bringing stack down"
  if [[ "${DRY_RUN}" -eq 1 ]]; then
    echo "[dry-run] docker compose -f ${COMPOSE_FILE} -p ${PROJECT_NAME} down --remove-orphans"
  else
    docker compose -f "${COMPOSE_FILE}" -p "${PROJECT_NAME}" down --remove-orphans \
      || warn "compose down failed (stack may already be gone)"
  fi
else
  info "No docker-compose.yml — trying project name only"
  if [[ "${DRY_RUN}" -eq 1 ]]; then
    echo "[dry-run] docker compose -p ${PROJECT_NAME} down --remove-orphans"
  else
    docker compose -p "${PROJECT_NAME}" down --remove-orphans 2>/dev/null \
      || warn "compose down by project name failed (ok if never used Compose here)"
  fi
fi

# Stop leftover containers for this project / name.
info "Stopping leftover containers matching spacemosquito / ${PROJECT_NAME}"
LEFTOVER_IDS="$(docker ps -aq --filter "label=com.docker.compose.project=${PROJECT_NAME}" 2>/dev/null || true)"
if [[ -z "${LEFTOVER_IDS}" ]]; then
  LEFTOVER_IDS="$(docker ps -a --format '{{.ID}} {{.Names}}' 2>/dev/null \
    | awk 'tolower($0) ~ /spacemosquito/ {print $1}' || true)"
fi
if [[ -n "${LEFTOVER_IDS}" ]]; then
  if [[ "${DRY_RUN}" -eq 1 ]]; then
    echo "[dry-run] docker rm -f ${LEFTOVER_IDS}"
  else
    echo "+ docker rm -f (leftover project containers)"
    # shellcheck disable=SC2086
    docker rm -f ${LEFTOVER_IDS} || warn "failed to remove some leftover containers"
  fi
else
  echo "  (none)"
fi

# --- Named volumes -----------------------------------------------------------
info "Looking for named volumes (pgdata)"
queue_volume "${PROJECT_NAME}_pgdata"
queue_volume "spacemosquito_pgdata"

while IFS= read -r vol; do
  [[ -z "$vol" ]] && continue
  vol_lc=$(printf '%s' "$vol" | tr '[:upper:]' '[:lower:]')
  case "$vol_lc" in
    *spacemosquito*)
      case "$vol_lc" in
        *pgdata*) queue_volume "$vol" ;;
      esac
      ;;
  esac
done <<EOF
$(docker volume ls -q 2>/dev/null || true)
EOF

if [[ -z "${VOLUMES_TO_REMOVE}" ]]; then
  echo "  (no matching volumes found)"
else
  for vol in ${VOLUMES_TO_REMOVE}; do
    remove_volume "$vol" || true
  done
fi

# --- Images (optional) -------------------------------------------------------
if [[ "${PURGE_IMAGES}" -eq 1 ]]; then
  info "Removing images matching spacemosquito / ${PROJECT_NAME}"
  while IFS= read -r img; do
    [[ -z "$img" ]] && continue
    [[ "$img" == "<none>:<none>" ]] && continue
    info "Removing image: $img"
    run docker image rm "$img" || warn "failed to remove image $img"
  done <<EOF
$(docker images --format '{{.Repository}}:{{.Tag}}' 2>/dev/null \
  | awk -v p="${PROJECT_NAME}" '
      BEGIN { pl = tolower(p) }
      {
        ll = tolower($0)
        if (ll ~ /spacemosquito/ || index(ll, pl "-app") || index(ll, pl "_app"))
          print
      }' \
  || true)
EOF
else
  info "Skipping image removal (pass --purge-images to remove related images)"
fi

# --- Bind mounts -------------------------------------------------------------
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
