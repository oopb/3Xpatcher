#!/usr/bin/env python3
from pathlib import Path
import sys

if len(sys.argv) != 2:
    raise SystemExit(f"Usage: {sys.argv[0]} /path/to/3x-ui-source")

root = Path(sys.argv[1]).resolve()
path = root / "internal/web/service/inbound.go"
text = path.read_text(encoding="utf-8")
old = "\t\tcase \"tuic\":\n\t\t\tif client.ID == \"\" {\n\t\t\t\treturn inbound, false, common.NewError(\"empty client ID\")\n\t\t\t}\n\t\t\tif client.Password == \"\" {\n\t\t\t\treturn inbound, false, common.NewError(\"tuic client requires a password\")\n\t\t\t}\n\t\t\tif client.Email == \"\" {\n\t\t\t\treturn inbound, false, common.NewError(\"empty client email\")\n\t\t\t}\n"
count = text.count(old)
if count != 1:
    raise SystemExit(f"v15 native TUIC inbound validation: expected exactly one anchor, found {count}")
path.write_text(text.replace(old, "", 1), encoding="utf-8")
print("V15 Go compatibility normalization applied.")
