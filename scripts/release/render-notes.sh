#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
VERSION="${LLSTACK_VERSION:-}"
PACKAGE_DIR="${LLSTACK_PACKAGE_DIR:-$ROOT_DIR/dist/packages/$VERSION}"
DIST_DIR="${LLSTACK_DIST_DIR:-$ROOT_DIR/dist/releases/$VERSION}"
TEMPLATE_FILE="${LLSTACK_RELEASE_TEMPLATE:-$ROOT_DIR/.github/release-notes.md}"
OUTPUT_FILE="${LLSTACK_RELEASE_NOTES_OUT:-$DIST_DIR/release-notes.md}"
GITHUB_REPOSITORY_NAME="${LLSTACK_GITHUB_REPOSITORY:-${GITHUB_REPOSITORY:-}}"
GENERATED_AT="${LLSTACK_RELEASE_NOTES_DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"

if [ -z "$VERSION" ]; then
  echo "LLSTACK_VERSION is required" >&2
  exit 1
fi
if [ ! -d "$PACKAGE_DIR" ]; then
  echo "package directory not found: $PACKAGE_DIR" >&2
  exit 1
fi
if [ ! -f "$TEMPLATE_FILE" ]; then
  echo "release notes template not found: $TEMPLATE_FILE" >&2
  exit 1
fi

artifact_rows=""
while read -r checksum file; do
  [ -n "${checksum:-}" ] || continue
  [ -n "${file:-}" ] || continue
  artifact_rows="${artifact_rows}| \`${file}\` | \`${checksum}\` |\n"
done <"$PACKAGE_DIR/checksums.txt"

if [ -z "$artifact_rows" ]; then
  echo "release notes require at least one checksum entry" >&2
  exit 1
fi

metadata_items=$'- `checksums.txt`\n- `index.json`\n- `sbom.spdx.json`\n- `provenance.json`'
signing_status="Detached signatures were not included in this release build."
verify_snippet=$'bash scripts/install-release.sh \\\n  --index <release-index-url> \\\n  --platform linux-amd64'

if [ -f "$PACKAGE_DIR/signatures.json" ]; then
  metadata_items="${metadata_items}"$'\n- `signatures.json` plus detached `.sig` files'
  signing_status="Detached OpenSSL signatures are included for archives and release metadata."
  verify_snippet=$'bash scripts/install-release.sh \\\n  --index <release-index-url> \\\n  --platform linux-amd64 \\\n  --pubkey /path/to/release-public.pem \\\n  --require-signature'
fi

release_index_url="<release-index-url>"
if [ -n "$GITHUB_REPOSITORY_NAME" ]; then
  release_index_url="https://github.com/${GITHUB_REPOSITORY_NAME}/releases/download/${VERSION}/index.json"
fi
verify_snippet="${verify_snippet//<release-index-url>/$release_index_url}"

template="$(cat "$TEMPLATE_FILE")"
template="${template//'{{VERSION}}'/$VERSION}"
template="${template//'{{GENERATED_AT}}'/$GENERATED_AT}"
template="${template//'{{ARTIFACT_TABLE_ROWS}}'/$artifact_rows}"
template="${template//'{{METADATA_ITEMS}}'/$metadata_items}"
template="${template//'{{SIGNING_STATUS}}'/$signing_status}"
template="${template//'{{VERIFY_SNIPPET}}'/$verify_snippet}"

mkdir -p "$(dirname "$OUTPUT_FILE")"
printf "%b\n" "$template" >"$OUTPUT_FILE"

echo "release notes written to $OUTPUT_FILE"
