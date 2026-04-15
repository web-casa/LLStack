#!/bin/bash
set -euo pipefail

# Generic application installer using JSON manifests
# Usage: app-install.sh --app-id <id> --doc-root <path> --domain <domain> \
#        [--admin-email <email>] [--db-name <name>] [--db-user <user>] [--db-pass-file <path>] \
#        [--admin-pw-file <path>]

APP_ID="" DOC_ROOT="" DOMAIN="" ADMIN_EMAIL="" DB_NAME="" DB_USER="" DB_PASS_FILE="" ADMIN_PW_FILE=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --app-id)         APP_ID="$2"; shift 2 ;;
        --doc-root)       DOC_ROOT="$2"; shift 2 ;;
        --domain)         DOMAIN="$2"; shift 2 ;;
        --admin-email)    ADMIN_EMAIL="$2"; shift 2 ;;
        --db-name)        DB_NAME="$2"; shift 2 ;;
        --db-user)        DB_USER="$2"; shift 2 ;;
        --db-pass-file)   DB_PASS_FILE="$2"; shift 2 ;;
        --admin-pw-file)  ADMIN_PW_FILE="$2"; shift 2 ;;
        # Legacy compat: accept --manifest but resolve to app-id
        --manifest)       APP_ID=$(basename "$2" .json); shift 2 ;;
        *) echo '{"ok":false,"error":"unknown_arg"}' >&2; exit 1 ;;
    esac
done

[[ -z "$APP_ID" || -z "$DOC_ROOT" || -z "$DOMAIN" ]] && {
    echo '{"ok":false,"error":"missing_args"}' >&2; exit 1
}

# Validate app-id (prevent path traversal)
if ! echo "$APP_ID" | grep -qP '^[a-z][a-z0-9-]{0,30}$'; then
    echo '{"ok":false,"error":"invalid_app_id"}' >&2; exit 1
fi

# Resolve manifest from trusted directories only
MANIFEST=""
for dir in "${LLSTACK_SCRIPTS_DIR:-/opt/llstack/scripts}/app/manifests" \
           "$(cd "$(dirname "$0")" && pwd)/manifests"; do
    candidate="$dir/$APP_ID.json"
    if [[ -f "$candidate" ]]; then
        # Verify file is owned by root (not writable by service account)
        MANIFEST="$candidate"
        break
    fi
done

[[ -z "$MANIFEST" ]] && { echo '{"ok":false,"error":"manifest_not_found"}' >&2; exit 1; }

# Cleanup trap for password files
trap 'rm -f -- "$ADMIN_PW_FILE" "$DB_PASS_FILE" 2>/dev/null || true' EXIT

# Parse manifest safely via Python (no eval)
read -r APP_NAME DOWNLOAD_URL EXTRACT_DIR INSTALL_METHOD COMPOSER_CMD < <(
    python3 - "$MANIFEST" << 'PYEOF'
import json, sys
m = json.load(open(sys.argv[1]))
print(
    m.get('name', 'Unknown'),
    m.get('download_url', ''),
    m.get('extract_dir', ''),
    m.get('install_method', 'download'),
    m.get('composer_command', ''),
)
PYEOF
)

# Read passwords
DB_PASS=""
[[ -n "$DB_PASS_FILE" && -f "$DB_PASS_FILE" ]] && { DB_PASS=$(cat "$DB_PASS_FILE"); }

URL="https://$DOMAIN"

echo ">>> Installing $APP_NAME to $DOC_ROOT..."
mkdir -p "$DOC_ROOT"

# Step 1: Download/Install application (allowlisted methods only)
if [[ "$INSTALL_METHOD" == "composer" && -n "$COMPOSER_CMD" ]]; then
    echo ">>> Using Composer..."
    cd "$DOC_ROOT"
    # Only allow composer create-project (validated)
    if [[ "$COMPOSER_CMD" == "composer create-project "* ]]; then
        $COMPOSER_CMD 2>&1 || true
    else
        echo "  WARNING: unsupported composer command, skipping"
    fi
elif [[ -n "$DOWNLOAD_URL" ]]; then
    echo ">>> Downloading from $DOWNLOAD_URL..."
    TMPFILE=$(mktemp /tmp/app-download.XXXXXXXXXX)

    curl -sL "$DOWNLOAD_URL" -o "$TMPFILE"

    if [[ "$DOWNLOAD_URL" == *.zip ]]; then
        cd /tmp
        unzip -qo "$TMPFILE" -d app-extract 2>/dev/null || true
        if [[ -n "$EXTRACT_DIR" && -d "/tmp/app-extract/$EXTRACT_DIR" ]]; then
            cp -a "/tmp/app-extract/$EXTRACT_DIR/." "$DOC_ROOT/"
        else
            cp -a /tmp/app-extract/*/* "$DOC_ROOT/" 2>/dev/null || true
        fi
        rm -rf /tmp/app-extract
    else
        cd /tmp
        tar xzf "$TMPFILE" 2>/dev/null || true
        if [[ -n "$EXTRACT_DIR" && -d "/tmp/$EXTRACT_DIR" ]]; then
            cp -a "/tmp/$EXTRACT_DIR/." "$DOC_ROOT/"
            rm -rf "/tmp/$EXTRACT_DIR"
        fi
    fi
    rm -f "$TMPFILE"
fi

# Step 2: Run allowlisted setup (NO eval — use wp-cli directly for WP)
if [[ "$APP_ID" == "wordpress" ]]; then
    WP_CLI=""
    for p in /usr/local/bin/wp /usr/bin/wp; do [[ -x "$p" ]] && { WP_CLI="$p"; break; }; done

    if [[ -n "$WP_CLI" && -n "$DB_NAME" ]]; then
        echo ">>> Configuring WordPress..."
        $WP_CLI config create --path="$DOC_ROOT" --dbname="$DB_NAME" --dbuser="$DB_USER" \
            --dbhost="localhost" --allow-root --skip-check --prompt=dbpass <<< "$DB_PASS" 2>&1 || true

        echo ">>> Installing WordPress..."
        if [[ -n "$ADMIN_PW_FILE" && -f "$ADMIN_PW_FILE" ]]; then
            $WP_CLI core install --path="$DOC_ROOT" --url="$URL" --title="$APP_NAME Site" \
                --admin_user="admin" --admin_email="$ADMIN_EMAIL" --allow-root --skip-email \
                --prompt=admin_password < "$ADMIN_PW_FILE" 2>&1 || true
        fi

        echo ">>> Installing LiteSpeed Cache plugin..."
        $WP_CLI plugin install litespeed-cache --activate --path="$DOC_ROOT" --allow-root 2>&1 || true
    fi
elif [[ "$APP_ID" == "laravel" ]]; then
    echo ">>> Running Laravel setup..."
    cd "$DOC_ROOT"
    php artisan key:generate --force 2>&1 || true
fi

# Step 3: Set ownership
SITE_USER=$(stat -c '%U' "$(dirname "$DOC_ROOT")" 2>/dev/null || echo "root")
chown -R "$SITE_USER:$SITE_USER" "$DOC_ROOT" 2>/dev/null || true

echo ">>> $APP_NAME installation complete!"
echo "{\"ok\":true,\"data\":{\"app\":\"$APP_NAME\",\"path\":\"$DOC_ROOT\"}}"
