#!/bin/bash
set -euo pipefail

# Upgrade a service to latest minor version
# Usage: service-upgrade.sh --service <mariadb|postgresql|redis> --action <check|upgrade>

SERVICE="" ACTION=""
while [[ $# -gt 0 ]]; do
    case "$1" in
        --service) SERVICE="$2"; shift 2 ;;
        --action)  ACTION="$2"; shift 2 ;;
        *) echo '{"ok":false,"error":"unknown_arg"}' >&2; exit 1 ;;
    esac
done

[[ -z "$SERVICE" || -z "$ACTION" ]] && { echo '{"ok":false,"error":"missing_args"}' >&2; exit 1; }

case "$SERVICE" in
    mariadb) PKG="MariaDB-server" SVC="mariadb" ;;
    postgresql) PKG="postgresql-server" SVC="postgresql" ;;
    redis)
        # Detect Redis or Valkey
        if rpm -q valkey &>/dev/null; then
            PKG="valkey" SVC="valkey"
        else
            PKG="redis" SVC="redis"
        fi
        ;;
    *) echo '{"ok":false,"error":"unsupported_service"}' >&2; exit 1 ;;
esac

# Get current version
CURRENT=$(rpm -q "$PKG" 2>/dev/null | head -1 || echo "not_installed")

case "$ACTION" in
    check)
        # Check for available updates
        UPDATES=$(dnf check-update "$PKG" 2>/dev/null | grep -E "^$PKG" | awk '{print $2}' || echo "")
        if [[ -n "$UPDATES" ]]; then
            echo "{\"ok\":true,\"data\":{\"service\":\"$SERVICE\",\"current\":\"$CURRENT\",\"available\":\"$UPDATES\",\"update_available\":true}}"
        else
            echo "{\"ok\":true,\"data\":{\"service\":\"$SERVICE\",\"current\":\"$CURRENT\",\"update_available\":false}}"
        fi
        ;;
    upgrade)
        echo ">>> Upgrading $SERVICE..."
        echo ">>> Current: $CURRENT"

        # Record dnf transaction ID for rollback
        BEFORE_TID=$(dnf history list 2>/dev/null | head -3 | tail -1 | awk '{print $1}')

        # Perform upgrade
        dnf update -y "$PKG" 2>&1

        # Restart service
        echo ">>> Restarting $SERVICE..."
        systemctl restart "$SVC" 2>/dev/null || true
        sleep 2

        # Verify
        AFTER=$(rpm -q "$PKG" 2>/dev/null | head -1 || echo "unknown")
        STATUS=$(systemctl is-active "$SVC" 2>/dev/null || echo "unknown")
        AFTER_TID=$(dnf history list 2>/dev/null | head -3 | tail -1 | awk '{print $1}')

        echo ">>> Upgrade complete: $CURRENT → $AFTER (status: $STATUS)"
        echo "{\"ok\":true,\"data\":{\"service\":\"$SERVICE\",\"previous\":\"$CURRENT\",\"current\":\"$AFTER\",\"status\":\"$STATUS\",\"rollback_tid\":\"$AFTER_TID\"}}"
        ;;
    *)
        echo '{"ok":false,"error":"invalid_action"}' >&2; exit 1
        ;;
esac
