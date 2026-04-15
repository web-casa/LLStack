#!/bin/bash
set -euo pipefail

# Automated restic backup — called by cron
# Backs up all active sites + their databases

REPO="/opt/llstack/backups/restic"
PW_FILE="/opt/llstack/data/.restic_password"
DB_PATH="${LLSTACK_DB_PATH:-/opt/llstack/data/llstack.db}"
SCRIPTS_DIR="$(cd "$(dirname "$0")/.." && pwd)"

if [[ ! -f "$PW_FILE" ]]; then
    echo "[$(date)] ERROR: Restic password file not found" >&2
    exit 1
fi

echo "[$(date)] Starting automated backup..."

# Get all active sites
SITES=$(sqlite3 "$DB_PATH" "SELECT domain, doc_root FROM sites WHERE status = 'active'" 2>/dev/null || true)

if [[ -z "$SITES" ]]; then
    echo "[$(date)] No active sites to backup"
    exit 0
fi

BACKED_UP=0
ERRORS=0

while IFS='|' read -r domain doc_root; do
    [[ -z "$domain" || -z "$doc_root" ]] && continue
    [[ ! -d "$doc_root" ]] && continue

    echo "[$(date)] Backing up: $domain"

    # Check for associated database (use Python for safe parameterized query)
    DB_NAME=$(python3 -c "
import sqlite3, sys
conn = sqlite3.connect(sys.argv[1])
r = conn.execute('SELECT di.name FROM db_instances di WHERE di.user_id = (SELECT user_id FROM sites WHERE domain = ?) LIMIT 1', (sys.argv[2],)).fetchone()
print(r[0] if r else '')
" "$DB_PATH" "$domain" 2>/dev/null || true)

    ARGS=(--repo "$REPO" --password-file "$PW_FILE" --paths "$doc_root" --tag "$domain")

    if [[ -n "$DB_NAME" ]]; then
        ARGS+=(--db-name "$DB_NAME" --db-engine "mariadb")
    fi

    if "$SCRIPTS_DIR/backup/backup-restic-snapshot.sh" "${ARGS[@]}" >/dev/null 2>&1; then
        BACKED_UP=$((BACKED_UP + 1))
        echo "[$(date)]   OK: $domain"
    else
        ERRORS=$((ERRORS + 1))
        echo "[$(date)]   FAILED: $domain" >&2
    fi
done <<< "$SITES"

# Apply retention policy from config
KEEP_LAST=$(sqlite3 "$DB_PATH" "SELECT json_extract(value, '$.retention.keep_last') FROM panel_config WHERE key = 'backup_schedule'" 2>/dev/null || echo "7")
KEEP_DAILY=$(sqlite3 "$DB_PATH" "SELECT json_extract(value, '$.retention.keep_daily') FROM panel_config WHERE key = 'backup_schedule'" 2>/dev/null || echo "30")

[[ -z "$KEEP_LAST" ]] && KEEP_LAST=7
[[ -z "$KEEP_DAILY" ]] && KEEP_DAILY=30

echo "[$(date)] Applying retention: keep-last=$KEEP_LAST, keep-daily=$KEEP_DAILY"
"$SCRIPTS_DIR/backup/backup-restic-forget.sh" \
    --repo "$REPO" --password-file "$PW_FILE" \
    --keep-last "$KEEP_LAST" --keep-daily "$KEEP_DAILY" \
    >/dev/null 2>&1 || true

echo "[$(date)] Backup complete: $BACKED_UP succeeded, $ERRORS failed"
