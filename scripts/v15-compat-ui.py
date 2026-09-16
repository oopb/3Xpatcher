#!/usr/bin/env python3
from pathlib import Path
import sys

if len(sys.argv) != 2:
    raise SystemExit(f"Usage: {sys.argv[0]} /path/to/3x-ui-source")

root = Path(sys.argv[1]).resolve()


def replace_once(rel: str, old: str, new: str, label: str) -> None:
    path = root / rel
    text = path.read_text(encoding="utf-8")
    if new in text and old not in text:
        return
    count = text.count(old)
    if count != 1:
        raise SystemExit(f"v15 ui {label}: expected exactly one anchor, found {count}")
    path.write_text(text.replace(old, new, 1), encoding="utf-8")


# v3.8.x includes native TUIC among client-attachable protocols. Preserve TUIC
# and append only the additional supplemental protocols owned by 3Xpatcher.
for rel in (
    "frontend/src/pages/clients/ClientFormModal.tsx",
    "frontend/src/pages/clients/ClientBulkAddModal.tsx",
):
    replace_once(
        rel,
        "  'amneziawg',\n  'tuic',\n]);",
        "  'amneziawg',\n  'tuic',\n  'anytls',\n  'shadowtls',\n  'naive',\n]);",
        f"{rel} attachable protocols",
    )

print("V15 frontend compatibility normalization applied.")
