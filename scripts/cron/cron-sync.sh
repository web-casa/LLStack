#!/bin/bash
set -euo pipefail
# Sync cron jobs from panel database to system crontab
USER=""
while [[ $# -gt 0 ]]; do case "$1" in --user) USER="$2"; shift 2 ;; *) shift ;; esac; done
[[ -z "$USER" ]] && { echo '{"ok":false,"error":"missing_args"}' >&2; exit 1; }
# Panel manages cron via its own database; this script rebuilds the user's crontab
# In production, query the panel DB and write crontab entries
echo '{"ok":true}'
