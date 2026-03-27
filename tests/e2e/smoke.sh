#!/usr/bin/env bash
set -euo pipefail

BIN_PATH="${1:-}"
BACKEND="${LLSTACK_SMOKE_BACKEND:-apache}"
SITE_NAME="${LLSTACK_SMOKE_SITE:-smoke.example.com}"

if [ -z "$BIN_PATH" ]; then
  echo "usage: tests/e2e/smoke.sh <binary-path>" >&2
  exit 1
fi
if [ ! -x "$BIN_PATH" ]; then
  echo "binary is not executable: $BIN_PATH" >&2
  exit 1
fi

"$BIN_PATH" version --json >/dev/null
"$BIN_PATH" status --json >/dev/null
"$BIN_PATH" install --backend "$BACKEND" --php_version 8.3 --db mariadb --site "$SITE_NAME" --dry-run --json >/dev/null
"$BIN_PATH" site:create "$SITE_NAME" --backend "$BACKEND" --non-interactive --dry-run --json >/dev/null
"$BIN_PATH" doctor --json >/dev/null

echo "smoke checks passed for $BIN_PATH"
