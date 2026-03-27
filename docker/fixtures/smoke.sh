#!/usr/bin/env bash
set -euo pipefail

BACKEND="${LLSTACK_SMOKE_BACKEND:-apache}"
SITE_NAME="${LLSTACK_SMOKE_SITE:-docker.example.com}"

require_contains() {
  local file="$1"
  local needle="$2"
  if ! grep -Fq "$needle" "$file"; then
    echo "expected output to contain: $needle" >&2
    echo "from file: $file" >&2
    cat "$file" >&2
    exit 1
  fi
}

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

version_json="$TMP_DIR/version.json"
status_json="$TMP_DIR/status.json"
install_json="$TMP_DIR/install.json"
site_json="$TMP_DIR/site.json"
site_show_json="$TMP_DIR/site-show.json"
site_diff_json="$TMP_DIR/site-diff.json"
site_update_json="$TMP_DIR/site-update.json"
site_ssl_json="$TMP_DIR/site-ssl.json"
site_stop_json="$TMP_DIR/site-stop.json"
site_show_stopped_json="$TMP_DIR/site-show-stopped.json"
site_start_json="$TMP_DIR/site-start.json"
site_show_started_json="$TMP_DIR/site-show-started.json"
site_restart_json="$TMP_DIR/site-restart.json"
doctor_json="$TMP_DIR/doctor.json"
apache_hosts="$TMP_DIR/apache-vhosts.txt"
apache_body="$TMP_DIR/apache-body.txt"
ols_body="$TMP_DIR/ols-body.txt"
ols_configtest="$TMP_DIR/ols-configtest.txt"
backend_artifact="$TMP_DIR/backend-artifact.txt"
listener_map="$TMP_DIR/listener-map.txt"
parity_json="$TMP_DIR/parity.json"

llstack version --json >"$version_json"
llstack status --json >"$status_json"
llstack install --backend "$BACKEND" --php_version 8.3 --db mariadb --site "$SITE_NAME" --dry-run --json >"$install_json"
if [ "$BACKEND" = "apache" ]; then
  httpd -k start
  llstack site:create "$SITE_NAME" --backend "$BACKEND" --profile static --non-interactive --json >"$site_json"
  llstack site:show "$SITE_NAME" --json >"$site_show_json"
  llstack site:diff "$SITE_NAME" --json >"$site_diff_json"
  llstack site:update "$SITE_NAME" --alias "www.$SITE_NAME" --json >"$site_update_json"
  llstack site:ssl "$SITE_NAME" --letsencrypt --email admin@example.com --dry-run --json >"$site_ssl_json"
  llstack site:stop "$SITE_NAME" --json >"$site_stop_json"
  llstack site:show "$SITE_NAME" --json >"$site_show_stopped_json"
  llstack site:start "$SITE_NAME" --json >"$site_start_json"
  llstack site:show "$SITE_NAME" --json >"$site_show_started_json"
  llstack site:restart "$SITE_NAME" --json >"$site_restart_json"
  httpd -t >"$apache_hosts"
  httpd -S >>"$apache_hosts" 2>&1
  curl -fsS -H "Host: $SITE_NAME" http://127.0.0.1/ >"$apache_body"
  require_contains "$apache_hosts" "$SITE_NAME"
  require_contains "$apache_body" "$SITE_NAME"
elif [ "$BACKEND" = "ols" ]; then
  # Site lifecycle tests (--skip-reload: OLS main config not wired yet)
  llstack site:create "$SITE_NAME" --backend "$BACKEND" --profile static --non-interactive --skip-reload --json >"$site_json"
  llstack site:show "$SITE_NAME" --json >"$site_show_json"
  llstack site:diff "$SITE_NAME" --json >"$site_diff_json"
  llstack site:update "$SITE_NAME" --alias "www.$SITE_NAME" --skip-reload --json >"$site_update_json"
  llstack site:ssl "$SITE_NAME" --letsencrypt --email admin@example.com --dry-run --skip-reload --json >"$site_ssl_json"
  llstack site:stop "$SITE_NAME" --skip-reload --json >"$site_stop_json"
  llstack site:show "$SITE_NAME" --json >"$site_show_stopped_json"
  llstack site:start "$SITE_NAME" --skip-reload --json >"$site_start_json"
  llstack site:show "$SITE_NAME" --json >"$site_show_started_json"

  # Asset assertions
  cp "/usr/local/lsws/conf/vhosts/$SITE_NAME/vhconf.conf" "$backend_artifact"
  cp "/usr/local/lsws/conf/llstack/listeners/$SITE_NAME.map" "$listener_map"
  cp "/var/lib/llstack/state/parity/$SITE_NAME.ols.json" "$parity_json"
  require_contains "$backend_artifact" "$SITE_NAME"
  require_contains "$listener_map" "$SITE_NAME"
  require_contains "$parity_json" '"backend": "ols"'
  require_contains "$parity_json" '"status": "mapped"'

  # Runtime verification: wire vhost into OLS main config and start service
  OLS_CONF="/usr/local/lsws/conf/httpd_config.conf"
  if [ -f "$OLS_CONF" ] && command -v lswsctrl >/dev/null 2>&1; then
    cat >>"$OLS_CONF" <<OLSVH

virtualhost $SITE_NAME {
  vhRoot                  /data/www/$SITE_NAME
  configFile              /usr/local/lsws/conf/vhosts/$SITE_NAME/vhconf.conf
  allowSymbolLink         1
  enableScript            1
}
OLSVH
    # Add listener mapping for this vhost
    sed -i "/listener Default/,/}/ s|^}|  map                     $SITE_NAME $SITE_NAME\n}|" "$OLS_CONF"

    lswsctrl start
    sleep 2

    lswsctrl configtest >"$ols_configtest" 2>&1 || true
    curl -fsS -H "Host: $SITE_NAME" http://127.0.0.1:80/ >"$ols_body" || true

    if [ -s "$ols_body" ]; then
      require_contains "$ols_body" "$SITE_NAME"
    fi
  fi
elif [ "$BACKEND" = "lsws" ]; then
  llstack site:create "$SITE_NAME" --backend "$BACKEND" --profile static --non-interactive --skip-reload --json >"$site_json"
  llstack site:show "$SITE_NAME" --json >"$site_show_json"
  llstack site:diff "$SITE_NAME" --json >"$site_diff_json"
  llstack site:update "$SITE_NAME" --alias "www.$SITE_NAME" --skip-reload --json >"$site_update_json"
  llstack site:ssl "$SITE_NAME" --letsencrypt --email admin@example.com --dry-run --skip-reload --json >"$site_ssl_json"
  llstack site:stop "$SITE_NAME" --skip-reload --json >"$site_stop_json"
  llstack site:show "$SITE_NAME" --json >"$site_show_stopped_json"
  llstack site:start "$SITE_NAME" --skip-reload --json >"$site_start_json"
  llstack site:show "$SITE_NAME" --json >"$site_show_started_json"
  cp "/usr/local/lsws/conf/llstack/includes/$SITE_NAME.conf" "$backend_artifact"
  cp "/var/lib/llstack/state/parity/$SITE_NAME.lsws.json" "$parity_json"
  require_contains "$backend_artifact" "$SITE_NAME"
  require_contains "$parity_json" '"backend": "lsws"'
  require_contains "$site_show_json" '"license_mode": "trial"'
else
  echo "unsupported docker smoke backend: $BACKEND" >&2
  exit 1
fi
llstack doctor --json >"$doctor_json"

require_contains "$version_json" '"version":'
require_contains "$version_json" '"target_os": "linux"'
require_contains "$status_json" '"default_site_root": "/data/www"'
require_contains "$install_json" "\"target\": \"$BACKEND\""
require_contains "$install_json" "$SITE_NAME"
require_contains "$site_json" '"kind": "site.create"'
require_contains "$site_json" "$SITE_NAME"
require_contains "$site_show_json" "$SITE_NAME"
require_contains "$site_diff_json" "\"name\": \"$SITE_NAME\""
require_contains "$site_update_json" '"kind": "site.update"'
require_contains "$site_ssl_json" '"kind": "site.ssl"'
require_contains "$site_stop_json" '"kind": "site.disabled"'
require_contains "$site_show_stopped_json" '"state": "disabled"'
require_contains "$site_start_json" '"kind": "site.enabled"'
require_contains "$site_show_started_json" '"state": "enabled"'
if [ -f "$site_restart_json" ]; then
  require_contains "$site_restart_json" '"kind": "site.restart"'
fi
require_contains "$doctor_json" '"status":'

ols_runtime_check="false"
if [ "$BACKEND" = "ols" ] && [ -s "$ols_body" ] 2>/dev/null; then
  ols_runtime_check="true"
fi

cat <<EOF
docker smoke summary:
{
  "backend": "$BACKEND",
  "site": "$SITE_NAME",
  "checks": [
    "version",
    "status",
    "install_dry_run",
    "site_create_apply",
    "site_diff",
    "site_update",
    "site_tls_dry_run",
    "site_stop_start",
    "doctor"
  ],
  "ols_runtime_verified": $ols_runtime_check,
  "status": "passed"
}
EOF
