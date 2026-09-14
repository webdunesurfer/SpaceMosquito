#!/usr/bin/env bash
# Rasterize extension icons from SVG → PNG (Chrome requires PNG for toolbar/manifest).
# Usage: ./scripts/render-extension-icons.sh
# Prefer librsvg (`rsvg-convert`); ImageMagick's built-in SVG renderer corrupts stroke art.
#
# Outputs per extension assets/:
#   icon.svg              — brand red (popup header)
#   icon-active-{N}.png   — red toolbar (Confluence tab)
#   icon-inactive.svg     — gray
#   icon-inactive-{N}.png — gray toolbar / manifest default
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
RED_SRC="$ROOT/dev/tasks/extension-themes/icons/06-side-flying.svg"
GRAY_SRC="$ROOT/dev/tasks/extension-themes/icons/06-side-flying-inactive.svg"
if [[ ! -f "$RED_SRC" ]]; then
  RED_SRC="$ROOT/firefox-extension/assets/icon.svg"
fi
if [[ ! -f "$GRAY_SRC" && -f "$RED_SRC" ]]; then
  sed 's/#9b4d4d/#8a93a0/g' "$RED_SRC" > "$GRAY_SRC"
fi

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
    echo "need rsvg-convert (librsvg) or magick to render icons" >&2
    exit 1
  fi
}

for dir in firefox-extension chrome-extension; do
  dest="$ROOT/$dir/assets"
  mkdir -p "$dest"
  cp "$RED_SRC" "$dest/icon.svg"
  cp "$GRAY_SRC" "$dest/icon-inactive.svg"
  for s in 16 32 48 128; do
    render_size "$RED_SRC" "$s" "$dest/icon-active-${s}.png"
    render_size "$GRAY_SRC" "$s" "$dest/icon-inactive-${s}.png"
  done
  # Remove obsolete single-color icon-N.png if present from older builds
  rm -f "$dest"/icon-16.png "$dest"/icon-32.png "$dest"/icon-48.png "$dest"/icon-128.png
  echo "rendered $dir/assets/icon-{active,inactive}-{16,32,48,128}.png"
done
