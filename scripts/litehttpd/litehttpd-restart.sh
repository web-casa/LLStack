#!/bin/bash
set -euo pipefail
/usr/local/lsws/bin/lswsctrl restart 2>/dev/null || true
echo '{"ok":true}'
