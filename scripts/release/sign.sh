#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
VERSION="${LLSTACK_VERSION:-0.1.0-dev}"
PACKAGE_DIR="${LLSTACK_PACKAGE_DIR:-$ROOT_DIR/dist/packages/$VERSION}"
SIGNING_KEY="${LLSTACK_SIGNING_KEY:-}"
SIGNING_PUBKEY="${LLSTACK_SIGNING_PUBKEY:-}"
SIGNATURES_FILE="$PACKAGE_DIR/signatures.json"

if [ ! -d "$PACKAGE_DIR" ]; then
  echo "package directory not found: $PACKAGE_DIR" >&2
  exit 1
fi
if [ -z "$SIGNING_KEY" ]; then
  echo "LLSTACK_SIGNING_KEY is required" >&2
  exit 1
fi
if [ ! -f "$SIGNING_KEY" ]; then
  echo "signing key not found: $SIGNING_KEY" >&2
  exit 1
fi
if ! command -v openssl >/dev/null 2>&1; then
  echo "openssl is required for release signing" >&2
  exit 1
fi

files=(
  "$PACKAGE_DIR/checksums.txt"
  "$PACKAGE_DIR/index.json"
  "$PACKAGE_DIR/sbom.spdx.json"
  "$PACKAGE_DIR/provenance.json"
)

for archive in "$PACKAGE_DIR"/llstack-"$VERSION"-*.tar.gz; do
  [ -f "$archive" ] || continue
  files+=("$archive")
done

entries=()
created="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

for file in "${files[@]}"; do
  if [ ! -f "$file" ]; then
    echo "expected release file not found for signing: $file" >&2
    exit 1
  fi
  sig_file="$file.sig"
  openssl dgst -sha256 -sign "$SIGNING_KEY" -out "$sig_file" "$file"
  sig_checksum="$(
    cd "$PACKAGE_DIR"
    sha256sum "$(basename "$sig_file")" | awk '{print $1}'
  )"
  entries+=("    {\"file\":\"$(basename "$file")\",\"signature\":\"$(basename "$sig_file")\",\"algorithm\":\"openssl-rsa-sha256\",\"signature_sha256\":\"$sig_checksum\"}")
done

{
  printf "{\n"
  printf "  \"version\": \"%s\",\n" "$VERSION"
  printf "  \"created_at\": \"%s\",\n" "$created"
  printf "  \"scheme\": \"openssl-rsa-sha256\",\n"
  if [ -n "$SIGNING_PUBKEY" ]; then
    printf "  \"public_key_hint\": \"%s\",\n" "$(basename "$SIGNING_PUBKEY")"
  else
    printf "  \"public_key_hint\": \"\",\n"
  fi
  printf "  \"signatures\": [\n"
  for i in "${!entries[@]}"; do
    printf "%s" "${entries[$i]}"
    if [ "$i" -lt "$((${#entries[@]} - 1))" ]; then
      printf ","
    fi
    printf "\n"
  done
  printf "  ]\n"
  printf "}\n"
} >"$SIGNATURES_FILE"

echo "release signatures written to $PACKAGE_DIR"
