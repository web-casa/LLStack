#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ARTIFACTS_DIR="${LLSTACK_DOCKER_ARTIFACTS_DIR:-$ROOT_DIR/dist/docker-smoke}"
SUMMARY_PATH="${LLSTACK_DOCKER_SUMMARY_PATH:-$ARTIFACTS_DIR/summary.json}"

DEFAULT_SERVICES=(
  "el9-apache"
  "el9-ols"
  "el9-lsws"
  "el10-apache"
  "el10-ols"
  "el10-lsws"
)

if [ $# -eq 0 ]; then
  services=("${DEFAULT_SERVICES[@]}")
else
  services=("$@")
fi

mkdir -p "$ARTIFACTS_DIR"

json_escape() {
  local value="$1"
  value="${value//\\/\\\\}"
  value="${value//\"/\\\"}"
  value="${value//$'\n'/\\n}"
  value="${value//$'\r'/\\r}"
  value="${value//$'\t'/\\t}"
  printf '%s' "$value"
}

all_passed=true
json_services=""
markdown_rows=""

for service in "${services[@]}"; do
  log_path="$ARTIFACTS_DIR/$service.log"
  status="missing"
  note="artifact missing"

  if [ -f "$log_path" ]; then
    if grep -Fq '"status": "passed"' "$log_path"; then
      status="passed"
      note="success marker found"
    else
      status="failed"
      note="success marker missing"
    fi
  fi

  if [ "$status" != "passed" ]; then
    all_passed=false
  fi

  escaped_log_path="$(json_escape "$log_path")"
  escaped_note="$(json_escape "$note")"
  if [ -n "$json_services" ]; then
    json_services+=","
  fi
  json_services+=$'\n'"    {\"service\":\"$service\",\"status\":\"$status\",\"log_path\":\"$escaped_log_path\",\"note\":\"$escaped_note\"}"
  markdown_rows+=$'\n'"- \`$service\`: \`$status\` ($note)"
done

generated_at="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
overall_status="passed"
exit_code=0
if [ "$all_passed" != "true" ]; then
  overall_status="failed"
  exit_code=1
fi

cat >"$SUMMARY_PATH" <<EOF
{
  "generated_at": "$generated_at",
  "artifacts_dir": "$(json_escape "$ARTIFACTS_DIR")",
  "overall_status": "$overall_status",
  "services": [$json_services
  ]
}
EOF

cat <<EOF
# Docker Smoke Summary

- generated_at: \`$generated_at\`
- artifacts_dir: \`$ARTIFACTS_DIR\`
- overall_status: \`$overall_status\`
- summary_json: \`$SUMMARY_PATH\`
$markdown_rows
EOF

exit "$exit_code"
