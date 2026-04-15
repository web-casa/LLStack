#!/bin/bash
set -euo pipefail

# Purge LSCache for a site
# Usage: site-cache-purge.sh --domain <domain>

DOMAIN=""
while [[ $# -gt 0 ]]; do
    case "$1" in
        --domain) DOMAIN="$2"; shift 2 ;;
        *) echo '{"ok":false,"error":"unknown_arg"}' >&2; exit 1 ;;
    esac
done

[[ -z "$DOMAIN" ]] && { echo '{"ok":false,"error":"missing_domain"}' >&2; exit 1; }

# Validate domain format
if ! echo "$DOMAIN" | grep -qP '^[a-zA-Z0-9._-]+$'; then
    echo '{"ok":false,"error":"invalid_domain"}' >&2; exit 1
fi

CACHE_DIR="/usr/local/lsws/cachedata/$DOMAIN"

if [[ -d "$CACHE_DIR" ]]; then
    rm -rf "$CACHE_DIR"
    mkdir -p "$CACHE_DIR"
    echo '{"ok":true,"data":{"purged":true}}'
else
    echo '{"ok":true,"data":{"purged":false,"message":"no_cache_dir"}}'
fi
