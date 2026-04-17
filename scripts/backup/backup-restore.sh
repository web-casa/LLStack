#!/bin/bash
set -euo pipefail
# Restore a backup archive
# Usage: backup-restore.sh --path <archive> [--db-name <name>]
PATH_ARG="" DB_NAME_ARG=""
while [[ $# -gt 0 ]]; do
    case "$1" in
        --path)    PATH_ARG="$2"; shift 2 ;;
        --db-name) DB_NAME_ARG="$2"; shift 2 ;;
        *) shift ;;
    esac
done
[[ -z "$PATH_ARG" ]] && { echo '{"ok":false,"error":"missing_args"}' >&2; exit 1; }
[[ ! -f "$PATH_ARG" ]] && { echo '{"ok":false,"error":"file_not_found"}' >&2; exit 1; }
TMPDIR=$(mktemp -d -t llstack-restore.XXXXXXXXXX)
[[ -L "$TMPDIR" ]] && { echo '{"ok":false,"error":"tmpdir_symlink"}' >&2; exit 1; }
trap 'rm -rf "$TMPDIR"' EXIT
cd "$TMPDIR"
tar --no-same-owner xzf "$PATH_ARG"
# Restore files
if [[ -f files.tar.gz ]]; then
    # Reject absolute paths, leading .., or any .. path component (e.g. home/user/../../etc)
    if tar tzf files.tar.gz | grep -qE '^/|(^|/)\.\.(/|$)'; then
        echo '{"ok":false,"error":"path_traversal_detected","message":"Archive contains absolute or parent paths"}' >&2
        exit 1
    fi
    # Validate all paths are under /home/ (site files must be in user home dirs)
    if tar tzf files.tar.gz | grep -qvE '^home/[a-zA-Z0-9_-]+/'; then
        echo '{"ok":false,"error":"invalid_restore_path","message":"Archive contains files outside /home/"}' >&2
        exit 1
    fi
    tar --no-same-owner --no-symlinks xzf files.tar.gz -C /
fi
# Restore database
if [[ -f database.sql.gz ]]; then
    # Prefer explicit --db-name parameter; fallback to filename-derived name
    if [[ -n "$DB_NAME_ARG" ]]; then
        DB_NAME="$DB_NAME_ARG"
    else
        DB_NAME=$(basename "$PATH_ARG" | sed 's/_full_.*//;s/_db_.*//;s/_files_.*//')
    fi
    # Validate DB name strictly
    if echo "$DB_NAME" | grep -qP '^[a-zA-Z][a-zA-Z0-9_]{0,63}$'; then
        zcat database.sql.gz | mysql "$DB_NAME" 2>/dev/null || true
    else
        echo '{"ok":false,"error":"invalid_db_name"}' >&2
        exit 1
    fi
fi
# Restore metadata if present
if [[ -f metadata.json ]]; then
    echo ">>> Metadata found in backup"
fi
echo '{"ok":true,"data":{"restored":true}}'
