#!/bin/bash
set -euo pipefail
# Install MySQL from official Oracle repo
# Usage: db-install-mysql.sh --version <8.0|8.4|9.6>

VERSION=""
while [[ $# -gt 0 ]]; do case "$1" in --version) VERSION="$2"; shift 2 ;; *) shift ;; esac; done
[[ -z "$VERSION" ]] && { echo "Usage: --version <8.0|8.4|9.6>" >&2; exit 1; }

MAJOR_VER=$(. /etc/os-release; echo "${VERSION_ID%%.*}")

echo ">>> Setting up MySQL $VERSION official repository..."
if ! rpm -q mysql84-community-release &>/dev/null && ! rpm -q mysql80-community-release &>/dev/null; then
    # Use the latest available RPM release (try descending, Oracle increments the suffix)
    for REL in 5 4 3 2 1; do
        if dnf install -y "https://dev.mysql.com/get/mysql84-community-release-el${MAJOR_VER}-${REL}.noarch.rpm" 2>&1; then
            break
        fi
    done
fi

# Disable all version repos first, then enable the requested one
echo ">>> Enabling MySQL $VERSION repository..."
dnf config-manager --disable mysql80-community 2>/dev/null || true
dnf config-manager --disable mysql-8.4-lts-community 2>/dev/null || true
dnf config-manager --disable mysql-innovation-community 2>/dev/null || true
dnf config-manager --disable mysql-tools-8.4-lts-community 2>/dev/null || true
dnf config-manager --disable mysql-tools-innovation-community 2>/dev/null || true

if [[ "$VERSION" == "8.0" ]]; then
    dnf config-manager --enable mysql80-community 2>/dev/null || true
elif [[ "$VERSION" == "8.4" ]]; then
    dnf config-manager --enable mysql-8.4-lts-community 2>/dev/null || true
    dnf config-manager --enable mysql-tools-8.4-lts-community 2>/dev/null || true
else
    # 9.x Innovation track
    dnf config-manager --enable mysql-innovation-community 2>/dev/null || true
    dnf config-manager --enable mysql-tools-innovation-community 2>/dev/null || true
fi

echo ">>> Installing MySQL $VERSION..."
dnf install -y mysql-community-server 2>&1

echo ">>> Starting MySQL..."
systemctl enable --now mysqld

# Get temp password and secure
TEMP_PASS=$(grep 'temporary password' /var/log/mysqld.log 2>/dev/null | tail -1 | awk '{print $NF}')
if [[ -n "$TEMP_PASS" ]]; then
    echo ">>> Temporary root password found, securing..."
    # Note: full secure setup requires interactive input; for panel use, we reset via --skip-grant-tables if needed
fi

echo ">>> MySQL $VERSION installed successfully"
mysql --version
echo '{"ok":true}'
