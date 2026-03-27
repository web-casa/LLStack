#!/usr/bin/env bash
set -euo pipefail

LSWSCTRL="/usr/local/lsws/bin/lswsctrl"

if [ $# -eq 0 ]; then
  echo "unsupported systemctl invocation: no arguments provided" >&2
  exit 1
fi

if [ "$1" = "--failed" ]; then
  echo "0 loaded units listed."
  exit 0
fi

if [ "$1" = "daemon-reload" ]; then
  exit 0
fi

if [ $# -lt 2 ]; then
  echo "unsupported systemctl invocation: $*" >&2
  exit 1
fi

action="$1"
unit="$2"

if [ "$action" = "is-active" ] && [ "$unit" != "lsws" ]; then
  echo "inactive"
  exit 3
fi

if [ "$action" = "start" ] || [ "$action" = "stop" ] || [ "$action" = "restart" ] || [ "$action" = "reload" ]; then
  if [ "$unit" != "lsws" ]; then
    exit 0
  fi
fi

if [ "$action" = "enable" ] || [ "$action" = "disable" ]; then
  exit 0
fi

if [ "$unit" != "lsws" ]; then
  echo "unsupported unit in container shim: $unit" >&2
  exit 1
fi

case "$action" in
  start)
    exec "$LSWSCTRL" start
    ;;
  reload)
    exec "$LSWSCTRL" reload
    ;;
  restart)
    exec "$LSWSCTRL" restart
    ;;
  stop)
    exec "$LSWSCTRL" stop
    ;;
  is-active)
    if pgrep -x lshttpd >/dev/null 2>&1; then
      echo "active"
      exit 0
    fi
    echo "inactive"
    exit 3
    ;;
  *)
    echo "unsupported systemctl action in container shim: $action" >&2
    exit 1
    ;;
esac
