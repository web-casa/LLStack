#!/bin/bash
set -euo pipefail

# Check cgroups v2 support and OLS lscgctl availability
# Usage: cgroup-check.sh
# Returns JSON with support status for each feature

LSCGCTL_PATH="/usr/local/lsws/lsns/bin/lscgctl"
LSSETUP_PATH="/usr/local/lsws/lsns/bin/lssetup"

cgroups_v2=false
lscgctl_available=false
memory_controller=false
cpu_controller=false
io_controller=false
os_name="Unknown"
needs_enablement=false

# Detect OS
if [[ -f /etc/redhat-release ]]; then
    os_name=$(cat /etc/redhat-release)
fi

# Check cgroups v2 mounted
if [[ -f /sys/fs/cgroup/cgroup.controllers ]]; then
    cgroups_v2=true
    controllers=$(cat /sys/fs/cgroup/cgroup.controllers)
    [[ "$controllers" == *"memory"* ]] && memory_controller=true
    [[ "$controllers" == *"cpu"* ]] && cpu_controller=true
    [[ "$controllers" == *"io"* ]] && io_controller=true
else
    # Check if RHEL 8 needs manual enablement
    if [[ -f /etc/redhat-release ]] && grep -qi 'release 8' /etc/redhat-release; then
        needs_enablement=true
    fi
fi

# Check lscgctl
if [[ -x "$LSCGCTL_PATH" ]]; then
    if "$LSCGCTL_PATH" version &>/dev/null; then
        lscgctl_available=true
    fi
fi

cat << EOF
{"ok": true, "data": {
  "cgroups_v2": $cgroups_v2,
  "lscgctl_available": $lscgctl_available,
  "memory_controller": $memory_controller,
  "cpu_controller": $cpu_controller,
  "io_controller": $io_controller,
  "os_name": "$os_name",
  "needs_enablement": $needs_enablement,
  "lscgctl_path": "$LSCGCTL_PATH",
  "lssetup_path": "$LSSETUP_PATH"
}}
EOF
