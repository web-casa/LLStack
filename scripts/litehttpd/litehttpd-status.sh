#!/bin/bash
set -euo pipefail
STATUS=$(systemctl is-active lshttpd 2>/dev/null || echo "not_installed")
PID=$(pgrep -f openlitespeed 2>/dev/null | head -1 || echo "0")
CONNS=$(ss -tnp 2>/dev/null | grep -c ":80 \|:443 " || echo "0")
echo "{\"ok\":true,\"data\":{\"status\":\"$STATUS\",\"pid\":$PID,\"connections\":$CONNS}}"
