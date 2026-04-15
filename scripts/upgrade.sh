#!/bin/bash
set -euo pipefail

# LLStack Panel Upgrade Script
# Usage: upgrade.sh [--version <tag>]

LLSTACK_DIR="/opt/llstack"
LLSTACK_REPO="https://github.com/web-casa/LLStack"
TARGET_VERSION=""

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

log()  { echo -e "${GREEN}[LLStack Upgrade]${NC} $*"; }
err()  { echo -e "${RED}[ERROR]${NC} $*" >&2; }

while [[ $# -gt 0 ]]; do
    case "$1" in
        --version) TARGET_VERSION="$2"; shift 2 ;;
        *) shift ;;
    esac
done

# Check root
if [[ $EUID -ne 0 ]]; then
    err "Must be run as root"
    exit 1
fi

# Check existing installation
if [[ ! -d "$LLSTACK_DIR" ]]; then
    err "LLStack not found at $LLSTACK_DIR. Run install.sh first."
    exit 1
fi

log "Starting upgrade..."

# 1. Backup current installation
BACKUP_DIR="/opt/llstack-backup-$(date +%Y%m%d%H%M%S)"
log "Backing up to $BACKUP_DIR..."
cp -r "$LLSTACK_DIR/backend" "$BACKUP_DIR-backend"
cp -r "$LLSTACK_DIR/data" "$BACKUP_DIR-data"

# 2. Download new version
log "Downloading new version..."
if [[ -d "/opt/llstack-panel" ]]; then
    # Dev mode: copy from local
    log "Dev mode: syncing from /opt/llstack-panel"
    # Clean old backend app/ to prevent .py shadowing compiled .so
    rm -rf "$LLSTACK_DIR/backend/app"
    rsync -a --exclude 'node_modules' --exclude '.venv' --exclude '__pycache__' --exclude 'dist' \
        /opt/llstack-panel/backend/ "$LLSTACK_DIR/backend/"
    rsync -a --exclude 'node_modules' --exclude 'dist' \
        /opt/llstack-panel/web/ "$LLSTACK_DIR/web/"
    rsync -a /opt/llstack-panel/scripts/ "$LLSTACK_DIR/scripts/"
else
    # Production: git pull or clone
    TMPDIR=$(mktemp -d)
    if [[ -n "$TARGET_VERSION" ]]; then
        git clone --depth 1 --branch "$TARGET_VERSION" "$LLSTACK_REPO" "$TMPDIR" 2>&1 | tail -1
    else
        git clone --depth 1 "$LLSTACK_REPO" "$TMPDIR" 2>&1 | tail -1
    fi
    # Clean old backend app/ to prevent .py shadowing compiled .so
    rm -rf "$LLSTACK_DIR/backend/app"
    rsync -a "$TMPDIR/backend/" "$LLSTACK_DIR/backend/"
    rsync -a "$TMPDIR/web/" "$LLSTACK_DIR/web/"
    rsync -a "$TMPDIR/scripts/" "$LLSTACK_DIR/scripts/"
    rm -rf "$TMPDIR"
fi

# 3. Update Python dependencies
log "Updating Python dependencies..."
"$LLSTACK_DIR/backend/.venv/bin/pip" install -q -r "$LLSTACK_DIR/backend/requirements.txt" 2>&1 | tail -1

# 4. Frontend — pre-built dist/ included in release
log "Frontend dist/ updated (no build needed)"

# 5. Run database migrations
log "Running migrations..."
cd "$LLSTACK_DIR/backend"
LLSTACK_DB_PATH="$LLSTACK_DIR/data/llstack.db" .venv/bin/python -c "
from app import create_app
app = create_app({'TURSO_DB_PATH': '$LLSTACK_DIR/data/llstack.db'})
print('Migrations applied')
"

# 6. Fix permissions
chown -R llstack:llstack "$LLSTACK_DIR"
chmod +x "$LLSTACK_DIR/scripts"/*/*.sh 2>/dev/null || true

# 7. Restart service
log "Restarting LLStack..."
systemctl restart llstack 2>/dev/null || {
    # If systemd service doesn't exist, restart gunicorn
    pkill -f "gunicorn.*serve_app" 2>/dev/null || true
    sleep 1
    cd "$LLSTACK_DIR/backend"
    LLSTACK_DB_PATH="$LLSTACK_DIR/data/llstack.db" \
        .venv/bin/gunicorn -w 1 --threads 4 -b 127.0.0.1:8001 serve_app:app --daemon \
        --log-file /var/log/llstack.log --timeout 120
}

sleep 2

# 8. Verify
if curl -s http://127.0.0.1:30333/api/auth/need-setup -X POST | grep -q '"code":0'; then
    log "Upgrade complete!"
    log "Backup saved at: $BACKUP_DIR-*"
else
    err "Upgrade may have failed. Check /var/log/llstack.log"
    err "Restore backup: cp -r $BACKUP_DIR-backend $LLSTACK_DIR/backend && cp -r $BACKUP_DIR-data $LLSTACK_DIR/data"
    exit 1
fi
