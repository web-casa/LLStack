#!/bin/bash
set -euo pipefail

# WP Auto-Update Checker — run from system cron every 30 minutes
# Reads auto-update configs from panel DB, executes updates in matching time windows
# Supports auto-rollback on failure (HTTP 5xx check)

PANEL_DB="${LLSTACK_DB_PATH:-/opt/llstack/data/llstack.db}"
SCRIPTS_DIR="${LLSTACK_SCRIPTS_DIR:-/opt/llstack/scripts}"
LOG_FILE="/var/log/llstack-auto-update.log"

log() { echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*" >> "$LOG_FILE"; }

[[ ! -f "$PANEL_DB" ]] && exit 0

CURRENT_HOUR=$(date +%H)
CURRENT_DOW=$(date +%w)  # 0=Sunday
CURRENT_DOM=$(date +%d)

# Read all auto-update configs from panel_config
CONFIGS=$(sqlite3 -json "$PANEL_DB" \
    "SELECT key, value FROM panel_config WHERE key LIKE 'wp_auto_update_%'" 2>/dev/null || echo "[]")

[[ "$CONFIGS" == "[]" ]] && exit 0

echo "$CONFIGS" | python3 -c "
import json, sys
configs = json.load(sys.stdin)
for row in configs:
    wp_id = row['key'].replace('wp_auto_update_', '')
    cfg = json.loads(row['value'])
    if cfg.get('enabled'):
        print(f\"{wp_id}|{cfg.get('frequency','weekly')}|{cfg.get('hour',3)}|{cfg.get('day_of_week',0)}|{int(cfg.get('update_core',True))}|{int(cfg.get('update_plugins',True))}|{int(cfg.get('update_themes',True))}|{int(cfg.get('auto_rollback',True))}\")
" 2>/dev/null | while IFS='|' read -r WP_ID FREQ HOUR DOW UPDATE_CORE UPDATE_PLUGINS UPDATE_THEMES AUTO_ROLLBACK; do

    # Check if this is the right time window
    [[ "$CURRENT_HOUR" != "$HOUR" ]] && continue

    case "$FREQ" in
        daily) ;; # run every day at the configured hour
        weekly)
            [[ "$CURRENT_DOW" != "$DOW" ]] && continue
            ;;
        monthly)
            [[ "$CURRENT_DOM" != "01" ]] && continue
            ;;
    esac

    # Validate WP_ID is numeric
    [[ "$WP_ID" =~ ^[0-9]+$ ]] || continue

    # Get WP instance info
    WP_INFO=$(sqlite3 -json "$PANEL_DB" \
        "SELECT w.path, s.domain, s.doc_root, s.user_id FROM wp_instances w JOIN sites s ON w.site_id = s.id WHERE w.id = $WP_ID" 2>/dev/null || echo "[]")

    WP_PATH=$(echo "$WP_INFO" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d[0]['path'] if d else '')" 2>/dev/null)
    DOMAIN=$(echo "$WP_INFO" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d[0]['domain'] if d else '')" 2>/dev/null)
    SITE_USER=$(stat -c '%U' "$WP_PATH" 2>/dev/null || echo "root")

    [[ -z "$WP_PATH" || ! -f "$WP_PATH/wp-config.php" ]] && continue

    log "Starting auto-update for WP #$WP_ID ($DOMAIN)"

    # Create backup before update
    if [[ "$AUTO_ROLLBACK" == "1" ]]; then
        BACKUP_DIR="/opt/llstack/backups/auto-update"
        mkdir -p "$BACKUP_DIR"
        BACKUP_FILE="$BACKUP_DIR/${DOMAIN}_$(date +%Y%m%d_%H%M%S).tar.gz"
        tar czf "$BACKUP_FILE" -C "$(dirname "$WP_PATH")" "$(basename "$WP_PATH")" 2>/dev/null || true

        # Backup database
        DB_NAME=$(grep -oP "define\(\s*'DB_NAME'\s*,\s*'\\K[^']*" "$WP_PATH/wp-config.php" 2>/dev/null || echo "")
        # Validate DB_NAME is a safe identifier
        [[ ! "$DB_NAME" =~ ^[a-zA-Z0-9_]+$ ]] && DB_NAME=""
        if [[ -n "$DB_NAME" ]]; then
            mysqldump "$DB_NAME" 2>/dev/null | gzip > "$BACKUP_DIR/${DOMAIN}_db_$(date +%Y%m%d_%H%M%S).sql.gz" || true
        fi
        log "  Backup created: $BACKUP_FILE"
    fi

    UPDATE_FAILED=false

    # Update core
    if [[ "$UPDATE_CORE" == "1" ]]; then
        if ! sudo -u "$SITE_USER" wp core update --path="$WP_PATH" 2>>"$LOG_FILE"; then
            log "  Core update failed"
            UPDATE_FAILED=true
        else
            log "  Core updated"
        fi
    fi

    # Update plugins
    if [[ "$UPDATE_PLUGINS" == "1" ]] && [[ "$UPDATE_FAILED" == "false" ]]; then
        if ! sudo -u "$SITE_USER" wp plugin update --all --path="$WP_PATH" 2>>"$LOG_FILE"; then
            log "  Plugin update failed"
            UPDATE_FAILED=true
        else
            log "  Plugins updated"
        fi
    fi

    # Update themes
    if [[ "$UPDATE_THEMES" == "1" ]] && [[ "$UPDATE_FAILED" == "false" ]]; then
        if ! sudo -u "$SITE_USER" wp theme update --all --path="$WP_PATH" 2>>"$LOG_FILE"; then
            log "  Theme update failed"
            UPDATE_FAILED=true
        else
            log "  Themes updated"
        fi
    fi

    # Post-update health check
    if [[ "$UPDATE_FAILED" == "false" ]]; then
        HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" --max-time 10 "https://$DOMAIN/" 2>/dev/null || echo "000")
        if [[ "$HTTP_CODE" -ge 500 ]] || [[ "$HTTP_CODE" == "000" ]]; then
            log "  Health check FAILED (HTTP $HTTP_CODE)"
            UPDATE_FAILED=true
        else
            log "  Health check passed (HTTP $HTTP_CODE)"
        fi
    fi

    # Auto-rollback on failure
    if [[ "$UPDATE_FAILED" == "true" ]] && [[ "$AUTO_ROLLBACK" == "1" ]] && [[ -f "$BACKUP_FILE" ]]; then
        log "  Rolling back from backup..."
        tar xzf "$BACKUP_FILE" -C "$(dirname "$WP_PATH")" 2>/dev/null || true
        if [[ -n "${DB_NAME:-}" ]]; then
            DB_BACKUP=$(ls -t "$BACKUP_DIR/${DOMAIN}_db_"*.sql.gz 2>/dev/null | head -1)
            if [[ -n "$DB_BACKUP" ]]; then
                zcat "$DB_BACKUP" | mysql "$DB_NAME" 2>/dev/null || true
            fi
        fi
        log "  Rollback completed"
    fi

    # Record result in panel DB
    RESULT="success"
    [[ "$UPDATE_FAILED" == "true" ]] && RESULT="failed_rollback"
    sqlite3 "$PANEL_DB" "INSERT INTO panel_config (key, value) VALUES ('wp_auto_update_last_${WP_ID}', '{\"time\":\"$(date -Iseconds)\",\"result\":\"$RESULT\"}') ON CONFLICT(key) DO UPDATE SET value = excluded.value" 2>/dev/null || true

    # Clean up old auto-update backups (keep last 3)
    if [[ -d "$BACKUP_DIR" ]]; then
        ls -t "$BACKUP_DIR/${DOMAIN}_"*.tar.gz 2>/dev/null | tail -n +4 | xargs rm -f 2>/dev/null || true
        ls -t "$BACKUP_DIR/${DOMAIN}_db_"*.sql.gz 2>/dev/null | tail -n +4 | xargs rm -f 2>/dev/null || true
    fi

    log "Auto-update for $DOMAIN completed: $RESULT"
done
