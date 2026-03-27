#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: scripts/upgrade.sh --from <binary-or-tarball-or-url> [--prefix /usr/local] [--name llstack] [--sha256 <hex>] [--pubkey /path/to/public.pem] [--require-signature]
EOF
}

SOURCE_PATH=""
PREFIX="/usr/local"
NAME="llstack"
EXPECTED_SHA256=""
VERIFY_PUBKEY=""
REQUIRE_SIGNATURE="false"

while [ $# -gt 0 ]; do
  case "$1" in
    --from)
      SOURCE_PATH="${2:-}"
      shift 2
      ;;
    --prefix)
      PREFIX="${2:-}"
      shift 2
      ;;
    --name)
      NAME="${2:-}"
      shift 2
      ;;
    --sha256)
      EXPECTED_SHA256="${2:-}"
      shift 2
      ;;
    --pubkey)
      VERIFY_PUBKEY="${2:-}"
      shift 2
      ;;
    --require-signature)
      REQUIRE_SIGNATURE="true"
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [ -z "$SOURCE_PATH" ]; then
  echo "--from is required" >&2
  usage >&2
  exit 1
fi

DEST="$PREFIX/bin/$NAME"
if [ ! -f "$DEST" ]; then
  echo "existing installation not found at $DEST; falling back to fresh install"
fi

args=(
  --from "$SOURCE_PATH"
  --prefix "$PREFIX"
  --name "$NAME"
)
if [ -n "$EXPECTED_SHA256" ]; then
  args+=(--sha256 "$EXPECTED_SHA256")
fi
if [ -n "$VERIFY_PUBKEY" ]; then
  args+=(--pubkey "$VERIFY_PUBKEY")
fi
if [ "$REQUIRE_SIGNATURE" = "true" ]; then
  args+=(--require-signature)
fi

"$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/install.sh" "${args[@]}"
echo "upgrade completed for $DEST"
