#!/bin/bash
set -euo pipefail

# Clone a database
# Usage: db-clone.sh --engine <engine> --source <db_name> --target <db_name>

ENGINE="" SOURCE="" TARGET=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --engine) ENGINE="$2"; shift 2 ;;
        --source) SOURCE="$2"; shift 2 ;;
        --target) TARGET="$2"; shift 2 ;;
        *) echo '{"ok":false,"error":"unknown_arg"}' >&2; exit 1 ;;
    esac
done

[[ -z "$ENGINE" || -z "$SOURCE" || -z "$TARGET" ]] && { echo '{"ok":false,"error":"missing_args"}' >&2; exit 1; }

for name in "$SOURCE" "$TARGET"; do
    if ! echo "$name" | grep -qP '^[a-zA-Z][a-zA-Z0-9_]{0,63}$'; then
        echo '{"ok":false,"error":"invalid_db_name"}' >&2; exit 1
    fi
done

[[ "$SOURCE" == "$TARGET" ]] && { echo '{"ok":false,"error":"source_equals_target"}' >&2; exit 1; }

case "$ENGINE" in
    mariadb|mysql)
        mysql -e "CREATE DATABASE IF NOT EXISTS \`$TARGET\` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"
        mysqldump --single-transaction --quick --routines --triggers --events "$SOURCE" 2>/dev/null | mysql "$TARGET"
        ;;
    postgresql)
        # Terminate active connections to source DB (required for TEMPLATE)
        # SOURCE is pre-validated: ^[a-zA-Z][a-zA-Z0-9_]{0,63}$ — safe for SQL string literal
        sudo -u postgres psql -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '$SOURCE' AND pid <> pg_backend_pid();" 2>/dev/null || true
        if ! sudo -u postgres psql -c "CREATE DATABASE \"$TARGET\" TEMPLATE \"$SOURCE\";" 2>/dev/null; then
            # Fallback: dump + restore if TEMPLATE fails
            sudo -u postgres psql -c "CREATE DATABASE \"$TARGET\";" 2>/dev/null
            sudo -u postgres pg_dump "$SOURCE" 2>/dev/null | sudo -u postgres psql "$TARGET" 2>/dev/null
        fi
        ;;
    *) echo '{"ok":false,"error":"unsupported_engine"}' >&2; exit 1 ;;
esac

echo "{\"ok\":true,\"data\":{\"source\":\"$SOURCE\",\"target\":\"$TARGET\",\"engine\":\"$ENGINE\"}}"
