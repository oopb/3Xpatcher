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
        raise SystemExit(f"v15 {label}: expected exactly one anchor, found {count}")
    path.write_text(text.replace(old, new, 1), encoding="utf-8")


# 3x-ui v3.8.x added native TUIC. Preserve it and append only the
# supplemental protocols owned by 3Xpatcher. This pre-normalization lets the
# older idempotent V2 patch step recognize the already-applied final form.
replace_once(
    "internal/database/model/model.go",
    'validate:"required,oneof=vmess vless trojan shadowsocks wireguard hysteria http mixed tunnel tun mtproto amneziawg tuic"',
    'validate:"required,oneof=vmess vless trojan shadowsocks wireguard hysteria http mixed tunnel tun mtproto amneziawg tuic anytls shadowtls naive"',
    "v3.8 native TUIC protocol validation",
)

# v3.8.x excludes native TUIC from Xray config generation explicitly. 3Xpatcher
# owns TUIC through the supplemental protocol dispatcher, so fold the new
# upstream TUIC branch into the same IsSingboxProtocol guard used by V2.
replace_once(
    "internal/web/service/xray.go",
    "\t\tif inbound.Protocol == model.MTProto || inbound.Protocol == model.AmneziaWG || inbound.Protocol == model.TUIC {\n\t\t\tcontinue\n\t\t}\n",
    "\t\tif inbound.Protocol == model.MTProto || inbound.Protocol == model.AmneziaWG || model.IsSingboxProtocol(inbound.Protocol) {\n\t\t\tcontinue\n\t\t}\n",
    "v3.8 native TUIC Xray exclusion",
)

# Local AddUser/RemoveUser also gained an explicit TUIC bypass. Supplemental
# dispatch must run first so TUIC changes reconcile the sing-box runtime just
# like AnyTLS/ShadowTLS/Naive instead of silently returning through the native
# TUIC branch.
for fn in ("AddUser", "RemoveUser"):
    if fn == "AddUser":
        sig = "func (l *Local) AddUser(_ context.Context, ib *model.Inbound, userMap map[string]any) error {\n"
    else:
        sig = "func (l *Local) RemoveUser(_ context.Context, ib *model.Inbound, email string) error {\n"
    old = sig + "\tif ib.Protocol == model.MTProto || ib.Protocol == model.AmneziaWG || ib.Protocol == model.TUIC {\n"
    new = sig + "\tif model.IsSingboxProtocol(ib.Protocol) {\n\t\treturn sbox.Reconcile()\n\t}\n\tif ib.Protocol == model.MTProto || ib.Protocol == model.AmneziaWG {\n"
    replace_once("internal/web/runtime/local.go", old, new, f"v3.8 native TUIC {fn} bypass")

print("V15 pre-compat normalization applied.")
