#!/bin/bash
set -euo pipefail

# Export panel configuration for migration to another server
# Usage: panel-export.sh --output <path>
# Exports: database, vhost configs, .htaccess files, SSL certs, panel config

OUTPUT=""
while [[ $# -gt 0 ]]; do case "$1" in --output) OUTPUT="$2"; shift 2 ;; *) shift ;; esac; done

LLSTACK_DIR="${LLSTACK_DIR:-/opt/llstack}"
DB_PATH="${LLSTACK_DB_PATH:-$LLSTACK_DIR/data/llstack.db}"

if [[ -z "$OUTPUT" ]]; then
    OUTPUT="/tmp/llstack-export-$(date +%Y%m%d%H%M%S).tar.gz"
fi

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

echo ">>> Exporting LLStack panel configuration..."

# 1. Panel database
echo "  Database..."
mkdir -p "$TMPDIR/data"
cp "$DB_PATH" "$TMPDIR/data/llstack.db" 2>/dev/null || true
cp "$LLSTACK_DIR/data/.llstack_"* "$TMPDIR/data/" 2>/dev/null || true

# 2. Vhost configurations
echo "  Vhost configs..."
mkdir -p "$TMPDIR/vhosts"
for vdir in /usr/local/lsws/conf/vhosts/*/; do
    [[ -d "$vdir" ]] || continue
    domain=$(basename "$vdir")
    mkdir -p "$TMPDIR/vhosts/$domain"
    cp "$vdir/vhconf.conf" "$TMPDIR/vhosts/$domain/" 2>/dev/null || true
done

# 3. SSL certificates
echo "  SSL certificates..."
mkdir -p "$TMPDIR/ssl"
for sdir in /usr/local/lsws/conf/ssl/*/; do
    [[ -d "$sdir" ]] || continue
    domain=$(basename "$sdir")
    mkdir -p "$TMPDIR/ssl/$domain"
    cp "$sdir"/*.pem "$TMPDIR/ssl/$domain/" 2>/dev/null || true
done

# 4. httpd_config.conf (LiteHttpd main config)
echo "  LiteHttpd config..."
cp /usr/local/lsws/conf/httpd_config.conf "$TMPDIR/httpd_config.conf" 2>/dev/null || true

# 5. .htaccess files from all sites
echo "  .htaccess files..."
mkdir -p "$TMPDIR/htaccess"
for htaccess in /home/*/public_html/*/.htaccess; do
    [[ -f "$htaccess" ]] || continue
    domain=$(basename "$(dirname "$htaccess")")
    cp "$htaccess" "$TMPDIR/htaccess/$domain.htaccess" 2>/dev/null || true
done

# 6. PHP configs
echo "  PHP configs..."
mkdir -p "$TMPDIR/php"
for ini in /etc/opt/remi/php*/php.ini; do
    [[ -f "$ini" ]] || continue
    ver=$(echo "$ini" | grep -oP 'php\K[0-9]+')
    cp "$ini" "$TMPDIR/php/php${ver}.ini" 2>/dev/null || true
done

# 7. Panel config (panel_config table entries)
echo "  Panel config..."

# 8. Metadata
cat > "$TMPDIR/export-info.json" << INFOEOF
{
    "exported_at": "$(date -Iseconds)",
    "hostname": "$(hostname)",
    "os": "$(. /etc/os-release; echo "$NAME $VERSION_ID")",
    "panel_version": "0.1.0"
}
INFOEOF

# Package
echo ">>> Creating archive..."
cd "$TMPDIR"
tar czf "$OUTPUT" ./*

SIZE=$(stat -c%s "$OUTPUT" 2>/dev/null || echo 0)
echo ">>> Export complete: $OUTPUT ($((SIZE / 1024)) KB)"
echo "{\"ok\":true,\"data\":{\"path\":\"$OUTPUT\",\"size\":$SIZE}}"
