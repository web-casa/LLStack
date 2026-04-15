#!/bin/bash
set -euo pipefail
USER=""
while [[ $# -gt 0 ]]; do case "$1" in --user) USER="$2"; shift 2 ;; *) shift ;; esac; done
[[ -z "$USER" ]] && { echo '{"ok":false,"error":"missing_args"}' >&2; exit 1; }
systemctl stop "redis@$USER" 2>/dev/null || true
systemctl disable "redis@$USER" 2>/dev/null || true
rm -rf "/home/$USER/.redis"
echo "{\"ok\":true,\"data\":{\"user\":\"$USER\"}}"
