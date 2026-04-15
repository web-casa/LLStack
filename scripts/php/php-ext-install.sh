#!/bin/bash
set -euo pipefail
VERSION="" EXT=""
while [[ $# -gt 0 ]]; do case "$1" in --version) VERSION="$2"; shift 2 ;; --ext) EXT="$2"; shift 2 ;; *) shift ;; esac; done
[[ -z "$VERSION" || -z "$EXT" ]] && { echo '{"ok":false,"error":"missing_args"}' >&2; exit 1; }
dnf install -y "php${VERSION}-php-${EXT}" 2>&1 | tail -1
echo "{\"ok\":true,\"data\":{\"version\":\"php${VERSION}\",\"extension\":\"$EXT\"}}"
