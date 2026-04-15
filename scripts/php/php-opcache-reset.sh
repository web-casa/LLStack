#!/bin/bash
set -euo pipefail
VERSION=""
while [[ $# -gt 0 ]]; do case "$1" in --version) VERSION="$2"; shift 2 ;; *) shift ;; esac; done
# OPcache reset by restarting lsphp (LSAPI restarts workers)
/usr/local/lsws/bin/lswsctrl reload 2>/dev/null || true
echo '{"ok":true}'
