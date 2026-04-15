#!/bin/bash
set -euo pipefail
DOMAIN="" DOC_ROOT="" PHP_VERSION="php83"
while [[ $# -gt 0 ]]; do case "$1" in --domain) DOMAIN="$2"; shift 2 ;; --doc-root) DOC_ROOT="$2"; shift 2 ;; --php) PHP_VERSION="$2"; shift 2 ;; *) shift ;; esac; done
[[ -z "$DOMAIN" || -z "$DOC_ROOT" ]] && { echo '{"ok":false,"error":"missing_args"}' >&2; exit 1; }
cd /tmp
curl -sL "https://github.com/typecho/typecho/releases/latest/download/typecho.zip" -o typecho.zip
unzip -qo typecho.zip -d typecho_tmp
cp -r typecho_tmp/* "$DOC_ROOT/" 2>/dev/null || cp -r typecho_tmp/build/* "$DOC_ROOT/" 2>/dev/null || true
SITE_USER=$(stat -c '%U' "$DOC_ROOT")
chown -R "$SITE_USER:$SITE_USER" "$DOC_ROOT"
rm -rf /tmp/typecho.zip /tmp/typecho_tmp
echo "{\"ok\":true,\"data\":{\"app\":\"typecho\",\"domain\":\"$DOMAIN\"}}"
