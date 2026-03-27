#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: scripts/install-release.sh --index <index.json-or-url> [--platform linux-amd64] [--prefix /usr/local] [--name llstack] [--upgrade] [--pubkey /path/to/public.pem] [--require-signature]
EOF
}

INDEX_SOURCE=""
PLATFORM=""
PREFIX="/usr/local"
NAME="llstack"
UPGRADE="false"
VERIFY_PUBKEY=""
REQUIRE_SIGNATURE=""
SKIP_SIGNATURE="false"

while [ $# -gt 0 ]; do
  case "$1" in
    --index)
      INDEX_SOURCE="${2:-}"
      shift 2
      ;;
    --platform)
      PLATFORM="${2:-}"
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
    --upgrade)
      UPGRADE="true"
      shift
      ;;
    --pubkey)
      VERIFY_PUBKEY="${2:-}"
      shift 2
      ;;
    --require-signature)
      REQUIRE_SIGNATURE="true"
      shift
      ;;
    --skip-signature)
      SKIP_SIGNATURE="true"
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

if [ -z "$INDEX_SOURCE" ]; then
  echo "--index is required" >&2
  usage >&2
  exit 1
fi

# Default: require signatures when a public key is provided, unless explicitly skipped
if [ -z "$REQUIRE_SIGNATURE" ]; then
  if [ -n "$VERIFY_PUBKEY" ] && [ "$SKIP_SIGNATURE" != "true" ]; then
    REQUIRE_SIGNATURE="true"
  else
    REQUIRE_SIGNATURE="false"
  fi
fi

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

detect_platform() {
  local os arch
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64)
      arch="amd64"
      ;;
    aarch64|arm64)
      arch="arm64"
      ;;
  esac
  printf '%s-%s\n' "$os" "$arch"
}

is_remote_source() {
  case "$1" in
    http://*|https://*|file://*)
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

download_file() {
  local src="$1"
  local out="$2"

  if command -v curl >/dev/null 2>&1; then
    curl -fsSL -o "$out" "$src"
    return
  fi
  if command -v wget >/dev/null 2>&1; then
    wget -qO "$out" "$src"
    return
  fi

  echo "remote source requires curl or wget: $src" >&2
  exit 1
}

resolve_index() {
  local src="$1"
  local target="$TMP_DIR/index.json"
  if is_remote_source "$src"; then
    download_file "$src" "$target"
    printf '%s\n' "$target"
    return
  fi
  if [ ! -f "$src" ]; then
    echo "index source does not exist: $src" >&2
    exit 1
  fi
  printf '%s\n' "$src"
}

index_base() {
  local src="$1"
  case "$src" in
    http://*|https://*)
      printf '%s\n' "${src%/*}"
      ;;
    file://*)
      printf '%s\n' "${src%/*}"
      ;;
    *)
      dirname "$src"
      ;;
  esac
}

extract_value() {
  local line="$1" key="$2"
  printf '%s\n' "$line" | sed -E "s/.*\"$key\":\"([^\"]+)\".*/\\1/"
}

verify_index_signature() {
  local index_path="$1"
  [ -n "$VERIFY_PUBKEY" ] || return 0
  if [ ! -f "$VERIFY_PUBKEY" ]; then
    echo "verification public key not found: $VERIFY_PUBKEY" >&2
    exit 1
  fi
  if ! command -v openssl >/dev/null 2>&1; then
    echo "openssl is required for signature verification" >&2
    exit 1
  fi

  local sig_source sig_path
  sig_source="${INDEX_SOURCE}.sig"
  sig_path="$TMP_DIR/index.json.sig"
  if is_remote_source "$sig_source"; then
    if ! download_file "$sig_source" "$sig_path"; then
      if [ "$REQUIRE_SIGNATURE" = "true" ]; then
        echo "signature file not found for release index: $INDEX_SOURCE" >&2
        exit 1
      fi
      echo "index signature verification skipped; no detached signature found for $INDEX_SOURCE"
      return
    fi
  else
    if [ ! -f "$sig_source" ]; then
      if [ "$REQUIRE_SIGNATURE" = "true" ]; then
        echo "signature file not found for release index: $INDEX_SOURCE" >&2
        exit 1
      fi
      echo "index signature verification skipped; no detached signature found for $INDEX_SOURCE"
      return
    fi
    cp "$sig_source" "$sig_path"
  fi

  if ! openssl dgst -sha256 -verify "$VERIFY_PUBKEY" -signature "$sig_path" "$index_path" >/dev/null 2>&1; then
    echo "signature verification failed for release index: $INDEX_SOURCE" >&2
    exit 1
  fi
}

PLATFORM="${PLATFORM:-$(detect_platform)}"
INDEX_PATH="$(resolve_index "$INDEX_SOURCE")"
verify_index_signature "$INDEX_PATH"
ENTRY_LINE="$(grep -F "\"platform\":\"$PLATFORM\"" "$INDEX_PATH" | head -n 1 || true)"

if [ -z "$ENTRY_LINE" ]; then
  echo "platform not found in release index: $PLATFORM" >&2
  exit 1
fi

ARCHIVE_NAME="$(extract_value "$ENTRY_LINE" "archive")"
SHA256_VALUE="$(extract_value "$ENTRY_LINE" "sha256")"
if [ -z "$ARCHIVE_NAME" ] || [ -z "$SHA256_VALUE" ] || [ "$ARCHIVE_NAME" = "$ENTRY_LINE" ] || [ "$SHA256_VALUE" = "$ENTRY_LINE" ]; then
  echo "failed to parse archive entry for platform $PLATFORM" >&2
  exit 1
fi

BASE="$(index_base "$INDEX_SOURCE")"
case "$BASE" in
  http://*|https://*|file://*)
    ARCHIVE_SOURCE="$BASE/$ARCHIVE_NAME"
    ;;
  *)
    ARCHIVE_SOURCE="$BASE/$ARCHIVE_NAME"
    ;;
esac

SCRIPT="$ROOT_DIR/scripts/install.sh"
if [ "$UPGRADE" = "true" ]; then
  SCRIPT="$ROOT_DIR/scripts/upgrade.sh"
fi

args=(--from "$ARCHIVE_SOURCE" --prefix "$PREFIX" --name "$NAME" --sha256 "$SHA256_VALUE")
if [ -n "$VERIFY_PUBKEY" ]; then
  args+=(--pubkey "$VERIFY_PUBKEY")
fi
if [ "$REQUIRE_SIGNATURE" = "true" ]; then
  args+=(--require-signature)
fi

"$SCRIPT" "${args[@]}"
