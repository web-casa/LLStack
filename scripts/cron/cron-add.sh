#!/bin/bash
set -euo pipefail
USER="" EXPRESSION="" COMMAND=""
while [[ $# -gt 0 ]]; do case "$1" in --user) USER="$2"; shift 2 ;; --expression) EXPRESSION="$2"; shift 2 ;; --command) COMMAND="$2"; shift 2 ;; *) shift ;; esac; done
[[ -z "$USER" || -z "$EXPRESSION" || -z "$COMMAND" ]] && { echo '{"ok":false,"error":"missing_args"}' >&2; exit 1; }
# Reject control characters to prevent crontab injection
if printf '%s%s' "$EXPRESSION" "$COMMAND" | grep -qP '[\x00-\x09\x0b-\x1f]'; then
    echo '{"ok":false,"error":"invalid_chars"}' >&2; exit 1
fi
(crontab -u "$USER" -l 2>/dev/null; echo "$EXPRESSION $COMMAND") | crontab -u "$USER" -
echo '{"ok":true}'
