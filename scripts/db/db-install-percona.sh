#!/bin/bash
set -euo pipefail
# Install Percona Server from official repo
# Usage: db-install-percona.sh --version <8.0|8.4>

VERSION=""
while [[ $# -gt 0 ]]; do case "$1" in --version) VERSION="$2"; shift 2 ;; *) shift ;; esac; done
[[ -z "$VERSION" ]] && { echo "Usage: --version <8.0|8.4>" >&2; exit 1; }

MAJOR_VER=$(. /etc/os-release; echo "${VERSION_ID%%.*}")

echo ">>> Setting up Percona repository..."
if ! rpm -q percona-release &>/dev/null; then
    dnf install -y "https://repo.percona.com/yum/percona-release-latest.noarch.rpm" 2>&1
fi

echo ">>> Enabling Percona Server $VERSION..."
if [[ "$VERSION" == "8.0" ]]; then
    percona-release setup ps80 2>&1
elif [[ "$VERSION" == "8.4" ]]; then
    # Try multiple known setup strings for Percona 8.4
    percona-release setup ps-8.4-lts 2>&1 || \
    percona-release setup ps-84-lts 2>&1 || \
    percona-release setup ps84 2>&1 || \
    { echo '{"ok":false,"error":"percona_84_setup_failed"}' >&2; exit 1; }
else
    echo "Unsupported version: $VERSION" >&2
    exit 1
fi

echo ">>> Installing Percona Server..."
dnf install -y percona-server-server 2>&1

echo ">>> Starting Percona Server..."
systemctl enable --now mysqld

echo ">>> Percona Server $VERSION installed successfully"
mysql --version
echo '{"ok":true}'
