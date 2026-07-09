#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

VERSION="${1:-dev}"
OUT_DIR="${OUT_DIR:-$ROOT/dist}"
LDFLAGS="-s -w -X github.com/vkh/spacemosquito/internal/cliapp.Version=${VERSION}"

mkdir -p "$OUT_DIR"

targets=(
  "darwin arm64"
  "darwin amd64"
  "linux amd64"
  "windows amd64"
)

for entry in "${targets[@]}"; do
  read -r goos goarch <<<"$entry"
  name="spacemosquito-${goos}-${goarch}"
  if [ "$goos" = "windows" ]; then
    name="${name}.exe"
  fi
  echo "Building $name (version $VERSION)"
  GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 \
    go build -ldflags "$LDFLAGS" -o "$OUT_DIR/$name" ./cmd/spacemosquito
done

(
  cd "$OUT_DIR"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum spacemosquito-* > SHA256SUMS
  else
    shasum -a 256 spacemosquito-* > SHA256SUMS
  fi
)

echo "Artifacts in $OUT_DIR"
ls -la "$OUT_DIR"
