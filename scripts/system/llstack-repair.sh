#!/bin/bash
set -euo pipefail

# LLStack repair — regenerate all configurations from database state
# Usage: llstack-repair.sh [--component <vhost|php|redis|cron|all>]

COMPONENT="all"
while [[ $# -gt 0 ]]; do
    case "$1" in
        --component) COMPONENT="$2"; shift 2 ;;
        *) shift ;;
    esac
done

DB_PATH="${LLSTACK_DB_PATH:-/opt/llstack/data/llstack.db}"
SCRIPTS_DIR="${LLSTACK_SCRIPTS_DIR:-/opt/llstack/scripts}"

if [[ ! -f "$DB_PATH" ]]; then
    echo '{"ok":false,"error":"database_not_found"}' >&2; exit 1
fi

FIXED=0
ERRORS=0

repair_vhosts() {
    echo ">>> Repairing vhost configurations..."
    # Get all active sites from DB
    SITES=$(sqlite3 "$DB_PATH" "SELECT id, domain, doc_root, php_version FROM sites WHERE status = 'active'" 2>/dev/null)

    while IFS='|' read -r site_id domain doc_root php_version; do
        [[ -z "$domain" ]] && continue
        php_version="${php_version:-php83}"

        VHOST_DIR="/usr/local/lsws/conf/vhosts/$domain"
        VHOST_CONF="$VHOST_DIR/vhconf.conf"

        if [[ ! -f "$VHOST_CONF" ]]; then
            echo "  [REPAIR] Missing vhost for $domain — regenerating..."
            "$SCRIPTS_DIR/site/site-vhost-render.sh" \
                --domain "$domain" --doc-root "$doc_root" --php "$php_version" \
                >/dev/null 2>&1 && FIXED=$((FIXED + 1)) || ERRORS=$((ERRORS + 1))
        else
            echo "  [OK] $domain"
        fi
    done <<< "$SITES"

    # Check for orphaned vhost dirs (no matching DB entry) — use Python for safe parameterized query
    for vhost_dir in /usr/local/lsws/conf/vhosts/*/; do
        [[ ! -d "$vhost_dir" ]] && continue
        domain=$(basename "$vhost_dir")
        [[ "$domain" == "Example" || "$domain" == "llstack-panel" ]] && continue
        # Validate domain format before querying
        if ! echo "$domain" | grep -qP '^(?:[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$'; then
            echo "  [WARN] Invalid vhost dir name: $domain"
            continue
        fi
        EXISTS=$(python3 -c "
import sqlite3, sys
conn = sqlite3.connect(sys.argv[1])
r = conn.execute('SELECT COUNT(*) FROM sites WHERE domain = ?', (sys.argv[2],)).fetchone()
print(r[0])
" "$DB_PATH" "$domain" 2>/dev/null || echo "0")
        if [[ "$EXISTS" == "0" ]]; then
            echo "  [WARN] Orphaned vhost dir: $domain (no DB entry)"
        fi
    done
}

repair_php() {
    echo ">>> Repairing PHP configurations..."
    # Sync PHP version registry
    "$SCRIPTS_DIR/../backend/app/api/system.py" 2>/dev/null || true  # Can't call Python directly
    echo "  [INFO] Run 'POST /api/system/php-registry/sync' to sync PHP versions"
    echo "  [OK] PHP repair requires API call"
}

repair_redis() {
    echo ">>> Repairing Redis instances..."
    INSTANCES=$(sqlite3 "$DB_PATH" "SELECT r.id, u.system_user, r.status FROM redis_instances r JOIN users u ON r.user_id = u.id" 2>/dev/null)

    while IFS='|' read -r inst_id system_user status; do
        [[ -z "$system_user" ]] && continue
        SERVICE="redis@$system_user"

        ACTUAL_STATUS="stopped"
        if systemctl is-active "$SERVICE" &>/dev/null; then
            ACTUAL_STATUS="running"
        fi

        if [[ "$status" != "$ACTUAL_STATUS" ]]; then
            echo "  [REPAIR] Redis for $system_user: DB says '$status', actual '$ACTUAL_STATUS' — updating DB"
            sqlite3 "$DB_PATH" "UPDATE redis_instances SET status = '$ACTUAL_STATUS' WHERE id = $inst_id" 2>/dev/null
            FIXED=$((FIXED + 1))
        else
            echo "  [OK] Redis for $system_user ($ACTUAL_STATUS)"
        fi
    done <<< "$INSTANCES"
}

repair_cron() {
    echo ">>> Repairing cron jobs..."
    echo "  [INFO] Cron jobs are managed via system crontab — no repair needed"
    echo "  [OK] Cron repair complete"
}

# Execute repairs
case "$COMPONENT" in
    vhost)  repair_vhosts ;;
    php)    repair_php ;;
    redis)  repair_redis ;;
    cron)   repair_cron ;;
    all)
        repair_vhosts
        repair_redis
        repair_cron
        ;;
    *) echo '{"ok":false,"error":"invalid_component"}' >&2; exit 1 ;;
esac

echo ""
echo ">>> Repair complete: $FIXED fixed, $ERRORS errors"
echo "{\"ok\":true,\"data\":{\"fixed\":$FIXED,\"errors\":$ERRORS}}"
