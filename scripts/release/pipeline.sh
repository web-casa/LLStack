#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
MODE="${1:-validate}"
VERSION="${LLSTACK_VERSION:-0.1.0-dev}"
DIST_DIR="${LLSTACK_DIST_DIR:-$ROOT_DIR/dist/releases/$VERSION}"
PACKAGE_DIR="${LLSTACK_PACKAGE_DIR:-$ROOT_DIR/dist/packages/$VERSION}"
PLATFORMS="${LLSTACK_PLATFORMS:-linux/amd64 linux/arm64}"
RUN_TESTS="${LLSTACK_RUN_TESTS:-1}"
RUN_BUILD="${LLSTACK_RUN_BUILD:-1}"
RUN_SIGN="${LLSTACK_RUN_SIGN:-0}"
RUN_VERIFY="${LLSTACK_RUN_VERIFY:-1}"
RUN_NOTES="${LLSTACK_RUN_NOTES:-1}"
RUN_PUBLISH="${LLSTACK_RUN_PUBLISH:-0}"
PUBLISH_PROVIDER="${LLSTACK_PUBLISH_PROVIDER:-}"
REQUIRE_V_PREFIX="${LLSTACK_REQUIRE_V_PREFIX:-0}"
EXPECT_TAG_MATCH="${LLSTACK_EXPECT_TAG_MATCH:-0}"
REQUIRE_EXISTING_TAG="${LLSTACK_REQUIRE_EXISTING_TAG:-0}"

run_make() {
  local target="$1"
  shift
  make -C "$ROOT_DIR" "$target" VERSION="$VERSION" DIST_DIR="$DIST_DIR" PACKAGE_DIR="$PACKAGE_DIR" PLATFORMS="$PLATFORMS" "$@"
}

validate_version() {
  LLSTACK_VERSION="$VERSION" \
  LLSTACK_GIT_REF_NAME="${LLSTACK_GIT_REF_NAME:-${GITHUB_REF_NAME:-}}" \
  LLSTACK_REQUIRE_V_PREFIX="$REQUIRE_V_PREFIX" \
  LLSTACK_EXPECT_TAG_MATCH="$EXPECT_TAG_MATCH" \
  LLSTACK_REQUIRE_EXISTING_TAG="$REQUIRE_EXISTING_TAG" \
  bash "$ROOT_DIR/scripts/release/validate-version.sh"
}

render_notes() {
  LLSTACK_VERSION="$VERSION" \
  LLSTACK_PACKAGE_DIR="$PACKAGE_DIR" \
  LLSTACK_DIST_DIR="$DIST_DIR" \
  LLSTACK_GITHUB_REPOSITORY="${LLSTACK_GITHUB_REPOSITORY:-${GITHUB_REPOSITORY:-}}" \
  bash "$ROOT_DIR/scripts/release/render-notes.sh"
}

case "$MODE" in
  validate|release)
    validate_version
    if [ "$RUN_TESTS" = "1" ]; then
      (cd "$ROOT_DIR" && go test ./...)
    fi
    if [ "$RUN_BUILD" = "1" ]; then
      (cd "$ROOT_DIR" && go build ./...)
    fi
    run_make build-cross
    run_make package
    if [ "$RUN_SIGN" = "1" ]; then
      test -n "${LLSTACK_SIGNING_KEY:-}" || { echo "LLSTACK_SIGNING_KEY is required when LLSTACK_RUN_SIGN=1" >&2; exit 1; }
      test -n "${LLSTACK_SIGNING_PUBKEY:-}" || { echo "LLSTACK_SIGNING_PUBKEY is required when LLSTACK_RUN_SIGN=1" >&2; exit 1; }
      LLSTACK_VERSION="$VERSION" \
      LLSTACK_PACKAGE_DIR="$PACKAGE_DIR" \
      LLSTACK_SIGNING_KEY="$LLSTACK_SIGNING_KEY" \
      LLSTACK_SIGNING_PUBKEY="$LLSTACK_SIGNING_PUBKEY" \
      bash "$ROOT_DIR/scripts/release/sign.sh"
    fi
    if [ "$RUN_VERIFY" = "1" ]; then
      LLSTACK_VERSION="$VERSION" \
      LLSTACK_PACKAGE_DIR="$PACKAGE_DIR" \
      LLSTACK_VERIFY_PUBKEY="${LLSTACK_VERIFY_PUBKEY:-}" \
      LLSTACK_REQUIRE_SIGNATURES="${LLSTACK_REQUIRE_SIGNATURES:-0}" \
      bash "$ROOT_DIR/scripts/release/verify.sh"
    fi
    if [ "$RUN_NOTES" = "1" ]; then
      render_notes
    fi
    if [ "$RUN_PUBLISH" = "1" ]; then
      test -n "$PUBLISH_PROVIDER" || { echo "LLSTACK_PUBLISH_PROVIDER is required when LLSTACK_RUN_PUBLISH=1" >&2; exit 1; }
      LLSTACK_VERSION="$VERSION" \
      LLSTACK_PACKAGE_DIR="$PACKAGE_DIR" \
      LLSTACK_DIST_DIR="$DIST_DIR" \
      LLSTACK_PUBLISH_PROVIDER="$PUBLISH_PROVIDER" \
      LLSTACK_PUBLISH_TARGET="${LLSTACK_PUBLISH_TARGET:-}" \
      LLSTACK_GITHUB_REPOSITORY="${LLSTACK_GITHUB_REPOSITORY:-${GITHUB_REPOSITORY:-}}" \
      bash "$ROOT_DIR/scripts/release/publish.sh"
    fi
    ;;
  *)
    echo "unsupported release pipeline mode: $MODE" >&2
    exit 1
    ;;
esac

echo "release pipeline completed: mode=$MODE version=$VERSION"
