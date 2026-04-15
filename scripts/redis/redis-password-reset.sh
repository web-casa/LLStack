#!/bin/bash
set -euo pipefail
USER="" PASSWORD="${REDIS_PASSWORD:-}"
while [[ $# -gt 0 ]]; do case "$1" in --user) USER="$2"; shift 2 ;; --password) PASSWORD="$2"; shift 2 ;; *) shift ;; esac; done
[[ -z "$USER" || -z "$PASSWORD" ]] && { echo '{"ok":false,"error":"missing_args"}' >&2; exit 1; }
# Reject passwords with control characters (prevent config injection)
if echo "$PASSWORD" | grep -qP '[\x00-\x1f]'; then
    echo '{"ok":false,"error":"invalid_password_chars"}' >&2; exit 1
fi
CONF="/home/$USER/.redis/redis.conf"
[[ ! -f "$CONF" ]] && { echo '{"ok":false,"error":"conf_not_found"}' >&2; exit 1; }
# Escape special chars for sed replacement
ESCAPED_PW=$(printf '%s\n' "$PASSWORD" | sed -e 's/[|&/\\]/\\&/g')
sed -i "s|^requirepass .*|requirepass $ESCAPED_PW|" "$CONF"
systemctl restart "redis@$USER" 2>/dev/null || true
echo '{"ok":true}'
