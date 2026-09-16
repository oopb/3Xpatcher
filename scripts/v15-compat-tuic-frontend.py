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
        raise SystemExit(f"v15 tuic frontend {label}: expected exactly one anchor, found {count}")
    path.write_text(text.replace(old, new, 1), encoding="utf-8")


def remove_once(rel: str, old: str, label: str) -> None:
    path = root / rel
    text = path.read_text(encoding="utf-8")
    count = text.count(old)
    if count == 0:
        return
    if count != 1:
        raise SystemExit(f"v15 tuic frontend {label}: expected at most one anchor, found {count}")
    path.write_text(text.replace(old, "", 1), encoding="utf-8")


# v15-pre detaches the native TUIC inbound-settings schema because 3Xpatcher
# keeps TUIC on its established sing-box settings shape. The v3.8 form adapter,
# however, still legitimately reuses the upstream per-client schema. Re-export
# only that client surface without reintroducing the native inbound schema.
replace_once(
    "frontend/src/schemas/protocols/inbound/index.ts",
    "export * from './trojan';\n",
    "export * from './trojan';\nexport { TuicClientSchema } from './tuic';\nexport type { TuicClient } from './tuic';\n",
    "TUIC client-only schema export",
)

# v3.8 also wires its sidecar-specific TuicFields into the native protocol form.
# Remove those three ownership points before v2-patch-frontend installs the
# sing-box TuicFields implementation, otherwise TypeScript sees duplicate names
# and the form would render both native and supplemental TUIC editors.
remove_once(
    "frontend/src/pages/inbounds/form/protocols/index.ts",
    "export { default as TuicFields } from './tuic';\n",
    "native TUIC protocol fields export",
)
remove_once(
    "frontend/src/pages/inbounds/form/InboundFormModal.tsx",
    "  TuicFields,\n",
    "native TUIC protocol fields import",
)
remove_once(
    "frontend/src/pages/inbounds/form/InboundFormModal.tsx",
    "      {protocol === Protocols.TUIC && <TuicFields />}\n",
    "native TUIC protocol fields render",
)

print("V15 TUIC frontend ownership normalization applied.")
