#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
COMPOSE_FILE="$ROOT_DIR/docker/compose/functional.yaml"
ARTIFACTS_DIR="${LLSTACK_DOCKER_ARTIFACTS_DIR:-$ROOT_DIR/dist/docker-smoke}"
DOCKER_PLATFORM="${LLSTACK_DOCKER_PLATFORM:-}"

if [ $# -eq 0 ]; then
  mapfile -t services < <(docker compose -f "$COMPOSE_FILE" config --services)
else
  services=("$@")
fi

cleanup() {
  docker compose -f "$COMPOSE_FILE" down --remove-orphans --volumes >/dev/null 2>&1 || true
}

assert_success_marker() {
  local log_file="$1"
  if ! grep -Fq '"status": "passed"' "$log_file"; then
    echo "docker smoke log is missing a passed status marker: $log_file" >&2
    cat "$log_file" >&2
    exit 1
  fi
}

if ! docker info >/dev/null 2>&1; then
  echo "docker daemon is unavailable for the current user; check /var/run/docker.sock access" >&2
  exit 2
fi

mkdir -p "$ARTIFACTS_DIR"
cleanup
trap cleanup EXIT

docker compose -f "$COMPOSE_FILE" config -q

for service in "${services[@]}"; do
  echo "running docker functional smoke for $service"
  log_file="$ARTIFACTS_DIR/$service.log"
  platform_args=()
  if [ -n "$DOCKER_PLATFORM" ]; then
    platform_args=(--build-arg "TARGETOS=linux" --build-arg "TARGETARCH=${DOCKER_PLATFORM#linux/}")
  fi
  COMPOSE_DOCKER_CLI_BUILD=1 DOCKER_BUILDKIT=1 docker compose -f "$COMPOSE_FILE" build ${platform_args:+"${platform_args[@]}"} "$service"
  docker compose -f "$COMPOSE_FILE" up --abort-on-container-exit --exit-code-from "$service" "$service" | tee "$log_file"
  docker compose -f "$COMPOSE_FILE" logs --no-color "$service" >>"$log_file"
  assert_success_marker "$log_file"
  docker compose -f "$COMPOSE_FILE" rm -sf "$service" >/dev/null 2>&1 || true
done

bash "$ROOT_DIR/scripts/docker/functional-report.sh" "${services[@]}"

echo "docker functional smoke completed for: ${services[*]}"
echo "artifacts: $ARTIFACTS_DIR"
