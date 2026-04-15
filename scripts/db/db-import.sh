#!/bin/bash
set -euo pipefail

# Import/restore a database from SQL dump
# Usage: db-import.sh --engine <mariadb|mysql|postgresql> --name <db_name> --file <path>

ENGINE="" NAME="" FILE=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --engine) ENGINE="$2"; shift 2 ;;
        --name)   NAME="$2"; shift 2 ;;
        --file)   FILE="$2"; shift 2 ;;
        *) echo '{"ok":false,"error":"unknown_arg"}' >&2; exit 1 ;;
    esac
done

[[ -z "$ENGINE" || -z "$NAME" || -z "$FILE" ]] && { echo '{"ok":false,"error":"missing_args"}' >&2; exit 1; }
[[ ! -f "$FILE" ]] && { echo '{"ok":false,"error":"file_not_found"}' >&2; exit 1; }

# Validate DB name
if ! echo "$NAME" | grep -qP '^[a-zA-Z][a-zA-Z0-9_]{0,63}$'; then
    echo '{"ok":false,"error":"invalid_db_name"}' >&2; exit 1
fi

case "$ENGINE" in
    mariadb|mysql)
        if [[ "$FILE" == *.gz ]]; then
            gunzip -c "$FILE" | mysql "$NAME" 2>/dev/null
        else
            mysql "$NAME" < "$FILE" 2>/dev/null
        fi
        ;;
    postgresql)
        if [[ "$FILE" == *.gz ]]; then
            gunzip -c "$FILE" | sudo -u postgres psql "$NAME" 2>/dev/null
        else
            sudo -u postgres psql "$NAME" < "$FILE" 2>/dev/null
        fi
        ;;
    *) echo '{"ok":false,"error":"unsupported_engine"}' >&2; exit 1 ;;
esac

# Note: caller handles file cleanup
echo '{"ok":true,"data":{"database":"'"$NAME"'","engine":"'"$ENGINE"'"}}'
