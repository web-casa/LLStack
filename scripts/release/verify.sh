#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
VERSION="${LLSTACK_VERSION:-0.1.0-dev}"
PACKAGE_DIR="${LLSTACK_PACKAGE_DIR:-$ROOT_DIR/dist/packages/$VERSION}"

CHECKSUM_FILE="$PACKAGE_DIR/checksums.txt"
INDEX_FILE="$PACKAGE_DIR/index.json"
SBOM_FILE="$PACKAGE_DIR/sbom.spdx.json"
PROVENANCE_FILE="$PACKAGE_DIR/provenance.json"
SIGNATURES_FILE="$PACKAGE_DIR/signatures.json"
VERIFY_PUBKEY="${LLSTACK_VERIFY_PUBKEY:-}"
REQUIRE_SIGNATURES="${LLSTACK_REQUIRE_SIGNATURES:-}"
if [ -z "$REQUIRE_SIGNATURES" ]; then
  if [ -n "$VERIFY_PUBKEY" ]; then
    REQUIRE_SIGNATURES="1"
  else
    REQUIRE_SIGNATURES="0"
  fi
fi

if [ ! -d "$PACKAGE_DIR" ]; then
  echo "package directory not found: $PACKAGE_DIR" >&2
  exit 1
fi
if [ ! -f "$CHECKSUM_FILE" ]; then
  echo "checksum file not found: $CHECKSUM_FILE" >&2
  exit 1
fi
for file in "$INDEX_FILE" "$SBOM_FILE" "$PROVENANCE_FILE"; do
  if [ ! -f "$file" ]; then
    echo "release metadata file not found: $file" >&2
    exit 1
  fi
done

verify_release_metadata() {
  while read -r checksum file; do
    [ -n "${checksum:-}" ] || continue
    [ -n "${file:-}" ] || continue
    archive="${file#./}"
    archive="${archive#* }"
    if [ ! -f "$PACKAGE_DIR/$archive" ]; then
      echo "archive referenced by checksums is missing: $archive" >&2
      exit 1
    fi
    if ! grep -F "\"archive\":\"$archive\"" "$INDEX_FILE" | grep -F "\"sha256\":\"$checksum\"" >/dev/null 2>&1; then
      echo "index.json does not match checksum entry for $archive" >&2
      exit 1
    fi
    if ! grep -F "$archive" "$SBOM_FILE" >/dev/null 2>&1 || ! grep -F "$checksum" "$SBOM_FILE" >/dev/null 2>&1; then
      echo "sbom.spdx.json does not describe $archive with checksum $checksum" >&2
      exit 1
    fi
    if ! grep -F "$archive" "$PROVENANCE_FILE" >/dev/null 2>&1 || ! grep -F "$checksum" "$PROVENANCE_FILE" >/dev/null 2>&1; then
      echo "provenance.json does not describe $archive with checksum $checksum" >&2
      exit 1
    fi
  done <"$CHECKSUM_FILE"
}

verify_release_signatures() {
  if [ ! -f "$SIGNATURES_FILE" ]; then
    if [ "$REQUIRE_SIGNATURES" = "1" ]; then
      echo "signatures are required but signatures.json is missing" >&2
      exit 1
    fi
    return
  fi

  if [ -z "$VERIFY_PUBKEY" ]; then
    if [ "$REQUIRE_SIGNATURES" = "1" ]; then
      echo "signatures are required but LLSTACK_VERIFY_PUBKEY is not set" >&2
      exit 1
    fi
    echo "signatures.json present; set LLSTACK_VERIFY_PUBKEY to verify detached signatures"
    return
  fi

  if [ ! -f "$VERIFY_PUBKEY" ]; then
    echo "verification public key not found: $VERIFY_PUBKEY" >&2
    exit 1
  fi
  if ! command -v openssl >/dev/null 2>&1; then
    echo "openssl is required for signature verification" >&2
    exit 1
  fi

  signed_files=(
    "$CHECKSUM_FILE"
    "$INDEX_FILE"
    "$SBOM_FILE"
    "$PROVENANCE_FILE"
  )
  for archive in "$PACKAGE_DIR"/llstack-"$VERSION"-*.tar.gz; do
    [ -f "$archive" ] || continue
    signed_files+=("$archive")
  done

  for file in "${signed_files[@]}"; do
    base="$(basename "$file")"
    sig_file="$file.sig"
    if [ ! -f "$sig_file" ]; then
      echo "signature file missing for $base: $(basename "$sig_file")" >&2
      exit 1
    fi
    if ! grep -F "\"file\":\"$base\"" "$SIGNATURES_FILE" | grep -F "\"signature\":\"$(basename "$sig_file")\"" >/dev/null 2>&1; then
      echo "signatures.json does not describe detached signature for $base" >&2
      exit 1
    fi
    if ! openssl dgst -sha256 -verify "$VERIFY_PUBKEY" -signature "$sig_file" "$file" >/dev/null 2>&1; then
      echo "detached signature verification failed for $base" >&2
      exit 1
    fi
  done

  echo "release signatures verified"
}

(
  cd "$PACKAGE_DIR"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum -c checksums.txt
  elif command -v shasum >/dev/null 2>&1; then
    while read -r checksum file; do
      actual="$(shasum -a 256 "$file" | awk '{print $1}')"
      if [ "$actual" != "$checksum" ]; then
        echo "$file: FAILED" >&2
        exit 1
      fi
      echo "$file: OK"
    done <checksums.txt
  else
    echo "checksum verification requires sha256sum or shasum" >&2
    exit 1
  fi
)

verify_release_metadata
verify_release_signatures
echo "release metadata verified"
