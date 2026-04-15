#!/bin/bash
set -euo pipefail
VERSION=""
while [[ $# -gt 0 ]]; do case "$1" in --version) VERSION="$2"; shift 2 ;; *) shift ;; esac; done
[[ -z "$VERSION" ]] && { echo '{"ok":false,"error":"missing_args"}' >&2; exit 1; }
dnf remove -y "php${VERSION}-php-*" 2>&1 | tail -1
# Remove extprocessor from httpd_config.conf
LSWS_CONF="/usr/local/lsws/conf/httpd_config.conf"
if [[ -f "$LSWS_CONF" ]]; then
    sed -i "/^extprocessor lsphp${VERSION} {/,/^}/d" "$LSWS_CONF"
fi
/usr/local/lsws/bin/lswsctrl reload 2>/dev/null || true
echo "{\"ok\":true,\"data\":{\"version\":\"php${VERSION}\"}}"
