#!/bin/bash
set -euo pipefail
# Auto-backup all active sites from panel DB

PANEL_DB="${LLSTACK_DB_PATH:-/opt/llstack/data/llstack.db}"
SCRIPTS_DIR="${LLSTACK_SCRIPTS_DIR:-/opt/llstack/scripts}"
BACKUP_DIR="${1:-/opt/llstack/backups}"
LOG_FILE="/var/log/llstack-auto-backup.log"

log() { echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*" >> "$LOG_FILE"; }

[[ ! -f "$PANEL_DB" ]] && { log "Panel DB not found"; exit 0; }

mkdir -p "$BACKUP_DIR"

# Iterate all active sites
sqlite3 -json "$PANEL_DB" \
    "SELECT id, domain, user_id FROM sites WHERE status = 'active' AND staging_of IS NULL" 2>/dev/null | \
python3 -c "
import json, sys
sites = json.load(sys.stdin)
for s in sites:
    print(f\"{s['id']}|{s['domain']}\")" 2>/dev/null | while IFS='|' read -r SITE_ID DOMAIN; do
    # Validate domain
    [[ "$DOMAIN" =~ ^[a-zA-Z0-9.-]+$ ]] || { log "Skip invalid domain: $DOMAIN"; continue; }

    # Find linked DB: prefer site_id FK, fallback to wp-config.php
    DB_NAME=$(sqlite3 "$PANEL_DB" \
        "SELECT name FROM db_instances WHERE site_id = $SITE_ID LIMIT 1" 2>/dev/null)
    if [[ -z "$DB_NAME" ]]; then
        WP_PATH=$(sqlite3 "$PANEL_DB" \
            "SELECT path FROM wp_instances WHERE site_id = $SITE_ID LIMIT 1" 2>/dev/null)
        if [[ -n "$WP_PATH" && -f "$WP_PATH/wp-config.php" ]]; then
            DB_NAME=$(grep -oP "define\(\s*'DB_NAME'\s*,\s*'\\K[^']*" "$WP_PATH/wp-config.php" 2>/dev/null || echo "")
            # Validate
            [[ ! "$DB_NAME" =~ ^[a-zA-Z_][a-zA-Z0-9_]{0,63}$ ]] && DB_NAME=""
        fi
    fi

    OUTPUT="$BACKUP_DIR/${DOMAIN}_auto_$(date +%Y%m%d_%H%M%S).tar.gz"
    ARGS=(--site "$DOMAIN" --type full --output "$OUTPUT")
    [[ -n "$DB_NAME" ]] && ARGS+=(--db-name "$DB_NAME")

    log "Backing up $DOMAIN..."
    if bash "$SCRIPTS_DIR/backup/backup-create.sh" "${ARGS[@]}" >> "$LOG_FILE" 2>&1; then
        log "  Success: $OUTPUT"
    else
        log "  Failed"
    fi
done

# Cleanup old auto-backups (keep last 7 per site)
find "$BACKUP_DIR" -name "*_auto_*.tar.gz" -type f 2>/dev/null | \
    awk -F'_auto_' '{print $1}' | sort -u | while read -r prefix; do
    ls -t "${prefix}_auto_"*.tar.gz 2>/dev/null | tail -n +8 | xargs rm -f 2>/dev/null || true
done
