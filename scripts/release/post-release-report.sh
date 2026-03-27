#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
VERSION="${LLSTACK_VERSION:-}"
PACKAGE_DIR="${LLSTACK_PACKAGE_DIR:-$ROOT_DIR/dist/packages/$VERSION}"
SUMMARY_FILE="${LLSTACK_RELEASE_SUMMARY_OUT:-$PACKAGE_DIR/release-summary.md}"
SUMMARY_JSON="${LLSTACK_RELEASE_SUMMARY_JSON_OUT:-$PACKAGE_DIR/release-summary.json}"
ASSETS_FILE="${LLSTACK_RELEASE_ASSETS_FILE:-}"
URL_FILE="${LLSTACK_RELEASE_URL_FILE:-}"
RELEASE_URL="${LLSTACK_RELEASE_URL:-}"
REMOTE_VERIFY_JSON="${LLSTACK_RELEASE_REMOTE_VERIFY_JSON:-}"

if [ -z "$VERSION" ]; then
  echo "LLSTACK_VERSION is required" >&2
  exit 1
fi
if [ ! -d "$PACKAGE_DIR" ]; then
  echo "package directory not found: $PACKAGE_DIR" >&2
  exit 1
fi

if [ -n "$URL_FILE" ] && [ -f "$URL_FILE" ]; then
  RELEASE_URL="$(tr -d '\r' <"$URL_FILE")"
fi

expected_assets=()
for file in "$PACKAGE_DIR"/*; do
  [ -f "$file" ] || continue
  base="$(basename "$file")"
  case "$base" in
    release-summary.md|release-summary.json|release-assets.txt|release-url.txt|remote-verify.json|remote-verify.md)
      continue
      ;;
  esac
  expected_assets+=("$base")
done

if [ "${#expected_assets[@]}" -eq 0 ]; then
  echo "no release assets found in $PACKAGE_DIR" >&2
  exit 1
fi

archive_count=0
signature_count=0
for base in "${expected_assets[@]}"; do
  case "$base" in
    llstack-"$VERSION"-*.tar.gz)
      archive_count=$((archive_count + 1))
      ;;
    *.sig)
      signature_count=$((signature_count + 1))
      ;;
  esac
done

remote_assets=()
if [ -n "$ASSETS_FILE" ] && [ -f "$ASSETS_FILE" ]; then
  while IFS= read -r line; do
    [ -n "${line:-}" ] || continue
    remote_assets+=("$line")
  done <"$ASSETS_FILE"
fi

missing_assets=()
unexpected_assets=()
status="passed"
remote_status="not-run"
remote_verify_status="not-run"
remote_verify_signature_count=""
if [ -n "$REMOTE_VERIFY_JSON" ] && [ -f "$REMOTE_VERIFY_JSON" ]; then
  remote_verify_status="$(sed -n 's/.*"status": "\([^"]*\)".*/\1/p' "$REMOTE_VERIFY_JSON" | head -n1)"
  remote_verify_signature_count="$(sed -n 's/.*"signature_count": \([0-9][0-9]*\).*/\1/p' "$REMOTE_VERIFY_JSON" | head -n1)"
  if [ "$remote_verify_status" = "failed" ]; then
    status="failed"
  fi
fi

if [ "${#remote_assets[@]}" -gt 0 ]; then
  remote_status="passed"
  for base in "${expected_assets[@]}"; do
    found=0
    for remote in "${remote_assets[@]}"; do
      if [ "$remote" = "$base" ]; then
        found=1
        break
      fi
    done
    if [ "$found" -eq 0 ]; then
      missing_assets+=("$base")
      remote_status="failed"
      status="failed"
    fi
  done
  for remote in "${remote_assets[@]}"; do
    found=0
    for base in "${expected_assets[@]}"; do
      if [ "$remote" = "$base" ]; then
        found=1
        break
      fi
    done
    if [ "$found" -eq 0 ]; then
      unexpected_assets+=("$remote")
    fi
  done
fi
if [ "$remote_verify_status" = "failed" ] && [ "$remote_status" = "not-run" ]; then
  remote_status="failed"
fi

expected_list=""
for base in "${expected_assets[@]}"; do
  expected_list="${expected_list}- \`${base}\`\n"
done

remote_list="remote asset verification was not run."
if [ "${#remote_assets[@]}" -gt 0 ]; then
  remote_list=""
  for remote in "${remote_assets[@]}"; do
    remote_list="${remote_list}- \`${remote}\`\n"
  done
fi

missing_list="none"
if [ "${#missing_assets[@]}" -gt 0 ]; then
  missing_list=""
  for base in "${missing_assets[@]}"; do
    missing_list="${missing_list}- \`${base}\`\n"
  done
fi

unexpected_list="none"
if [ "${#unexpected_assets[@]}" -gt 0 ]; then
  unexpected_list=""
  for base in "${unexpected_assets[@]}"; do
    unexpected_list="${unexpected_list}- \`${base}\`\n"
  done
fi

mkdir -p "$(dirname "$SUMMARY_FILE")"
cat >"$SUMMARY_FILE" <<EOF
# Release Verification Summary

- Version: \`$VERSION\`
- Status: \`$status\`
- Release URL: ${RELEASE_URL:-not-published}
- Local archive count: \`$archive_count\`
- Detached signature count: \`$signature_count\`
- Remote asset verification: \`$remote_status\`
- Remote checksum/signature verification: \`${remote_verify_status}\`

## Expected Local Assets

$(printf "%b" "$expected_list")

## Remote Release Assets

$(printf "%b" "$remote_list")

## Missing Remote Assets

$(printf "%b" "$missing_list")

## Unexpected Remote Assets

$(printf "%b" "$unexpected_list")

## Remote Verify Details

- Detached signatures fetched: \`${remote_verify_signature_count:-unknown}\`
EOF

json_array() {
  local first=1
  for item in "$@"; do
    if [ $first -eq 0 ]; then
      printf ","
    fi
    printf "\"%s\"" "$item"
    first=0
  done
}

cat >"$SUMMARY_JSON" <<EOF
{
  "version": "$VERSION",
  "status": "$status",
  "release_url": "${RELEASE_URL}",
  "archive_count": $archive_count,
  "signature_count": $signature_count,
  "remote_status": "$remote_status",
  "remote_verify_status": "$remote_verify_status",
  "expected_assets": [$(json_array "${expected_assets[@]}")],
  "remote_assets": [$(json_array "${remote_assets[@]}")],
  "missing_assets": [$(json_array "${missing_assets[@]}")],
  "unexpected_assets": [$(json_array "${unexpected_assets[@]}")]
}
EOF

if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
  cat "$SUMMARY_FILE" >>"$GITHUB_STEP_SUMMARY"
fi

echo "release verification summary written to $SUMMARY_FILE"
echo "release verification json written to $SUMMARY_JSON"

if [ "$status" != "passed" ]; then
  exit 1
fi
