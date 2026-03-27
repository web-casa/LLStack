#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
VERSION="${LLSTACK_VERSION:-}"
PACKAGE_DIR="${LLSTACK_PACKAGE_DIR:-$ROOT_DIR/dist/packages/$VERSION}"
DIST_DIR="${LLSTACK_DIST_DIR:-$ROOT_DIR/dist/releases/$VERSION}"
PROVIDER="${LLSTACK_PUBLISH_PROVIDER:-}"
PUBLISH_TARGET="${LLSTACK_PUBLISH_TARGET:-}"
RELEASE_NAME="${LLSTACK_RELEASE_NAME:-LLStack $VERSION}"
RELEASE_NOTES="${LLSTACK_RELEASE_NOTES:-$DIST_DIR/release-notes.md}"
GITHUB_REPOSITORY_NAME="${LLSTACK_GITHUB_REPOSITORY:-${GITHUB_REPOSITORY:-}}"
ASSETS_OUT="${LLSTACK_PUBLISH_ASSETS_OUT:-$PACKAGE_DIR/release-assets.txt}"
URL_OUT="${LLSTACK_PUBLISH_URL_OUT:-$PACKAGE_DIR/release-url.txt}"

usage() {
  cat <<'EOF'
Usage: scripts/release/publish.sh

Environment variables:
  LLSTACK_VERSION              Release version (required)
  LLSTACK_PUBLISH_PROVIDER     Provider: github | directory (required)
  LLSTACK_PACKAGE_DIR          Package directory containing release artifacts
  LLSTACK_PUBLISH_TARGET       Provider-specific target:
                                 github:    repository (e.g. owner/repo, defaults to GITHUB_REPOSITORY)
                                 directory: target directory path (required)
  LLSTACK_RELEASE_NAME         Release title (default: LLStack <version>)
  LLSTACK_RELEASE_NOTES        Path to release notes file
  LLSTACK_PUBLISH_ASSETS_OUT   Output file for published asset listing
  LLSTACK_PUBLISH_URL_OUT      Output file for release URL

Providers:
  github      Create a GitHub Release and upload assets using `gh` CLI
  directory   Copy release artifacts to a local/network directory

EOF
}

if [ -z "$VERSION" ]; then
  echo "LLSTACK_VERSION is required" >&2
  usage >&2
  exit 1
fi
if [ -z "$PROVIDER" ]; then
  echo "LLSTACK_PUBLISH_PROVIDER is required" >&2
  usage >&2
  exit 1
fi
if [ ! -d "$PACKAGE_DIR" ]; then
  echo "package directory not found: $PACKAGE_DIR" >&2
  exit 1
fi

collect_assets() {
  local dir="$1"
  for file in "$dir"/*; do
    [ -f "$file" ] || continue
    base="$(basename "$file")"
    case "$base" in
      release-summary.md|release-summary.json|release-assets.txt|release-url.txt|remote-verify.json|remote-verify.md|stage)
        continue
        ;;
    esac
    printf '%s\n' "$base"
  done
}

publish_github() {
  local repo="${PUBLISH_TARGET:-$GITHUB_REPOSITORY_NAME}"
  if [ -z "$repo" ]; then
    echo "LLSTACK_PUBLISH_TARGET or GITHUB_REPOSITORY is required for github provider" >&2
    exit 1
  fi
  if ! command -v gh >/dev/null 2>&1; then
    echo "gh CLI is required for github provider (https://cli.github.com)" >&2
    exit 1
  fi

  local notes_arg=()
  if [ -f "$RELEASE_NOTES" ]; then
    notes_arg=(--notes-file "$RELEASE_NOTES")
  else
    notes_arg=(--notes "Release $VERSION")
  fi

  local files=()
  while IFS= read -r asset; do
    files+=("$PACKAGE_DIR/$asset")
  done < <(collect_assets "$PACKAGE_DIR")

  if [ "${#files[@]}" -eq 0 ]; then
    echo "no assets to publish" >&2
    exit 1
  fi

  gh release create "$VERSION" \
    --repo "$repo" \
    --title "$RELEASE_NAME" \
    "${notes_arg[@]}" \
    "${files[@]}"

  local release_url
  release_url="$(gh release view "$VERSION" --repo "$repo" --json url --jq '.url')"

  mkdir -p "$(dirname "$URL_OUT")" "$(dirname "$ASSETS_OUT")"
  printf '%s\n' "$release_url" >"$URL_OUT"

  gh release view "$VERSION" --repo "$repo" --json assets --jq '.assets[].name' >"$ASSETS_OUT"

  echo "published to github: $release_url"
}

publish_directory() {
  local target_dir="${PUBLISH_TARGET:-}"
  if [ -z "$target_dir" ]; then
    echo "LLSTACK_PUBLISH_TARGET is required for directory provider" >&2
    exit 1
  fi

  mkdir -p "$target_dir"

  local count=0
  while IFS= read -r asset; do
    cp "$PACKAGE_DIR/$asset" "$target_dir/$asset"
    count=$((count + 1))
  done < <(collect_assets "$PACKAGE_DIR")

  if [ "$count" -eq 0 ]; then
    echo "no assets to publish" >&2
    exit 1
  fi

  mkdir -p "$(dirname "$URL_OUT")" "$(dirname "$ASSETS_OUT")"
  printf 'file://%s\n' "$target_dir" >"$URL_OUT"

  collect_assets "$target_dir" >"$ASSETS_OUT"

  echo "published $count assets to directory: $target_dir"
}

case "$PROVIDER" in
  github)
    publish_github
    ;;
  directory)
    publish_directory
    ;;
  *)
    echo "unsupported publish provider: $PROVIDER" >&2
    usage >&2
    exit 1
    ;;
esac
