#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
VERSION="${LLSTACK_VERSION:-}"
BASE_URL="${LLSTACK_REMOTE_BASE_URL:-}"
INDEX_URL="${LLSTACK_REMOTE_INDEX_URL:-}"
VERIFY_PUBKEY="${LLSTACK_VERIFY_PUBKEY:-}"
REQUIRE_SIGNATURES="${LLSTACK_REQUIRE_SIGNATURES:-0}"
OUTPUT_JSON="${LLSTACK_REMOTE_VERIFY_JSON_OUT:-}"
OUTPUT_MD="${LLSTACK_REMOTE_VERIFY_MD_OUT:-}"

if [ -z "$VERSION" ]; then
  echo "LLSTACK_VERSION is required" >&2
  exit 1
fi
if [ -z "$INDEX_URL" ] && [ -z "$BASE_URL" ]; then
  echo "set LLSTACK_REMOTE_INDEX_URL or LLSTACK_REMOTE_BASE_URL" >&2
  exit 1
fi

fetch() {
  local src="$1"
  local dest="$2"
  if [[ "$src" == file://* ]]; then
    cp "${src#file://}" "$dest"
    return
  fi
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$src" -o "$dest"
    return
  fi
  if command -v wget >/dev/null 2>&1; then
    wget -qO "$dest" "$src"
    return
  fi
  echo "curl or wget is required for remote verification" >&2
  exit 1
}

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT
PKG_DIR="$TMP_DIR/package"
mkdir -p "$PKG_DIR"

if [ -n "$INDEX_URL" ]; then
  fetch "$INDEX_URL" "$PKG_DIR/index.json"
  case "$INDEX_URL" in
    */index.json)
      BASE_URL="${INDEX_URL%/index.json}"
      ;;
    *)
      echo "LLSTACK_REMOTE_INDEX_URL must end with /index.json" >&2
      exit 1
      ;;
  esac
else
  fetch "${BASE_URL%/}/index.json" "$PKG_DIR/index.json"
fi

for meta in checksums.txt sbom.spdx.json provenance.json; do
  fetch "${BASE_URL%/}/$meta" "$PKG_DIR/$meta"
done

if fetch "${BASE_URL%/}/signatures.json" "$PKG_DIR/signatures.json" 2>/dev/null; then
  :
else
  rm -f "$PKG_DIR/signatures.json"
fi

archives=()
while read -r line; do
  archive="$(printf '%s' "$line" | sed -n 's/.*"archive":"\([^"]*\)".*/\1/p')"
  [ -n "$archive" ] || continue
  archives+=("$archive")
done <"$PKG_DIR/index.json"

if [ "${#archives[@]}" -eq 0 ]; then
  echo "no archives found in remote index" >&2
  exit 1
fi

signature_count=0
for archive in "${archives[@]}"; do
  fetch "${BASE_URL%/}/$archive" "$PKG_DIR/$archive"
done
for signed in checksums.txt index.json sbom.spdx.json provenance.json "${archives[@]}"; do
  if [ -f "$PKG_DIR/signatures.json" ]; then
    if fetch "${BASE_URL%/}/$signed.sig" "$PKG_DIR/$signed.sig" 2>/dev/null; then
      signature_count=$((signature_count + 1))
    fi
  fi
done

LLSTACK_VERSION="$VERSION" \
LLSTACK_PACKAGE_DIR="$PKG_DIR" \
LLSTACK_VERIFY_PUBKEY="$VERIFY_PUBKEY" \
LLSTACK_REQUIRE_SIGNATURES="$REQUIRE_SIGNATURES" \
bash "$ROOT_DIR/scripts/release/verify.sh"

status="passed"
generated_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
if [ -z "$OUTPUT_JSON" ]; then
  OUTPUT_JSON="$PKG_DIR/remote-verify.json"
fi
if [ -z "$OUTPUT_MD" ]; then
  OUTPUT_MD="$PKG_DIR/remote-verify.md"
fi

json_assets=""
first=1
for archive in "${archives[@]}"; do
  if [ $first -eq 0 ]; then
    json_assets="${json_assets},"
  fi
  json_assets="${json_assets}\"$archive\""
  first=0
done

mkdir -p "$(dirname "$OUTPUT_JSON")" "$(dirname "$OUTPUT_MD")"
cat >"$OUTPUT_JSON" <<EOF
{
  "version": "$VERSION",
  "base_url": "${BASE_URL}",
  "status": "$status",
  "generated_at": "$generated_at",
  "signature_count": $signature_count,
  "archives": [$json_assets]
}
EOF

cat >"$OUTPUT_MD" <<EOF
# Remote Release Verification

- Version: \`$VERSION\`
- Base URL: \`${BASE_URL}\`
- Status: \`$status\`
- Archive count: \`${#archives[@]}\`
- Detached signatures fetched: \`$signature_count\`
EOF

echo "remote release verification completed: $BASE_URL"
