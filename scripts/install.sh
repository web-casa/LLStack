#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: scripts/install.sh --from <binary-or-tarball-or-url> [--prefix /usr/local] [--name llstack] [--sha256 <hex>] [--pubkey /path/to/public.pem] [--require-signature]
EOF
}

SOURCE_PATH=""
PREFIX="/usr/local"
NAME="llstack"
SKIP_BACKUP="false"
EXPECTED_SHA256=""
VERIFY_PUBKEY=""
REQUIRE_SIGNATURE=""
SKIP_SIGNATURE="false"

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
    --skip-signature)
      SKIP_SIGNATURE="true"
      shift
      ;;
    --skip-backup)
      SKIP_BACKUP="true"
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

# Default: require signatures when a public key is provided, unless explicitly skipped
if [ -z "$REQUIRE_SIGNATURE" ]; then
  if [ -n "$VERIFY_PUBKEY" ] && [ "$SKIP_SIGNATURE" != "true" ]; then
    REQUIRE_SIGNATURE="true"
  else
    REQUIRE_SIGNATURE="false"
  fi
fi

BIN_DIR="$PREFIX/bin"
DEST="$BIN_DIR/$NAME"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

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

download_source() {
  local src="$1"
  local target="${2:-}"
  local base="${src##*/}"
  base="${base%%\?*}"
  if [ -z "$base" ] || [ "$base" = "$src" ]; then
    base="llstack-download"
  fi
  if [ -z "$target" ]; then
    target="$TMP_DIR/$base"
  fi

  if command -v curl >/dev/null 2>&1; then
    curl -fsSL -o "$target" "$src"
    printf '%s\n' "$target"
    return
  fi
  if command -v wget >/dev/null 2>&1; then
    wget -qO "$target" "$src"
    printf '%s\n' "$target"
    return
  fi

  echo "remote source requires curl or wget: $src" >&2
  exit 1
}

sha256_file() {
  local path="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$path" | awk '{print $1}'
    return
  fi
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$path" | awk '{print $1}'
    return
  fi

  echo "sha256 verification requires sha256sum or shasum" >&2
  exit 1
}

verify_sha256() {
  local path="$1"
  local expected="$2"
  [ -n "$expected" ] || return 0

  local actual
  actual="$(sha256_file "$path")"
  if [ "$actual" != "$expected" ]; then
    echo "sha256 mismatch for $path" >&2
    echo "expected: $expected" >&2
    echo "actual:   $actual" >&2
    exit 1
  fi
}

signature_source_for() {
  local src="$1"
  case "$src" in
    http://*|https://*|file://*)
      printf '%s.sig\n' "$src"
      ;;
    *)
      printf '%s.sig\n' "$src"
      ;;
  esac
}

resolve_signature() {
  local src="$1"
  local sig_src
  sig_src="$(signature_source_for "$src")"

  if is_remote_source "$sig_src"; then
    local target="$TMP_DIR/$(basename "${sig_src%%\?*}")"
    if download_source "$sig_src" "$target" >/dev/null 2>&1; then
      printf '%s\n' "$target"
      return
    fi
    return
  fi

  if [ -f "$sig_src" ]; then
    printf '%s\n' "$sig_src"
  fi
}

verify_signature() {
  local path="$1"
  local pubkey="$2"
  local require="$3"

  [ -n "$pubkey" ] || return 0
  if [ ! -f "$pubkey" ]; then
    echo "verification public key not found: $pubkey" >&2
    exit 1
  fi
  if ! command -v openssl >/dev/null 2>&1; then
    echo "openssl is required for signature verification" >&2
    exit 1
  fi

  local sig_path=""
  sig_path="$(resolve_signature "$SOURCE_PATH" || true)"
  if [ -z "$sig_path" ]; then
    if [ "$require" = "true" ]; then
      echo "signature file not found for $SOURCE_PATH" >&2
      exit 1
    fi
    echo "signature verification skipped; no detached signature found for $SOURCE_PATH"
    return
  fi

  if ! openssl dgst -sha256 -verify "$pubkey" -signature "$sig_path" "$path" >/dev/null 2>&1; then
    echo "signature verification failed for $path" >&2
    exit 1
  fi
}

resolve_binary() {
  local src="$1"
  if [ -d "$src" ]; then
    find "$src" -type f \( -name "$NAME" -o -name "llstack" \) | head -n 1
    return
  fi
  case "$src" in
    *.tar.gz|*.tgz)
      tar -C "$TMP_DIR" -xzf "$src"
      find "$TMP_DIR" -type f \( -path "*/bin/$NAME" -o -name "$NAME" -o -name "llstack" \) | head -n 1
      ;;
    *)
      printf '%s\n' "$src"
      ;;
  esac
}

if [ -z "$SOURCE_PATH" ]; then
  echo "--from is required" >&2
  usage >&2
  exit 1
fi

RESOLVED_SOURCE="$SOURCE_PATH"
if is_remote_source "$SOURCE_PATH"; then
  RESOLVED_SOURCE="$(download_source "$SOURCE_PATH")"
elif [ ! -e "$SOURCE_PATH" ]; then
  echo "source path does not exist: $SOURCE_PATH" >&2
  exit 1
fi

if [ -f "$RESOLVED_SOURCE" ]; then
  verify_sha256 "$RESOLVED_SOURCE" "$EXPECTED_SHA256"
  verify_signature "$RESOLVED_SOURCE" "$VERIFY_PUBKEY" "$REQUIRE_SIGNATURE"
fi

SOURCE_BIN="$(resolve_binary "$RESOLVED_SOURCE")"
if [ -z "$SOURCE_BIN" ] || [ ! -f "$SOURCE_BIN" ]; then
  echo "could not resolve installable binary from $SOURCE_PATH" >&2
  exit 1
fi

mkdir -p "$BIN_DIR"
if [ -f "$DEST" ] && [ "$SKIP_BACKUP" != "true" ]; then
  backup="$DEST.bak.$(date +%Y%m%d%H%M%S)"
  cp "$DEST" "$backup"
  echo "backed up existing binary to $backup"
fi

install -m 0755 "$SOURCE_BIN" "$DEST"
echo "installed $DEST"
