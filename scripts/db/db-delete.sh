#!/bin/bash
set -euo pipefail
ENGINE="" NAME=""
while [[ $# -gt 0 ]]; do case "$1" in --engine) ENGINE="$2"; shift 2 ;; --name) NAME="$2"; shift 2 ;; *) shift ;; esac; done
[[ -z "$ENGINE" || -z "$NAME" ]] && { echo '{"ok":false,"error":"missing_args"}' >&2; exit 1; }
case "$ENGINE" in
    mariadb|mysql) mysql -e "DROP DATABASE IF EXISTS \`$NAME\`;" 2>/dev/null ;;
    postgresql) sudo -u postgres psql -c "DROP DATABASE IF EXISTS \"$NAME\";" 2>/dev/null ;;
esac
echo "{\"ok\":true,\"data\":{\"name\":\"$NAME\",\"engine\":\"$ENGINE\"}}"
