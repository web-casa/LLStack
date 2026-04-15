#!/bin/bash
set -euo pipefail
/usr/local/lsws/bin/lswsctrl reload 2>/dev/null || true
echo '{"ok":true}'
