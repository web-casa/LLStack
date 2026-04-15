#!/bin/bash
set -euo pipefail
USERNAME=""
while [[ $# -gt 0 ]]; do case "$1" in --username) USERNAME="$2"; shift 2 ;; *) shift ;; esac; done
[[ -z "$USERNAME" ]] && { echo '{"ok":false,"error":"missing_args"}' >&2; exit 1; }
# Stop user services
systemctl stop "redis@$USERNAME" 2>/dev/null || true
systemctl disable "redis@$USERNAME" 2>/dev/null || true
# Remove user (keep home dir by default for safety)
userdel "$USERNAME" 2>/dev/null || true
echo "{\"ok\":true,\"data\":{\"username\":\"$USERNAME\"}}"
