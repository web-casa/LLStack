#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
VERSION="${1:-${LLSTACK_VERSION:-}}"
REF_NAME="${LLSTACK_GIT_REF_NAME:-${GITHUB_REF_NAME:-}}"
REQUIRE_V_PREFIX="${LLSTACK_REQUIRE_V_PREFIX:-0}"
EXPECT_TAG_MATCH="${LLSTACK_EXPECT_TAG_MATCH:-0}"
REQUIRE_EXISTING_TAG="${LLSTACK_REQUIRE_EXISTING_TAG:-0}"

if [ -z "$VERSION" ]; then
  echo "release version is required" >&2
  exit 1
fi

semver_re='^v?[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$'
if ! [[ "$VERSION" =~ $semver_re ]]; then
  echo "release version must look like semver: $VERSION" >&2
  exit 1
fi

if [ "$REQUIRE_V_PREFIX" = "1" ] && [[ ! "$VERSION" =~ ^v ]]; then
  echo "release version must use a v-prefixed tag: $VERSION" >&2
  exit 1
fi

if [ "$EXPECT_TAG_MATCH" = "1" ]; then
  if [ -z "$REF_NAME" ]; then
    echo "tag/version guard requires LLSTACK_GIT_REF_NAME or GITHUB_REF_NAME" >&2
    exit 1
  fi
  normalized_ref="${REF_NAME#refs/tags/}"
  if [ "$normalized_ref" != "$VERSION" ]; then
    echo "release version does not match current tag: version=$VERSION ref=$normalized_ref" >&2
    exit 1
  fi
fi

if [ "$REQUIRE_EXISTING_TAG" = "1" ]; then
  if ! git -C "$ROOT_DIR" rev-parse -q --verify "refs/tags/$VERSION" >/dev/null 2>&1; then
    echo "release tag does not exist in git refs: $VERSION" >&2
    exit 1
  fi
fi

echo "release version validated: $VERSION"
