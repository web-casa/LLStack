#!/bin/bash
set -euo pipefail

# Set cgroup v2 resource limits for a user
# Usage: cgroup-set.sh --user <system_user> --cpu <percent> --mem <MB> --io <MB/s> --tasks <max>
# Uses lscgctl if available, falls back to systemctl set-property

USER=""
CPU=0      # 0=unlimited, 100=1 core
MEM=0      # 0=unlimited, in MB
IO=0       # 0=unlimited, in MB/s
TASKS=0    # 0=unlimited

while [[ $# -gt 0 ]]; do
    case "$1" in
        --user)  USER="$2"; shift 2 ;;
        --cpu)   CPU="$2"; shift 2 ;;
        --mem)   MEM="$2"; shift 2 ;;
        --io)    IO="$2"; shift 2 ;;
        --tasks) TASKS="$2"; shift 2 ;;
        *) echo '{"ok":false,"error":"unknown_arg"}' >&2; exit 1 ;;
    esac
done

[[ -z "$USER" ]] && { echo '{"ok":false,"error":"missing_user"}' >&2; exit 1; }

# Validate user exists
if ! id "$USER" &>/dev/null; then
    echo '{"ok":false,"error":"user_not_found"}' >&2; exit 1
fi

# Per-user lock to prevent concurrent limit changes
LOCK_FILE="/var/lock/llstack-cgroup-${USER}.lock"
exec 201>"$LOCK_FILE"
flock -w 5 201 || { echo '{"ok":false,"error":"cgroup_locked"}' >&2; exit 1; }

# Check cgroups v2 available
if [[ ! -f /sys/fs/cgroup/cgroup.controllers ]]; then
    echo '{"ok":false,"error":"cgroups_v2_not_available","message":"cgroups v2 not mounted. For EL8: grubby --update-kernel=ALL --args=systemd.unified_cgroup_hierarchy=1 && reboot"}' >&2
    exit 1
fi

LSCGCTL="/usr/local/lsws/lsns/bin/lscgctl"
UID_NUM=$(id -u "$USER")

# Try lscgctl first (OLS native, most reliable)
if [[ -x "$LSCGCTL" ]] && "$LSCGCTL" version &>/dev/null; then
    CMD=("$LSCGCTL" set "$USER")
    [[ "$CPU" -gt 0 ]] && CMD+=(--cpu "$CPU")
    [[ "$MEM" -gt 0 ]] && CMD+=(--mem "${MEM}M")
    [[ "$IO" -gt 0 ]] && CMD+=(--io "$((IO * 1024 * 1024))")
    [[ "$TASKS" -gt 0 ]] && CMD+=(--tasks "$TASKS")

    if "${CMD[@]}" 2>/dev/null; then
        echo "{\"ok\":true,\"data\":{\"method\":\"lscgctl\",\"user\":\"$USER\",\"cpu\":$CPU,\"mem\":$MEM,\"io\":$IO,\"tasks\":$TASKS}}"
        exit 0
    fi
fi

# Fallback: systemctl set-property on user slice (DA pattern)
SLICE="user-${UID_NUM}.slice"
PROPS=()

if [[ "$CPU" -gt 0 ]]; then
    PROPS+=(CPUQuota="${CPU}%")
fi
if [[ "$MEM" -gt 0 ]]; then
    PROPS+=(MemoryMax="${MEM}M")
fi
if [[ "$TASKS" -gt 0 ]]; then
    PROPS+=(TasksMax="$TASKS")
fi
# IO limits via systemd require specifying block device, skip for now
# (lscgctl handles this better)

if [[ ${#PROPS[@]} -eq 0 ]]; then
    echo '{"ok":true,"data":{"method":"none","message":"no limits to set"}}'
    exit 0
fi

PROP_ARGS=()
for p in "${PROPS[@]}"; do
    PROP_ARGS+=("--property=$p")
done

IO_NOTE=""
if [[ "$IO" -gt 0 ]]; then
    IO_NOTE=",\"io_warning\":\"IO limits require lscgctl, not applied via systemd fallback\""
fi

if systemctl set-property "$SLICE" "${PROP_ARGS[@]}" --runtime 2>/dev/null; then
    # Also persist for next boot
    PERSIST_OK=true
    if ! systemctl set-property "$SLICE" "${PROP_ARGS[@]}" 2>/dev/null; then
        PERSIST_OK=false
    fi
    echo "{\"ok\":true,\"data\":{\"method\":\"systemd\",\"user\":\"$USER\",\"cpu\":$CPU,\"mem\":$MEM,\"io\":0,\"tasks\":$TASKS,\"persistent\":$PERSIST_OK${IO_NOTE}}}"
else
    echo '{"ok":false,"error":"set_property_failed"}' >&2; exit 1
fi
