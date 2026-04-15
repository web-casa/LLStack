#!/bin/bash
set -euo pipefail

# Push staging site to production (overwrite production with staging content)
# Usage: site-staging-push.sh --staging-domain <domain> --prod-domain <domain> \
#        --mode <all|files|database> [--staging-db <db>] [--prod-db <db>]

STAGING_DOMAIN="" PROD_DOMAIN="" MODE="all"
STAGING_DB="" PROD_DB=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --staging-domain) STAGING_DOMAIN="$2"; shift 2 ;;
        --prod-domain)    PROD_DOMAIN="$2"; shift 2 ;;
        --mode)           MODE="$2"; shift 2 ;;
        --staging-db)     STAGING_DB="$2"; shift 2 ;;
        --prod-db)        PROD_DB="$2"; shift 2 ;;
        *) shift ;;
    esac
done

if [[ -z "$STAGING_DOMAIN" || -z "$PROD_DOMAIN" ]]; then
    echo '{"ok":false,"error":"missing_args"}' >&2; exit 1
fi

# Validate domains
validate_domain() {
    echo "$1" | grep -qP '^(?:[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$'
}
if ! validate_domain "$STAGING_DOMAIN" || ! validate_domain "$PROD_DOMAIN"; then
    echo '{"ok":false,"error":"invalid_domain"}' >&2; exit 1
fi

if [[ "$MODE" != "all" && "$MODE" != "files" && "$MODE" != "database" ]]; then
    echo '{"ok":false,"error":"invalid_mode","message":"mode must be all, files, or database"}' >&2; exit 1
fi

# Resolve paths
STAGING_VHOST="/usr/local/lsws/conf/vhosts/$STAGING_DOMAIN"
PROD_VHOST="/usr/local/lsws/conf/vhosts/$PROD_DOMAIN"

# Find doc_roots from vhost configs
_get_docroot() {
    grep -oP 'docRoot\s+\K\S+' "$1/vhconf.conf" 2>/dev/null || echo ""
}
STAGING_ROOT=$(_get_docroot "$STAGING_VHOST")
PROD_ROOT=$(_get_docroot "$PROD_VHOST")

if [[ -z "$STAGING_ROOT" || -z "$PROD_ROOT" ]]; then
    echo '{"ok":false,"error":"docroot_not_found"}' >&2; exit 1
fi

# Step 1: Backup production before push
echo ">>> Step 1: Backing up production..."
BACKUP_DIR="/opt/llstack/backups/staging-push-$(date +%Y%m%d%H%M%S)"
mkdir -p "$BACKUP_DIR"

if [[ "$MODE" == "all" || "$MODE" == "files" ]]; then
    if ! cp -a "$PROD_ROOT" "$BACKUP_DIR/files" 2>/dev/null; then
        echo '{"ok":false,"error":"backup_failed","message":"Failed to backup production files"}' >&2
        exit 1
    fi
    echo "    Files backed up to $BACKUP_DIR/files"
fi

if [[ "$MODE" == "all" || "$MODE" == "database" ]]; then
    if [[ -n "$PROD_DB" ]]; then
        if ! mysqldump "$PROD_DB" > "$BACKUP_DIR/db.sql" 2>/dev/null; then
            echo '{"ok":false,"error":"backup_failed","message":"Failed to backup production database"}' >&2
            exit 1
        fi
        echo "    Database backed up to $BACKUP_DIR/db.sql"
    fi
fi

# Step 2: Push files
if [[ "$MODE" == "all" || "$MODE" == "files" ]]; then
    echo ">>> Step 2: Pushing files..."
    rsync -a --delete \
        --exclude='.git' \
        --exclude='wp-config.php' \
        --exclude='.env' \
        --exclude='.user.ini' \
        --exclude='wp-content/debug.log' \
        "$STAGING_ROOT/" "$PROD_ROOT/"

    # Fix ownership (production site owner)
    PROD_OWNER=$(stat -c '%U' "$PROD_ROOT" 2>/dev/null || echo "root")
    chown -R "$PROD_OWNER:$PROD_OWNER" "$PROD_ROOT"
    echo "    Files synced: $STAGING_ROOT → $PROD_ROOT"
fi

# Step 3: Push database
if [[ "$MODE" == "all" || "$MODE" == "database" ]]; then
    if [[ -n "$STAGING_DB" && -n "$PROD_DB" ]]; then
        echo ">>> Step 3: Pushing database..."
        # Dump staging DB and import to production
        mysqldump "$STAGING_DB" 2>/dev/null | mysql "$PROD_DB" 2>/dev/null

        # Replace staging domain with production domain in WordPress
        if command -v wp &>/dev/null && [[ -f "$PROD_ROOT/wp-config.php" ]]; then
            cd "$PROD_ROOT"
            wp search-replace "https://$STAGING_DOMAIN" "https://$PROD_DOMAIN" --all-tables --skip-columns=guid --allow-root 2>&1 || true
            wp search-replace "http://$STAGING_DOMAIN" "http://$PROD_DOMAIN" --all-tables --skip-columns=guid --allow-root 2>&1 || true
            echo "    WP domain replaced: $STAGING_DOMAIN → $PROD_DOMAIN"
        else
            mysql "$PROD_DB" -e "
                UPDATE wp_options SET option_value = REPLACE(option_value, '$STAGING_DOMAIN', '$PROD_DOMAIN')
                WHERE option_name IN ('siteurl', 'home');
            " 2>/dev/null || true
            echo "    wp_options updated (WP-CLI not available for full replace)"
        fi
        echo "    Database pushed: $STAGING_DB → $PROD_DB"
    else
        echo ">>> Step 3: Skipped (no database specified)"
    fi
fi

# Step 4: Reload LiteHttpd (only if files were changed)
if [[ "$MODE" == "all" || "$MODE" == "files" ]]; then
    echo ">>> Step 4: Reloading LiteHttpd..."
    /usr/local/lsws/bin/lswsctrl reload &>/dev/null || true
fi

echo '{"ok":true,"backup_dir":"'"$BACKUP_DIR"'"}'
