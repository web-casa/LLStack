#!/bin/bash
set -euo pipefail
VERSION="" KEY="" VALUE=""
while [[ $# -gt 0 ]]; do case "$1" in --version) VERSION="$2"; shift 2 ;; --key) KEY="$2"; shift 2 ;; --value) VALUE="$2"; shift 2 ;; *) shift ;; esac; done
[[ -z "$VERSION" || -z "$KEY" ]] && { echo '{"ok":false,"error":"missing_args"}' >&2; exit 1; }
INI="/etc/opt/remi/php${VERSION}/php.ini"
[[ ! -f "$INI" ]] && { echo '{"ok":false,"error":"ini_not_found"}' >&2; exit 1; }
if grep -q "^${KEY}\s*=" "$INI"; then
    sed -i "s|^${KEY}\s*=.*|${KEY} = ${VALUE}|" "$INI"
else
    echo "${KEY} = ${VALUE}" >> "$INI"
fi
echo "{\"ok\":true,\"data\":{\"key\":\"$KEY\",\"value\":\"$VALUE\"}}"
