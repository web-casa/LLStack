#!/bin/bash
set -euo pipefail

# Get status of a user's Redis instance
# Usage: REDIS_PASSWORD=xxx redis-instance-status.sh --user <system_user>

USER=""
PASSWORD="${REDIS_PASSWORD:-}"

while [[ $# -gt 0 ]]; do
    case "$1" in
        --user)     USER="$2"; shift 2 ;;
        --password) PASSWORD="$2"; shift 2 ;;  # legacy fallback
        *) echo '{"ok": false, "error": "unknown_arg"}' >&2; exit 1 ;;
    esac
done

if [[ -z "$USER" ]]; then
    echo '{"ok": false, "error": "missing_args"}' >&2
    exit 1
fi

SOCKET="/home/$USER/.redis/redis.sock"
SERVICE="redis@$USER"

# Check if running
if ! systemctl is-active "$SERVICE" &>/dev/null; then
    echo '{"ok": true, "data": {"status": "stopped", "memory_used_bytes": 0, "memory_peak_bytes": 0, "connected_clients": 0, "hit_rate": 0, "uptime_seconds": 0}}'
    exit 0
fi

# Get INFO from Redis (password via REDISCLI_AUTH env to avoid /proc exposure)
export REDISCLI_AUTH="${PASSWORD}"

INFO=$(redis-cli -s "$SOCKET" --no-auth-warning INFO 2>/dev/null || echo "")

if [[ -z "$INFO" ]]; then
    echo '{"ok": true, "data": {"status": "running", "memory_used_bytes": 0, "memory_peak_bytes": 0, "connected_clients": 0, "hit_rate": 0, "uptime_seconds": 0}}'
    exit 0
fi

MEM_USED=$(echo "$INFO" | grep "^used_memory:" | cut -d: -f2 | tr -d '\r' || echo "0")
MEM_PEAK=$(echo "$INFO" | grep "^used_memory_peak:" | cut -d: -f2 | tr -d '\r' || echo "0")
CLIENTS=$(echo "$INFO" | grep "^connected_clients:" | cut -d: -f2 | tr -d '\r' || echo "0")
UPTIME=$(echo "$INFO" | grep "^uptime_in_seconds:" | cut -d: -f2 | tr -d '\r' || echo "0")

HITS=$(echo "$INFO" | grep "^keyspace_hits:" | cut -d: -f2 | tr -d '\r' || echo "0")
MISSES=$(echo "$INFO" | grep "^keyspace_misses:" | cut -d: -f2 | tr -d '\r' || echo "0")
TOTAL=$((HITS + MISSES))
if [[ "$TOTAL" -gt 0 ]]; then
    HIT_RATE=$(awk "BEGIN{printf \"%.1f\", $HITS * 100.0 / $TOTAL}")
else
    HIT_RATE="0.0"
fi

cat << EOF
{"ok": true, "data": {"status": "running", "memory_used_bytes": $MEM_USED, "memory_peak_bytes": $MEM_PEAK, "connected_clients": $CLIENTS, "hit_rate": $HIT_RATE, "uptime_seconds": $UPTIME}}
EOF
