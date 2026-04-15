#!/bin/bash
set -euo pipefail

# Output system info as JSON for the dashboard
# Usage: sysinfo.sh

CPU_CORES=$(nproc)
CPU_MODEL=$(grep -m1 'model name' /proc/cpuinfo 2>/dev/null | cut -d: -f2 | xargs || echo "Unknown")

# CPU usage (1-second sample)
CPU_USAGE=$(top -bn1 | grep '^%Cpu' | awk '{print 100.0 - $8}' 2>/dev/null || echo "0")

# Memory
read -r MEM_TOTAL MEM_AVAIL <<< "$(awk '/MemTotal/{t=$2} /MemAvailable/{a=$2} END{print t*1024, a*1024}' /proc/meminfo)"
MEM_USED=$((MEM_TOTAL - MEM_AVAIL))
if [ "$MEM_TOTAL" -gt 0 ]; then
    MEM_PCT=$(awk "BEGIN{printf \"%.1f\", $MEM_USED * 100.0 / $MEM_TOTAL}")
else
    MEM_PCT="0.0"
fi

# Disk
read -r DISK_TOTAL DISK_USED DISK_PCT <<< "$(df -B1 / | awk 'NR==2{print $2, $3, $5}' | tr -d '%')"

# Load
LOAD=$(cat /proc/loadavg | awk '{printf "[%.2f, %.2f, %.2f]", $1, $2, $3}')

# Uptime
UPTIME_SEC=$(awk '{print int($1)}' /proc/uptime)

# OS info
OS_NAME=$(. /etc/os-release 2>/dev/null && echo "$NAME" || echo "Linux")
OS_VERSION=$(. /etc/os-release 2>/dev/null && echo "$VERSION_ID" || echo "")
KERNEL=$(uname -r)
ARCH=$(uname -m)

cat <<EOF
{
  "ok": true,
  "data": {
    "cpu": {"cores": $CPU_CORES, "usage_percent": $CPU_USAGE, "model": "$CPU_MODEL"},
    "memory": {"total_bytes": $MEM_TOTAL, "used_bytes": $MEM_USED, "usage_percent": $MEM_PCT},
    "disk": {"total_bytes": $DISK_TOTAL, "used_bytes": $DISK_USED, "usage_percent": $DISK_PCT, "mount_point": "/"},
    "load": $LOAD,
    "uptime_seconds": $UPTIME_SEC,
    "os": {"name": "$OS_NAME", "version": "$OS_VERSION", "kernel": "$KERNEL", "arch": "$ARCH"}
  }
}
EOF
