#!/bin/bash
set -euo pipefail

# Remove cgroup resource limits for a user
# Usage: cgroup-remove.sh --user <system_user>

USER=""
while [[ $# -gt 0 ]]; do
    case "$1" in
        --user) USER="$2"; shift 2 ;;
        *) shift ;;
    esac
done

[[ -z "$USER" ]] && { echo '{"ok":false,"error":"missing_user"}' >&2; exit 1; }

LSCGCTL="/usr/local/lsws/lsns/bin/lscgctl"

# Try lscgctl first
if [[ -x "$LSCGCTL" ]] && "$LSCGCTL" version &>/dev/null; then
    "$LSCGCTL" remove "$USER" 2>/dev/null || true
    echo '{"ok":true,"data":{"method":"lscgctl"}}'
    exit 0
fi

# Fallback: reset systemd slice properties
UID_NUM=$(id -u "$USER" 2>/dev/null || echo "0")
if [[ "$UID_NUM" -gt 0 ]]; then
    SLICE="user-${UID_NUM}.slice"
    systemctl set-property "$SLICE" CPUQuota= MemoryMax= TasksMax= --runtime 2>/dev/null || true
    systemctl set-property "$SLICE" CPUQuota= MemoryMax= TasksMax= 2>/dev/null || true
fi

echo '{"ok":true,"data":{"method":"systemd"}}'
