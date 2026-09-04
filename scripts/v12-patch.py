#!/usr/bin/env python3
from pathlib import Path
import sys

if len(sys.argv) != 2:
    raise SystemExit("usage: v12-patch.py /path/to/3x-ui-source")

root = Path(sys.argv[1])


def rep(rel: str, old: str, new: str) -> None:
    path = root / rel
    text = path.read_text(encoding="utf-8")
    if old not in text:
        raise SystemExit(f"v12 patch anchor missing in {rel}: {old[:120]!r}")
    path.write_text(text.replace(old, new, 1), encoding="utf-8")


# Keep the raw subscription client identity request-scoped.  The ordinary
# panel QR/export path remains standards-oriented, while clients with known
# import quirks can receive a format they actually understand.
rep(
    "internal/sub/service.go",
    "\tsubscriptionBody bool\n\t// usageShown emits info once per subscription identity, including twins.\n",
    "\tsubscriptionBody bool\n\t// clientUserAgent is populated only for the raw /sub HTTP request. It is\n"
    "\t// intentionally request-scoped so panel QR/export and Clash/JSON output\n"
    "\t// keep their native formats.\n"
    "\tclientUserAgent string\n"
    "\t// usageShown emits info once per subscription identity, including twins.\n",
)
rep(
    "internal/sub/service.go",
    "\ts.address = host\n\ts.usageShown = map[string]bool{}\n",
    "\ts.address = host\n\ts.clientUserAgent = \"\"\n\ts.usageShown = map[string]bool{}\n",
)
rep(
    "internal/sub/controller.go",
    "\tsubReq := a.subService.ForRequest(host)\n\tsubReq.subscriptionBody = true\n\tsubs, _, _, traffic, err := subReq.getSubs(subId)\n",
    "\tsubReq := a.subService.ForRequest(host)\n\tsubReq.subscriptionBody = true\n\tsubReq.clientUserAgent = userAgent\n\tsubs, _, _, traffic, err := subReq.getSubs(subId)\n",
)

print("v12 client compatibility patch applied")
