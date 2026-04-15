#!/bin/bash
set -euo pipefail

# Delete a site and its LiteHttpd vhost configuration
# Usage: site-delete.sh --domain <domain> [--remove-files]

DOMAIN=""
REMOVE_FILES=false

while [[ $# -gt 0 ]]; do
    case "$1" in
        --domain)       DOMAIN="$2"; shift 2 ;;
        --remove-files) REMOVE_FILES=true; shift ;;
        *) echo '{"ok": false, "error": "unknown_arg"}' >&2; exit 1 ;;
    esac
done

if [[ -z "$DOMAIN" ]]; then
    echo '{"ok": false, "error": "missing_args", "message": "--domain is required"}' >&2
    exit 1
fi

# Validate domain format
if ! echo "$DOMAIN" | grep -qP '^(?:[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$'; then
    echo '{"ok": false, "error": "invalid_domain"}' >&2
    exit 1
fi

VHOST_DIR="/usr/local/lsws/conf/vhosts/$DOMAIN"
LSWS_CONF="/usr/local/lsws/conf/httpd_config.conf"

# 1. Remove vhost directory
if [[ -d "$VHOST_DIR" ]]; then
    rm -rf "$VHOST_DIR"
fi

# 2. Remove vhost registration from httpd_config.conf
if [[ -f "$LSWS_CONF" ]]; then
    # Remove the virtualhost block
    ESCAPED_DOMAIN=$(printf '%s\n' "$DOMAIN" | sed -e 's/[.^$*[\\/&]/\\&/g')
    sed -i "/^virtualhost $ESCAPED_DOMAIN {/,/^}/d" "$LSWS_CONF"
fi

# 3. Remove log files
rm -f "/usr/local/lsws/logs/$DOMAIN.access.log" "/usr/local/lsws/logs/$DOMAIN.error.log"

# 4. Optionally remove document root
if [[ "$REMOVE_FILES" == true ]]; then
    # Find doc_root from vhost conf backup or just by convention
    # We don't remove /home/USER entirely, only the domain directory
    for home in /home/*/public_html/"$DOMAIN"; do
        if [[ -d "$home" ]]; then
            rm -rf "$home"
        fi
    done
fi

# 5. Reload LiteHttpd
if true; then  # lswsctrl has no configtest — reload unconditionally
    /usr/local/lsws/bin/lswsctrl reload &>/dev/null || true
fi

echo '{"ok": true, "data": {"domain": "'"$DOMAIN"'", "files_removed": '"$REMOVE_FILES"'}}'
