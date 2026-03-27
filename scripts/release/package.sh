#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
VERSION="${LLSTACK_VERSION:-0.1.0-dev}"
DIST_DIR="${LLSTACK_DIST_DIR:-$ROOT_DIR/dist/releases/$VERSION}"
PACKAGE_DIR="${LLSTACK_PACKAGE_DIR:-$ROOT_DIR/dist/packages/$VERSION}"
STAGE_DIR="$PACKAGE_DIR/stage"

# Collect build context for provenance (escape double quotes for JSON safety)
json_safe() { printf '%s' "$1" | sed 's/"/\\"/g'; }
GIT_COMMIT="$(json_safe "${LLSTACK_GIT_COMMIT:-$(git -C "$ROOT_DIR" rev-parse HEAD 2>/dev/null || echo "unknown")}")"
GIT_REF="$(json_safe "${LLSTACK_GIT_REF:-$(git -C "$ROOT_DIR" describe --tags --always 2>/dev/null || echo "unknown")}")"
GIT_REPO="$(json_safe "${LLSTACK_GIT_REPO:-$(git -C "$ROOT_DIR" remote get-url origin 2>/dev/null || echo "unknown")}")"
GO_VERSION="$(json_safe "${LLSTACK_GO_VERSION:-$(go version 2>/dev/null | awk '{print $3}' || echo "unknown")}")"
BUILD_OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
BUILD_ARCH="$(uname -m)"
BUILD_HOST="$(json_safe "$(hostname 2>/dev/null || echo "unknown")")"

if [ ! -d "$DIST_DIR" ]; then
  echo "release build directory not found: $DIST_DIR" >&2
  exit 1
fi

rm -rf "$STAGE_DIR"
mkdir -p "$PACKAGE_DIR" "$STAGE_DIR"
: >"$PACKAGE_DIR/checksums.txt"
entries=()
spdx_packages=()
spdx_relationships=()
created="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

for platform_dir in "$DIST_DIR"/*-*; do
  [ -d "$platform_dir" ] || continue
  platform="$(basename "$platform_dir")"
  bundle_name="llstack-$VERSION-$platform"
  bundle_root="$STAGE_DIR/$bundle_name"
  archive_path="$PACKAGE_DIR/$bundle_name.tar.gz"

  mkdir -p "$bundle_root/bin" "$bundle_root/scripts" "$bundle_root/docs"
  cp "$platform_dir/llstack" "$bundle_root/bin/llstack"
  cp "$ROOT_DIR/scripts/install.sh" "$bundle_root/scripts/install.sh"
  cp "$ROOT_DIR/scripts/upgrade.sh" "$bundle_root/scripts/upgrade.sh"
  cp "$ROOT_DIR/dev_docs/COMPATIBILITY.md" "$bundle_root/docs/COMPATIBILITY.md"
  cp "$ROOT_DIR/dev_docs/KNOWN_LIMITATIONS.md" "$bundle_root/docs/KNOWN_LIMITATIONS.md"
  if [ -f "$DIST_DIR/metadata.json" ]; then
    cp "$DIST_DIR/metadata.json" "$bundle_root/metadata.json"
  fi

  tar -C "$STAGE_DIR" -czf "$archive_path" "$bundle_name"
  checksum="$(
    cd "$PACKAGE_DIR"
    sha256sum "$(basename "$archive_path")" | awk '{print $1}'
  )"
  (
    cd "$PACKAGE_DIR"
    printf "%s  %s\n" "$checksum" "$(basename "$archive_path")" >>"checksums.txt"
  )
  entries+=("    {\"platform\":\"$platform\",\"archive\":\"$(basename "$archive_path")\",\"sha256\":\"$checksum\"}")
  package_id="SPDXRef-Package-$(printf '%s' "$platform" | tr '/.' '--')"
  spdx_packages+=("    {\"name\":\"$(basename "$archive_path")\",\"SPDXID\":\"$package_id\",\"versionInfo\":\"$VERSION\",\"downloadLocation\":\"NOASSERTION\",\"filesAnalyzed\":false,\"checksums\":[{\"algorithm\":\"SHA256\",\"checksumValue\":\"$checksum\"}]}")
  spdx_relationships+=("    {\"spdxElementId\":\"SPDXRef-DOCUMENT\",\"relationshipType\":\"DESCRIBES\",\"relatedSpdxElement\":\"$package_id\"}")
done

{
  printf "{\n"
  printf "  \"version\": \"%s\",\n" "$VERSION"
  printf "  \"packages\": [\n"
  for i in "${!entries[@]}"; do
    printf "%s" "${entries[$i]}"
    if [ "$i" -lt "$((${#entries[@]} - 1))" ]; then
      printf ","
    fi
    printf "\n"
  done
  printf "  ]\n"
  printf "}\n"
} >"$PACKAGE_DIR/index.json"

{
  printf "{\n"
  printf "  \"spdxVersion\": \"SPDX-2.3\",\n"
  printf "  \"dataLicense\": \"CC0-1.0\",\n"
  printf "  \"SPDXID\": \"SPDXRef-DOCUMENT\",\n"
  printf "  \"name\": \"llstack-release-%s\",\n" "$VERSION"
  printf "  \"documentNamespace\": \"https://llstack.dev/spdx/releases/%s\",\n" "$VERSION"
  printf "  \"creationInfo\": {\n"
  printf "    \"created\": \"%s\",\n" "$created"
  printf "    \"creators\": [\"Tool: LLStack scripts/release/package.sh\"]\n"
  printf "  },\n"
  printf "  \"packages\": [\n"
  for i in "${!spdx_packages[@]}"; do
    printf "%s" "${spdx_packages[$i]}"
    if [ "$i" -lt "$((${#spdx_packages[@]} - 1))" ]; then
      printf ","
    fi
    printf "\n"
  done
  printf "  ],\n"
  printf "  \"relationships\": [\n"
  for i in "${!spdx_relationships[@]}"; do
    printf "%s" "${spdx_relationships[$i]}"
    if [ "$i" -lt "$((${#spdx_relationships[@]} - 1))" ]; then
      printf ","
    fi
    printf "\n"
  done
  printf "  ]\n"
  printf "}\n"
} >"$PACKAGE_DIR/sbom.spdx.json"

cat >"$PACKAGE_DIR/provenance.json" <<EOF
{
  "version": "$VERSION",
  "created_at": "$created",
  "builder": {
    "tool": "LLStack scripts/release/package.sh",
    "go_version": "$GO_VERSION",
    "build_os": "$BUILD_OS",
    "build_arch": "$BUILD_ARCH",
    "build_host": "$BUILD_HOST"
  },
  "source": {
    "repository": "$GIT_REPO",
    "commit": "$GIT_COMMIT",
    "ref": "$GIT_REF"
  },
  "references": {
    "build_metadata": "metadata.json",
    "checksums": "checksums.txt",
    "index": "index.json",
    "sbom": "sbom.spdx.json"
  },
  "artifacts": [
$(first=1; for platform_dir in "$DIST_DIR"/*-*; do
  [ -d "$platform_dir" ] || continue
  platform="$(basename "$platform_dir")"
  archive_name="llstack-$VERSION-$platform.tar.gz"
  checksum="$(grep "  $archive_name$" "$PACKAGE_DIR/checksums.txt" | awk '{print $1}')"
  if [ $first -eq 0 ]; then
    printf ",\n"
  fi
  printf "    {\n"
  printf "      \"platform\": \"%s\",\n" "$platform"
  printf "      \"archive\": \"%s\",\n" "$archive_name"
  printf "      \"sha256\": \"%s\"\n" "$checksum"
  printf "    }"
  first=0
done)
  ]
}
EOF

echo "release packages written to $PACKAGE_DIR"
