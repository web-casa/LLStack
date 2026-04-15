#!/bin/bash
set -euo pipefail

# Output status of all managed services as JSON
# Usage: service-status.sh

check_service() {
    local name="$1"
    local display="$2"
    local unit="$3"

    if ! systemctl list-unit-files "$unit.service" &>/dev/null; then
        return
    fi

    local status
    if systemctl is-active "$unit" &>/dev/null; then
        status="running"
    elif systemctl is-enabled "$unit" &>/dev/null; then
        status="stopped"
    else
        status="disabled"
    fi

    local enabled="false"
    systemctl is-enabled "$unit" &>/dev/null && enabled="true"

    echo "    {\"name\": \"$name\", \"display\": \"$display\", \"status\": \"$status\", \"enabled\": $enabled}"
}

echo '{"ok": true, "data": ['

FIRST=true
for entry in \
    "litehttpd|LiteHttpd|lshttpd" \
    "mariadb|MariaDB|mariadb" \
    "mysql|MySQL|mysqld" \
    "postgresql|PostgreSQL|postgresql" \
    "redis|Redis|redis" \
; do
    IFS='|' read -r name display unit <<< "$entry"
    line=$(check_service "$name" "$display" "$unit")
    if [ -n "$line" ]; then
        if [ "$FIRST" = true ]; then
            FIRST=false
        else
            echo ","
        fi
        printf '%s' "$line"
    fi
done

# Check PHP versions
for lsphp_bin in /opt/remi/php*/root/usr/bin/lsphp; do
    [ -f "$lsphp_bin" ] || continue
    ver=$(echo "$lsphp_bin" | grep -oP 'php\K[0-9]+')
    display="PHP ${ver:0:1}.${ver:1}"
    if [ "$FIRST" = true ]; then
        FIRST=false
    else
        echo ","
    fi
    echo -n "    {\"name\": \"php$ver\", \"display\": \"$display\", \"status\": \"installed\", \"enabled\": true}"
done

echo ""
echo "]}"
