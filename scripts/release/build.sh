#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
VERSION="${LLSTACK_VERSION:-0.1.0-dev}"
COMMIT="${LLSTACK_COMMIT:-$(git -C "$ROOT_DIR" rev-parse --short HEAD 2>/dev/null || echo unknown)}"
BUILD_DATE="${LLSTACK_BUILD_DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"
DIST_DIR="${LLSTACK_DIST_DIR:-$ROOT_DIR/dist/releases/$VERSION}"
PLATFORMS="${LLSTACK_PLATFORMS:-linux/amd64 linux/arm64}"

mkdir -p "$DIST_DIR"

for target in $PLATFORMS; do
  os="${target%/*}"
  arch="${target#*/}"
  out_dir="$DIST_DIR/$os-$arch"
  out_bin="$out_dir/llstack"

  mkdir -p "$out_dir"
  echo "building $target -> $out_bin"

  (
    cd "$ROOT_DIR"
    CGO_ENABLED=0 \
    GOOS="$os" \
    GOARCH="$arch" \
    go build \
      -trimpath \
      -ldflags "-s -w -X main.version=$VERSION -X main.commit=$COMMIT -X main.buildDate=$BUILD_DATE -X main.targetOS=$os -X main.targetArch=$arch" \
      -o "$out_bin" \
      ./cmd/llstack
  )
done

cat >"$DIST_DIR/metadata.json" <<EOF
{
  "version": "$VERSION",
  "commit": "$COMMIT",
  "build_date": "$BUILD_DATE",
  "platforms": [
$(first=1; for target in $PLATFORMS; do
  if [ $first -eq 0 ]; then
    printf ",\n"
  fi
  printf "    \"%s\"" "$target"
  first=0
done)
  ]
}
EOF

echo "release artifacts written to $DIST_DIR"
