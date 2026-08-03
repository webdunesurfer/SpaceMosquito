#!/usr/bin/env bash
# Build Firefox + Chrome extension zips (dist/ contents at archive root).
# Usage: build-extension-zips.sh <version> [out_dir]
#   version: tag or semver (e.g. v0.1.0 or 0.1.0). Zip names keep the given
#   string; manifest.json gets the leading "v" stripped.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VERSION="${1:?usage: build-extension-zips.sh <version> [out_dir]}"
OUT_DIR="${2:-$REPO_ROOT/dist}"
MANIFEST_VERSION="${VERSION#v}"

mkdir -p "$OUT_DIR"

stamp_manifest() {
  local path="$1"
  local ver="$2"
  node -e '
    const fs = require("fs");
    const path = process.argv[1];
    const ver = process.argv[2];
    const m = JSON.parse(fs.readFileSync(path, "utf8"));
    m.version = ver;
    fs.writeFileSync(path, JSON.stringify(m, null, 2) + "\n");
  ' "$path" "$ver"
}

build_one() {
  local name="$1"   # firefox | chrome
  local dir="$REPO_ROOT/${name}-extension"
  local zip_path="$OUT_DIR/spacemosquito-${name}-${VERSION}.zip"
  local mf="$dir/manifest.json"
  local bak="$mf.bak.$$"

  echo "Building ${name}-extension (manifest version $MANIFEST_VERSION)"
  cp "$mf" "$bak"
  # Always restore the working-tree manifest (version is stamped only for the build).
  restore_manifest() { mv "$bak" "$mf" 2>/dev/null || true; }
  trap restore_manifest EXIT
  stamp_manifest "$mf" "$MANIFEST_VERSION"
  (
    cd "$dir"
    npm ci
    npx webpack --mode production
  )
  trap - EXIT
  restore_manifest

  rm -f "$zip_path"
  ( cd "$dir/dist" && zip -r "$zip_path" . )
  echo "Wrote $zip_path"
}

build_one firefox
build_one chrome
