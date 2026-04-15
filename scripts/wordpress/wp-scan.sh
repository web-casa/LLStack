#!/bin/bash
set -euo pipefail

# Scan filesystem for WordPress installations
# Usage: wp-scan.sh [--base-dir /home]
# Returns JSON array of found WP installations

BASE_DIRS=()
while [[ $# -gt 0 ]]; do
    case "$1" in
        --base-dir) BASE_DIRS+=("$2"); shift 2 ;;
        *) shift ;;
    esac
done

# Default: scan /home if no dirs specified
if [[ ${#BASE_DIRS[@]} -eq 0 ]]; then
    BASE_DIRS=("/home")
fi

WP_CLI=""
for path in /usr/local/bin/wp /usr/bin/wp; do
    if [[ -x "$path" ]]; then
        WP_CLI="$path"
        break
    fi
done

echo '['
FIRST=true

# Find all wp-config.php files under home directories
while IFS= read -r config_file; do
    wp_dir=$(dirname "$config_file")

    # Extract site info
    VERSION=""
    SITE_URL=""
    TITLE=""

    if [[ -n "$WP_CLI" ]]; then
        # Get info via wp-cli (most reliable)
        VERSION=$($WP_CLI core version --path="$wp_dir" --allow-root 2>/dev/null || echo "")
        SITE_URL=$($WP_CLI option get siteurl --path="$wp_dir" --allow-root --skip-plugins --skip-themes 2>/dev/null || echo "")
        TITLE=$($WP_CLI option get blogname --path="$wp_dir" --allow-root --skip-plugins --skip-themes 2>/dev/null || echo "")
    fi

    # Fallback: parse wp-includes/version.php
    if [[ -z "$VERSION" && -f "$wp_dir/wp-includes/version.php" ]]; then
        VERSION=$(grep -oP "wp_version\s*=\s*'\K[^']+" "$wp_dir/wp-includes/version.php" 2>/dev/null || echo "")
    fi

    # Detect owner
    OWNER=$(stat -c '%U' "$wp_dir" 2>/dev/null || echo "unknown")

    if [[ "$FIRST" == true ]]; then
        FIRST=false
    else
        echo ','
    fi

    # Use Python to build safe JSON for all fields
    python3 -c "
import json, sys
print(json.dumps({
    'path': sys.argv[1],
    'version': sys.argv[2],
    'site_url': sys.argv[3],
    'title': sys.argv[4],
    'owner': sys.argv[5],
}))
" "$wp_dir" "$VERSION" "$SITE_URL" "$TITLE" "$OWNER" 2>/dev/null || echo '  {}'

done < <(for d in "${BASE_DIRS[@]}"; do find "$d" -maxdepth 5 -name "wp-config.php" -not -path "*/wp-content/*" 2>/dev/null; done)

echo ']'
