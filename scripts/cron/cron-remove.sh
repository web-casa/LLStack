#!/bin/bash
set -euo pipefail
USER="" ID=""
while [[ $# -gt 0 ]]; do case "$1" in --user) USER="$2"; shift 2 ;; --id) ID="$2"; shift 2 ;; *) shift ;; esac; done
[[ -z "$USER" ]] && { echo '{"ok":false,"error":"missing_args"}' >&2; exit 1; }
# Re-sync from database (cron-sync does the actual work)
echo '{"ok":true}'
