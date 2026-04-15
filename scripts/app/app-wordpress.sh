#!/bin/bash
set -euo pipefail

# WordPress one-click installer
# Usage: app-wordpress.sh --domain <domain> --doc-root <path> --php <php_version>

DOMAIN=""
DOC_ROOT=""
PHP_VERSION="php83"
WP_LOCALE="en_US"

while [[ $# -gt 0 ]]; do
    case "$1" in
        --domain)   DOMAIN="$2"; shift 2 ;;
        --doc-root) DOC_ROOT="$2"; shift 2 ;;
        --php)      PHP_VERSION="$2"; shift 2 ;;
        --locale)   WP_LOCALE="$2"; shift 2 ;;
        *) shift ;;
    esac
done

if [[ -z "$DOMAIN" || -z "$DOC_ROOT" ]]; then
    echo '{"ok": false, "error": "missing_args"}' >&2
    exit 1
fi

WP_URL="https://wordpress.org/latest.tar.gz"

# 1. Download WordPress
cd /tmp
curl -sL "$WP_URL" -o wordpress.tar.gz
tar xzf wordpress.tar.gz

# 2. Copy to doc_root
if [[ -f "$DOC_ROOT/index.html" ]]; then
    mv "$DOC_ROOT/index.html" "$DOC_ROOT/index.html.bak" 2>/dev/null || true
fi
cp -r wordpress/* "$DOC_ROOT/"
rm -rf /tmp/wordpress /tmp/wordpress.tar.gz

# 3. Set permissions
SITE_USER=$(stat -c '%U' "$DOC_ROOT")
chown -R "$SITE_USER:$SITE_USER" "$DOC_ROOT"
find "$DOC_ROOT" -type d -exec chmod 755 {} \;
find "$DOC_ROOT" -type f -exec chmod 644 {} \;

# 4. Create wp-config.php template (user needs to complete DB setup via web)
if [[ -f "$DOC_ROOT/wp-config-sample.php" && ! -f "$DOC_ROOT/wp-config.php" ]]; then
    cp "$DOC_ROOT/wp-config-sample.php" "$DOC_ROOT/wp-config.php"
    chown "$SITE_USER:$SITE_USER" "$DOC_ROOT/wp-config.php"
fi

# 5. Create security .htaccess for WordPress
cat > "$DOC_ROOT/.htaccess" << 'HTEOF'
# WordPress permalinks
<IfModule litehttpd_htaccess>
RewriteEngine On
RewriteBase /
RewriteRule ^index\.php$ - [L]
RewriteCond %{REQUEST_FILENAME} !-f
RewriteCond %{REQUEST_FILENAME} !-d
RewriteRule . /index.php [L]
</IfModule>

# Security headers
Header always set X-Content-Type-Options "nosniff"
Header always set X-Frame-Options "SAMEORIGIN"
Header always set X-XSS-Protection "1; mode=block"

# Protect sensitive files
<FilesMatch "^(wp-config\.php|readme\.html|license\.txt)$">
    Require all denied
</FilesMatch>

# Block XML-RPC
<Files xmlrpc.php>
    Require all denied
</Files>

# Protect uploads from PHP execution
<If "%{REQUEST_URI} =~ m#/wp-content/uploads/.*\.php#">
    Require all denied
</If>

# Brute force protection
LSBruteForceProtection On
LSBruteForceAllowedAttempts 5
LSBruteForceWindow 300
LSBruteForceAction throttle
LSBruteForceThrottleDuration 60
LSBruteForceProtectPath /wp-login.php
HTEOF
chown "$SITE_USER:$SITE_USER" "$DOC_ROOT/.htaccess"

# 6. Get WordPress version
WP_VER=$(grep 'wp_version =' "$DOC_ROOT/wp-includes/version.php" 2>/dev/null | grep -oP "'[^']+'" | tr -d "'" || echo "unknown")

cat << EOF
{"ok": true, "data": {"app": "wordpress", "version": "$WP_VER", "domain": "$DOMAIN", "doc_root": "$DOC_ROOT"}}
EOF
