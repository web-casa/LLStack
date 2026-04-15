#!/bin/bash
set -euo pipefail

# Parse OLS bytes logs and output bandwidth usage per domain
# Usage: bandwidth-collect.sh
# Reads $SERVER_ROOT/logs/*.bytes files, outputs JSON

LOG_DIR="/usr/local/lsws/logs"
MONTH=$(date +%Y-%m)

echo '['
FIRST=true

for bytes_log in "$LOG_DIR"/*.bytes; do
    [[ ! -f "$bytes_log" ]] && continue
    [[ ! -s "$bytes_log" ]] && continue

    # Extract domain from filename (e.g., example.com.bytes)
    BASENAME=$(basename "$bytes_log")
    DOMAIN="${BASENAME%.bytes}"
    [[ -z "$DOMAIN" ]] && continue

    # Sum bytes: format is "%O %I" per line (output input)
    TOTALS=$(awk '{out += $1; inp += $2} END {printf "%d %d", out, inp}' "$bytes_log" 2>/dev/null || echo "0 0")
    BYTES_OUT=$(echo "$TOTALS" | awk '{print $1}')
    BYTES_IN=$(echo "$TOTALS" | awk '{print $2}')

    [[ "$FIRST" == true ]] && FIRST=false || echo ','
    echo "  {\"domain\": \"$DOMAIN\", \"month\": \"$MONTH\", \"bytes_out\": $BYTES_OUT, \"bytes_in\": $BYTES_IN}"
done

echo ']'
