#!/bin/bash
set -euo pipefail

# Generate WordPress SSO login file (self-deleting PHP token)
# Usage: wp-sso.sh --path <wp_root> --token-file <path>

WP_PATH="" TOKEN="" TOKEN_FILE=""
while [[ $# -gt 0 ]]; do
    case "$1" in
        --path)       WP_PATH="$2"; shift 2 ;;
        --token)      TOKEN="$2"; shift 2 ;;
        --token-file) TOKEN_FILE="$2"; shift 2 ;;
        *) echo '{"ok":false,"error":"unknown_arg"}' >&2; exit 1 ;;
    esac
done

# Read token from file if provided (avoids /proc exposure)
if [[ -n "$TOKEN_FILE" && -f "$TOKEN_FILE" ]]; then
    TOKEN=$(cat "$TOKEN_FILE")
    rm -f "$TOKEN_FILE" 2>/dev/null || true
fi

[[ -z "$WP_PATH" || -z "$TOKEN" ]] && { echo '{"ok":false,"error":"missing_args"}' >&2; exit 1; }

# Validate token format (urlsafe base64 only)
if ! echo "$TOKEN" | grep -qP '^[A-Za-z0-9_-]+$'; then
    echo '{"ok":false,"error":"invalid_token_format"}' >&2; exit 1
fi
[[ ! -f "$WP_PATH/wp-config.php" ]] && { echo '{"ok":false,"error":"not_wordpress"}' >&2; exit 1; }

SSO_FILE="wp-llstack-sso-${TOKEN:0:16}.php"
SSO_PATH="$WP_PATH/$SSO_FILE"
EXPIRY=$(($(date +%s) + 60))

cat > "$SSO_PATH" << 'PHPEOF'
<?php
/**
 * LLStack WordPress SSO — self-deleting after use
 */
$expected_token = '%%TOKEN%%';
$expiry = %%EXPIRY%%;
$token = $_GET['token'] ?? '';

if ($token !== $expected_token || time() > $expiry) {
    @unlink(__FILE__);
    http_response_code(403);
    die('Token expired or invalid');
}

// Self-delete immediately
@unlink(__FILE__);

// Bootstrap WordPress
define('ABSPATH', __DIR__ . '/');
require_once ABSPATH . 'wp-load.php';

// Login as first admin user
$admins = get_users(['role' => 'administrator', 'number' => 1, 'orderby' => 'ID']);
if (empty($admins)) {
    wp_die('No admin user found');
}

$user = $admins[0];
wp_set_current_user($user->ID);
wp_set_auth_cookie($user->ID, true);
do_action('wp_login', $user->user_login, $user);

// Redirect to dashboard
wp_safe_redirect(admin_url());
exit;
PHPEOF

# Replace placeholders
sed -i "s|%%TOKEN%%|$TOKEN|g" "$SSO_PATH"
sed -i "s|%%EXPIRY%%|$EXPIRY|g" "$SSO_PATH"

# Set ownership to site user
SITE_USER=$(stat -c '%U' "$WP_PATH" 2>/dev/null || echo "nobody")
chown "$SITE_USER:$SITE_USER" "$SSO_PATH" 2>/dev/null || true
chmod 600 "$SSO_PATH"

echo "{\"ok\":true,\"data\":{\"sso_file\":\"$SSO_FILE\",\"expires_at\":$EXPIRY}}"
