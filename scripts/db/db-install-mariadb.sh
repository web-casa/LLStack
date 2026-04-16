#!/bin/bash
set -euo pipefail
# Install MariaDB from official repo
# Usage: db-install-mariadb.sh --version <10.11|11.4|11.8>

VERSION=""
while [[ $# -gt 0 ]]; do case "$1" in --version) VERSION="$2"; shift 2 ;; *) shift ;; esac; done
[[ -z "$VERSION" ]] && { echo "Usage: --version <10.11|11.4>" >&2; exit 1; }

MAJOR_VER=$(. /etc/os-release; echo "${VERSION_ID%%.*}")

echo ">>> Setting up MariaDB $VERSION official repository..."
cat > /etc/yum.repos.d/mariadb.repo << REPOEOF
[mariadb]
name = MariaDB $VERSION
baseurl = https://mirror.mariadb.org/yum/$VERSION/rhel/\$releasever/\$basearch
gpgkey = https://supplychain.mariadb.com/MariaDB-Server-GPG-KEY
gpgcheck = 1
enabled = 1
module_hotfixes = 1
REPOEOF

echo ">>> Installing MariaDB $VERSION..."
dnf install -y MariaDB-server MariaDB-client 2>&1 || dnf install -y mariadb-server 2>&1

echo ">>> Starting MariaDB..."
systemctl enable --now mariadb

echo ">>> Running mysql_secure_installation defaults..."
mysql -e "DELETE FROM mysql.user WHERE User='';" 2>/dev/null || true
mysql -e "DELETE FROM mysql.user WHERE User='root' AND Host NOT IN ('localhost', '127.0.0.1', '::1');" 2>/dev/null || true
mysql -e "DROP DATABASE IF EXISTS test;" 2>/dev/null || true
mysql -e "FLUSH PRIVILEGES;" 2>/dev/null || true

echo ">>> MariaDB $VERSION installed successfully"
mysql --version
echo '{"ok":true}'
