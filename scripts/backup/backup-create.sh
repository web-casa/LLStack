#!/bin/bash
set -euo pipefail

# Create site backup (files + database)
# Usage: backup-create.sh --site <domain> --type <full|files|db> --output <path>

SITE=""
TYPE="full"
OUTPUT=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --site)   SITE="$2"; shift 2 ;;
        --type)   TYPE="$2"; shift 2 ;;
        --output) OUTPUT="$2"; shift 2 ;;
        *) echo '{"ok": false, "error": "unknown_arg"}' >&2; exit 1 ;;
    esac
done

if [[ -z "$SITE" || -z "$OUTPUT" ]]; then
    echo '{"ok": false, "error": "missing_args"}' >&2
    exit 1
fi

# Prevent concurrent backups for same site
LOCK_FILE="/var/lock/llstack-backup-$(echo "$SITE" | tr '/' '_').lock"
exec 100>"$LOCK_FILE"
if ! flock -n 100; then
    echo '{"ok": false, "error": "backup_already_running"}' >&2
    exit 1
fi

# Find doc_root
DOC_ROOT=""
VHCONF="/usr/local/lsws/conf/vhosts/$SITE/vhconf.conf"
if [[ -f "$VHCONF" ]]; then
    DOC_ROOT=$(grep 'docRoot' "$VHCONF" | awk '{print $2}' | head -1)
fi

# Fallback: search home directories
if [[ -z "$DOC_ROOT" || ! -d "$DOC_ROOT" ]]; then
    for d in /home/*/public_html/"$SITE"; do
        if [[ -d "$d" ]]; then
            DOC_ROOT="$d"
            break
        fi
    done
fi

mkdir -p "$(dirname "$OUTPUT")"
TMPDIR=$(mktemp -d -t llstack-backup.XXXXXXXXXX)
[[ -L "$TMPDIR" ]] && { echo '{"ok":false,"error":"tmpdir_symlink"}' >&2; exit 1; }
trap 'rm -rf "$TMPDIR"' EXIT

# Backup files
if [[ "$TYPE" == "full" || "$TYPE" == "files" ]]; then
    if [[ -d "$DOC_ROOT" ]]; then
        tar czf "$TMPDIR/files.tar.gz" -C "$(dirname "$DOC_ROOT")" "$(basename "$DOC_ROOT")" 2>/dev/null
    fi
fi

# Backup database (attempt MySQL dump if db exists with site name patterns)
if [[ "$TYPE" == "full" || "$TYPE" == "db" ]]; then
    DB_NAME=$(echo "$SITE" | tr '.' '_' | tr '-' '_')
    # Validate derived DB name
    if ! echo "$DB_NAME" | grep -qP '^[a-zA-Z_][a-zA-Z0-9_]{0,63}$'; then
        echo "    Skipping database: invalid derived name '$DB_NAME'"
    elif mysql -e "USE \`$DB_NAME\`" 2>/dev/null; then
        mysqldump "$DB_NAME" 2>/dev/null | gzip > "$TMPDIR/database.sql.gz"
    fi
fi

# Create final archive
cd "$TMPDIR"
tar czf "$OUTPUT" ./* 2>/dev/null

SIZE=$(stat -c%s "$OUTPUT" 2>/dev/null || echo 0)

cat << EOF
{"ok": true, "data": {"path": "$OUTPUT", "size": $SIZE, "type": "$TYPE"}}
EOF
