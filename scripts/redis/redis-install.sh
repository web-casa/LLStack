#!/bin/bash
set -euo pipefail
# Install Redis server (REMI module on EL10, EPEL on EL9)

MAJOR_VER=$(. /etc/os-release; echo "${VERSION_ID%%.*}")

echo ">>> Installing Redis..."
if [[ "$MAJOR_VER" == "10" ]]; then
    dnf module enable redis:remi-8.6 -y 2>/dev/null || true
    dnf install -y redis 2>&1 || dnf install -y valkey 2>&1
else
    dnf install -y redis 2>&1
fi

echo ">>> Starting Redis..."
systemctl enable --now redis 2>/dev/null || systemctl enable --now valkey 2>/dev/null || true

echo ">>> Redis installed"
redis-server --version 2>/dev/null || valkey-server --version 2>/dev/null || true
echo '{"ok":true}'
