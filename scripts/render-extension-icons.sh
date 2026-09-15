#!/usr/bin/env bash
# Rasterize extension icons from SVG → PNG (Chrome requires PNG for toolbar/manifest).
# Usage: ./scripts/render-extension-icons.sh
# Prefer librsvg (`rsvg-convert`); ImageMagick's built-in SVG renderer corrupts stroke art.
#
# PNGs are committed in-repo. Regenerating requires rsvg-convert (or magick as fallback).
# If neither tool is present and PNGs already exist, this script exits 0 (build can proceed).
#
# Outputs per extension assets/:
#   icon.svg              — brand green (popup header)
#   icon-active-{N}.png   — green toolbar (Confluence tab)
#   icon-inactive.svg     — gray
#   icon-inactive-{N}.png — gray toolbar / manifest default
#   icon-crawl-f{0-5}-{16,32}.png — crawl animation frames
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
ACTIVE_SRC="$ROOT/dev/tasks/extension-themes/icons/06-side-flying.svg"
GRAY_SRC="$ROOT/dev/tasks/extension-themes/icons/06-side-flying-inactive.svg"
CRAWL_SRC_DIR="$ROOT/dev/tasks/extension-crawl-icon-animation/icons"
if [[ ! -f "$ACTIVE_SRC" ]]; then
  ACTIVE_SRC="$ROOT/firefox-extension/assets/icon.svg"
fi
if [[ ! -f "$GRAY_SRC" && -f "$ACTIVE_SRC" ]]; then
  sed 's/#4a7a5c/#8a93a0/g' "$ACTIVE_SRC" > "$GRAY_SRC"
fi

SIZES=(16 32 48 128)
CRAWL_SIZES=(16 32)

have_pngs() {
  local dest="$1"
  for s in "${SIZES[@]}"; do
    [[ -f "$dest/icon-active-${s}.png" ]] || return 1
    [[ -f "$dest/icon-inactive-${s}.png" ]] || return 1
  done
  for i in 0 1 2 3 4 5; do
    for s in "${CRAWL_SIZES[@]}"; do
      [[ -f "$dest/icon-crawl-f${i}-${s}.png" ]] || return 1
    done
  done
  return 0
}

render_size() {
  local src="$1" size="$2" out="$3"
  if command -v rsvg-convert >/dev/null 2>&1; then
    rsvg-convert -w "$size" -h "$size" -b none "$src" -o "$out"
  elif command -v magick >/dev/null 2>&1; then
    echo "warning: rsvg-convert not found; ImageMagick MSVG often crops icons badly" >&2
    magick -background none "$src" \
      -trim +repage \
      -resize "${size}x${size}" \
      -gravity center -background none -extent "${size}x${size}" \
      -depth 8 "PNG32:$out"
  else
    return 2
  fi
}

if ! command -v rsvg-convert >/dev/null 2>&1 && ! command -v magick >/dev/null 2>&1; then
  missing=0
  for dir in firefox-extension chrome-extension; do
    if ! have_pngs "$ROOT/$dir/assets"; then
      missing=1
    fi
  done
  if [[ "$missing" -eq 0 ]]; then
    echo "skip icon render: no rsvg-convert/magick; using committed PNGs" >&2
    echo "  (install librsvg to regenerate: brew install librsvg)" >&2
    exit 0
  fi
  echo "need rsvg-convert (librsvg) or magick to render icons (committed PNGs missing)" >&2
  exit 1
fi

for dir in firefox-extension chrome-extension; do
  dest="$ROOT/$dir/assets"
  mkdir -p "$dest"
  cp "$ACTIVE_SRC" "$dest/icon.svg"
  cp "$GRAY_SRC" "$dest/icon-inactive.svg"
  for s in "${SIZES[@]}"; do
    render_size "$ACTIVE_SRC" "$s" "$dest/icon-active-${s}.png"
    render_size "$GRAY_SRC" "$s" "$dest/icon-inactive-${s}.png"
  done
  for i in 0 1 2 3 4 5; do
    src="$CRAWL_SRC_DIR/crawl-f${i}.svg"
    if [[ -f "$src" ]]; then
      for s in "${CRAWL_SIZES[@]}"; do
        render_size "$src" "$s" "$dest/icon-crawl-f${i}-${s}.png"
      done
    fi
  done
  # Remove obsolete single-color icon-N.png if present from older builds
  rm -f "$dest"/icon-16.png "$dest"/icon-32.png "$dest"/icon-48.png "$dest"/icon-128.png
  echo "rendered $dir/assets/icon-{active,inactive}-{16,32,48,128}.png + crawl frames"
done
