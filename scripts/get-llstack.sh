#!/usr/bin/env bash
#
# LLStack One-Line Installer
#
# Usage:
#   curl -sSL https://get.llstack.com | bash
#   curl -sSL https://get.llstack.com | bash -s -- --version v0.1.0
#
# Environment variables:
#   LLSTACK_VERSION    Override version (default: latest)
#   LLSTACK_PREFIX     Install prefix (default: /usr/local)
#   LLSTACK_NO_INSTALL Skip running `llstack install` after download
#
set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

info()  { printf "${GREEN}▸${NC} %s\n" "$*"; }
warn()  { printf "${YELLOW}▸${NC} %s\n" "$*"; }
error() { printf "${RED}✗${NC} %s\n" "$*" >&2; }
header() { printf "\n${BOLD}${CYAN}%s${NC}\n\n" "$*"; }

# --- Configuration ---
GITHUB_REPO="${LLSTACK_GITHUB_REPO:-web-casa/llstack}"
VERSION="${LLSTACK_VERSION:-}"
PREFIX="${LLSTACK_PREFIX:-/usr/local}"
BIN_DIR="$PREFIX/bin"
NO_INSTALL="${LLSTACK_NO_INSTALL:-}"

# --- Parse arguments ---
while [ $# -gt 0 ]; do
  case "$1" in
    --version)   VERSION="${2:-}"; shift 2 ;;
    --prefix)    PREFIX="${2:-}"; BIN_DIR="$PREFIX/bin"; shift 2 ;;
    --no-install) NO_INSTALL=1; shift ;;
    -h|--help)
      cat <<'HELP'
LLStack Installer

Usage: curl -sSL https://get.llstack.com | bash [-- OPTIONS]

Options:
  --version VERSION   Install specific version (default: latest)
  --prefix  PATH      Install prefix (default: /usr/local)
  --no-install        Only download binary, skip interactive setup
  -h, --help          Show this help

Environment:
  LLSTACK_VERSION     Same as --version
  LLSTACK_PREFIX      Same as --prefix
  LLSTACK_NO_INSTALL  Same as --no-install (set to 1)
HELP
      exit 0
      ;;
    *) error "Unknown option: $1"; exit 1 ;;
  esac
done

# --- Preflight checks ---
header "LLStack Installer"

# Must be root
if [ "$(id -u)" -ne 0 ]; then
  error "This installer must be run as root."
  echo "  sudo bash -c \"\$(curl -sSL https://get.llstack.com)\""
  exit 1
fi

# Check OS
if [ ! -f /etc/os-release ]; then
  error "Cannot detect OS. LLStack requires Rocky Linux / AlmaLinux / RHEL 9 or 10."
  exit 1
fi

. /etc/os-release
OS_MAJOR="${VERSION_ID%%.*}"

case "$ID" in
  rocky|almalinux|rhel|centos)
    case "$OS_MAJOR" in
      9|10) info "Detected: $PRETTY_NAME" ;;
      *) error "Unsupported OS version: $PRETTY_NAME (requires EL9 or EL10)"; exit 1 ;;
    esac
    ;;
  *)
    error "Unsupported OS: $PRETTY_NAME (requires Rocky/Alma/RHEL 9 or 10)"
    exit 1
    ;;
esac

# Check architecture
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64)  PLATFORM="linux-amd64" ;;
  aarch64) PLATFORM="linux-arm64" ;;
  *) error "Unsupported architecture: $ARCH"; exit 1 ;;
esac
info "Architecture: $ARCH ($PLATFORM)"

# Check download tool
DOWNLOADER=""
if command -v curl >/dev/null 2>&1; then
  DOWNLOADER="curl"
elif command -v wget >/dev/null 2>&1; then
  DOWNLOADER="wget"
else
  error "curl or wget is required. Install with: dnf install curl"
  exit 1
fi

# --- Resolve version ---
if [ -z "$VERSION" ]; then
  info "Resolving latest version..."
  if [ "$DOWNLOADER" = "curl" ]; then
    VERSION=$(curl -sSL "https://api.github.com/repos/$GITHUB_REPO/releases/latest" 2>/dev/null | grep '"tag_name"' | head -1 | cut -d'"' -f4)
  else
    VERSION=$(wget -qO- "https://api.github.com/repos/$GITHUB_REPO/releases/latest" 2>/dev/null | grep '"tag_name"' | head -1 | cut -d'"' -f4)
  fi
  if [ -z "$VERSION" ]; then
    warn "Could not detect latest version from GitHub. Using v0.1.0"
    VERSION="v0.1.0"
  fi
fi
info "Version: $VERSION"

# --- Download ---
DOWNLOAD_URL="https://github.com/$GITHUB_REPO/releases/download/$VERSION/llstack-$VERSION-$PLATFORM.tar.gz"
CHECKSUM_URL="https://github.com/$GITHUB_REPO/releases/download/$VERSION/checksums.txt"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

info "Downloading LLStack $VERSION for $PLATFORM..."
if [ "$DOWNLOADER" = "curl" ]; then
  curl -fsSL -o "$TMP_DIR/llstack.tar.gz" "$DOWNLOAD_URL" || {
    error "Download failed. Check version and network."
    error "URL: $DOWNLOAD_URL"
    exit 1
  }
  curl -fsSL -o "$TMP_DIR/checksums.txt" "$CHECKSUM_URL" 2>/dev/null || true
else
  wget -qO "$TMP_DIR/llstack.tar.gz" "$DOWNLOAD_URL" || {
    error "Download failed."
    exit 1
  }
  wget -qO "$TMP_DIR/checksums.txt" "$CHECKSUM_URL" 2>/dev/null || true
fi

# --- Verify checksum ---
if [ -f "$TMP_DIR/checksums.txt" ] && command -v sha256sum >/dev/null 2>&1; then
  EXPECTED=$(grep "llstack-$VERSION-$PLATFORM.tar.gz" "$TMP_DIR/checksums.txt" | awk '{print $1}')
  if [ -n "$EXPECTED" ]; then
    ACTUAL=$(sha256sum "$TMP_DIR/llstack.tar.gz" | awk '{print $1}')
    if [ "$EXPECTED" = "$ACTUAL" ]; then
      info "Checksum verified ✓"
    else
      error "Checksum mismatch!"
      error "Expected: $EXPECTED"
      error "Actual:   $ACTUAL"
      exit 1
    fi
  fi
fi

# --- Extract and install ---
info "Installing to $BIN_DIR/llstack..."
mkdir -p "$BIN_DIR"

# Backup existing binary
if [ -f "$BIN_DIR/llstack" ]; then
  BACKUP="$BIN_DIR/llstack.bak.$(date +%Y%m%d%H%M%S)"
  cp "$BIN_DIR/llstack" "$BACKUP"
  warn "Backed up existing binary to $BACKUP"
fi

tar xzf "$TMP_DIR/llstack.tar.gz" -C "$TMP_DIR"
BINARY=$(find "$TMP_DIR" -type f -name "llstack" | head -1)
if [ -z "$BINARY" ]; then
  error "Could not find llstack binary in archive"
  exit 1
fi

install -m 0755 "$BINARY" "$BIN_DIR/llstack"

# Verify installation
INSTALLED_VERSION=$("$BIN_DIR/llstack" version 2>/dev/null | head -1 || echo "unknown")
info "Installed: $BIN_DIR/llstack ($INSTALLED_VERSION)"

# --- Post-install ---
header "Installation Complete!"

echo ""
printf "  ${BOLD}LLStack $VERSION${NC} has been installed to ${CYAN}$BIN_DIR/llstack${NC}\n"
echo ""

if [ -n "$NO_INSTALL" ]; then
  echo "  Next steps:"
  echo "    llstack install         Interactive setup wizard"
  echo "    llstack --help          Show all commands"
  echo "    llstack tui             Open TUI interface"
  echo ""
  exit 0
fi

# Ask if user wants to run interactive installer
echo "  Would you like to run the interactive setup wizard now?"
echo "  This will install and configure your web stack (Apache/PHP/DB/Cache)."
echo ""
printf "  Run setup wizard? [Y/n] "
read -r REPLY
REPLY="${REPLY:-y}"

case "$REPLY" in
  [yY]|[yY][eE][sS]|"")
    echo ""
    exec "$BIN_DIR/llstack" install
    ;;
  *)
    echo ""
    echo "  You can run the wizard later with:"
    echo "    llstack install"
    echo ""
    echo "  Other useful commands:"
    echo "    llstack --help          Show all commands"
    echo "    llstack tui             Open TUI interface"
    echo "    llstack doctor          Run diagnostics"
    echo ""
    ;;
esac
