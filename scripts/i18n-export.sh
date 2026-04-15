#!/bin/bash
set -euo pipefail

# Export i18n keys for community translation contribution
# Usage: i18n-export.sh [--lang <code>] [--format <json|csv>]
# Example: i18n-export.sh --lang ja --format csv > ja-template.csv

LANG_CODE=""
FORMAT="json"

while [[ $# -gt 0 ]]; do
    case "$1" in
        --lang)   LANG_CODE="$2"; shift 2 ;;
        --format) FORMAT="$2"; shift 2 ;;
        *) shift ;;
    esac
done

LOCALES_DIR="$(dirname "$0")/../web/src/locales"
EN_FILE="$LOCALES_DIR/en.json"

if [[ ! -f "$EN_FILE" ]]; then
    echo "Error: en.json not found at $EN_FILE" >&2
    exit 1
fi

if [[ "$FORMAT" == "csv" ]]; then
    echo "key,english,translation"
    python3 -c "
import json, csv, sys

def flatten(d, prefix=''):
    for k, v in d.items():
        key = f'{prefix}.{k}' if prefix else k
        if isinstance(v, dict):
            flatten(v, key)
        else:
            writer.writerow([key, v, ''])

writer = csv.writer(sys.stdout)
with open('$EN_FILE') as f:
    flatten(json.load(f))
"
elif [[ "$FORMAT" == "json" ]]; then
    if [[ -n "$LANG_CODE" ]]; then
        # Generate template with empty values
        python3 -c "
import json

def empty_values(d):
    result = {}
    for k, v in d.items():
        if isinstance(v, dict):
            result[k] = empty_values(v)
        else:
            result[k] = ''
    return result

with open('$EN_FILE') as f:
    data = json.load(f)

print(json.dumps(empty_values(data), ensure_ascii=False, indent=4))
"
    else
        # Show stats
        python3 -c "
import json

def count_keys(d, prefix=''):
    count = 0
    for k, v in d.items():
        if isinstance(v, dict):
            count += count_keys(v, f'{prefix}.{k}')
        else:
            count += 1
    return count

with open('$EN_FILE') as f:
    en = json.load(f)

print(f'Total i18n keys: {count_keys(en)}')
print(f'Namespaces: {list(en.keys())}')
print(f'\\nAvailable locales:')
import os, glob
for f in sorted(glob.glob('$LOCALES_DIR/*.json')):
    name = os.path.basename(f)
    with open(f) as fh:
        keys = count_keys(json.load(fh))
    print(f'  {name}: {keys} keys')
"
    fi
fi
