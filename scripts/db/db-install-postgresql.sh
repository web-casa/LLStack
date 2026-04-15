#!/bin/bash
set -euo pipefail
# Install PostgreSQL from PGDG official repo
# Usage: db-install-postgresql.sh --version <16|17>

VERSION=""
while [[ $# -gt 0 ]]; do case "$1" in --version) VERSION="$2"; shift 2 ;; *) shift ;; esac; done
[[ -z "$VERSION" ]] && { echo "Usage: --version <16|17>" >&2; exit 1; }

MAJOR_VER=$(. /etc/os-release; echo "${VERSION_ID%%.*}")

echo ">>> Setting up PostgreSQL PGDG repository..."
if ! rpm -q pgdg-redhat-repo &>/dev/null; then
    dnf install -y "https://download.postgresql.org/pub/repos/yum/reporpms/EL-${MAJOR_VER}-x86_64/pgdg-redhat-repo-latest.noarch.rpm" 2>&1
fi

# Disable built-in PostgreSQL module
dnf -qy module disable postgresql 2>/dev/null || true

echo ">>> Installing PostgreSQL $VERSION..."
dnf install -y "postgresql${VERSION}-server" "postgresql${VERSION}" 2>&1

echo ">>> Initializing database..."
"/usr/pgsql-${VERSION}/bin/postgresql-${VERSION}-setup" initdb 2>&1 || true

echo ">>> Starting PostgreSQL..."
systemctl enable --now "postgresql-${VERSION}"

echo ">>> PostgreSQL $VERSION installed successfully"
"/usr/pgsql-${VERSION}/bin/psql" --version
echo '{"ok":true}'
