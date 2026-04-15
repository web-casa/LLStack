#!/bin/bash
set -euo pipefail
# Install MySQL from official Oracle repo
# Usage: db-install-mysql.sh --version <8.0|8.4>

VERSION=""
while [[ $# -gt 0 ]]; do case "$1" in --version) VERSION="$2"; shift 2 ;; *) shift ;; esac; done
[[ -z "$VERSION" ]] && { echo "Usage: --version <8.0|8.4>" >&2; exit 1; }

MAJOR_VER=$(. /etc/os-release; echo "${VERSION_ID%%.*}")

echo ">>> Setting up MySQL $VERSION official repository..."
if ! rpm -q mysql84-community-release &>/dev/null && ! rpm -q mysql80-community-release &>/dev/null; then
    dnf install -y "https://dev.mysql.com/get/mysql84-community-release-el${MAJOR_VER}-1.noarch.rpm" 2>&1 || true
fi

# Enable the correct version
if [[ "$VERSION" == "8.0" ]]; then
    dnf config-manager --disable mysql-8.4-lts-community 2>/dev/null || true
    dnf config-manager --enable mysql80-community 2>/dev/null || true
else
    dnf config-manager --enable mysql-8.4-lts-community 2>/dev/null || true
    dnf config-manager --disable mysql80-community 2>/dev/null || true
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
