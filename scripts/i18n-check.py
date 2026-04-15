#!/usr/bin/env python3
"""Check i18n completeness for frontend translation keys."""

from __future__ import annotations

import json
import re
import sys
from pathlib import Path
from typing import Any

PROJECT_ROOT = Path(__file__).resolve().parent.parent
SOURCE_DIRS = (
    PROJECT_ROOT / "web" / "src" / "pages",
    PROJECT_ROOT / "web" / "src" / "components",
)
LOCALE_FILES = (
    PROJECT_ROOT / "web" / "src" / "locales" / "en.json",
    PROJECT_ROOT / "web" / "src" / "locales" / "zh.json",
    PROJECT_ROOT / "web" / "src" / "locales" / "zh-TW.json",
)
TRANSLATION_CALL_PATTERN = re.compile(
    r"""\bt\(\s*(["'])(?P<key>[^"'\\\r\n]+)\1\s*\)"""
)


def flatten_keys(value: Any, prefix: str = "") -> set[str]:
    """Flatten nested locale objects into dotted leaf keys."""
    if not isinstance(value, dict):
        return {prefix} if prefix else set()

    keys: set[str] = set()
    for key, child in value.items():
        dotted_key = f"{prefix}.{key}" if prefix else key
        keys.update(flatten_keys(child, dotted_key))
    return keys


def is_translation_key(key: str) -> bool:
    """Accept only namespaced static keys like common.save or auth.login."""
    return "." in key


def extract_used_keys() -> set[str]:
    """Collect static translation keys from JSX files."""
    keys: set[str] = set()

    for source_dir in SOURCE_DIRS:
        for path in source_dir.rglob("*.jsx"):
            content = path.read_text(encoding="utf-8")
            for match in TRANSLATION_CALL_PATTERN.finditer(content):
                key = match.group("key").strip()
                if is_translation_key(key):
                    keys.add(key)

    return keys


def load_locale_keys(path: Path) -> set[str]:
    data = json.loads(path.read_text(encoding="utf-8"))
    return flatten_keys(data)


def print_key_list(title: str, keys: set[str]) -> None:
    print(f"{title} ({len(keys)}):")
    for key in sorted(keys):
        print(f"  - {key}")


def main() -> int:
    used_keys = extract_used_keys()
    issues_found = False

    print(f"Project root: {PROJECT_ROOT}")
    print(f"Static translation keys used in code: {len(used_keys)}")

    for locale_file in LOCALE_FILES:
        locale_keys = load_locale_keys(locale_file)
        missing_keys = used_keys - locale_keys
        orphaned_keys = locale_keys - used_keys

        print()
        print(f"Locale: {locale_file.relative_to(PROJECT_ROOT)}")
        print(f"  Defined keys: {len(locale_keys)}")
        print(f"  Missing keys: {len(missing_keys)}")
        print(f"  Orphaned keys: {len(orphaned_keys)}")

        if missing_keys:
            issues_found = True
            print_key_list("  Missing", missing_keys)

        # Orphaned keys are informational only (some keys used by backend)
        # if orphaned_keys:
        #     print_key_list("  Orphaned", orphaned_keys)

    if issues_found:
        print()
        print("i18n completeness check failed.")
        return 1

    print()
    print("i18n completeness check passed.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
