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


def remove_once(rel: str, old: str, label: str) -> None:
    path = root / rel
    text = path.read_text(encoding="utf-8")
    count = text.count(old)
    if count == 0:
        return
    if count != 1:
        raise SystemExit(f"v15 {label}: expected at most one anchor, found {count}")
    path.write_text(text.replace(old, "", 1), encoding="utf-8")


# 3x-ui v3.8.x added native TUIC. Preserve its canonical protocol constant but
# extend the validator with the additional protocols owned by 3Xpatcher.
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

# Subscription SQL gained native TUIC in v3.8.x. Keep it and extend the same
# relationship query with 3Xpatcher's additional sing-box protocols.
replace_once(
    "internal/sub/service.go",
    "inbounds.protocol in ('vmess','vless','trojan','shadowsocks','hysteria','wireguard','amneziawg','mtproto','tuic')",
    "inbounds.protocol in ('vmess','vless','trojan','shadowsocks','hysteria','wireguard','amneziawg','mtproto','tuic','anytls','shadowtls','naive')",
    "v3.8 subscription TUIC protocol list",
)

# Frontend protocol enum/map already contains TUIC in v3.8.x. Normalize directly
# to the V2 final form so the legacy patch remains idempotent and only adds the
# protocols that are still absent upstream.
replace_once(
    "frontend/src/schemas/primitives/protocol.ts",
    "  'amneziawg',\n  'tuic',\n]);",
    "  'amneziawg',\n  'tuic',\n  'anytls',\n  'shadowtls',\n  'naive',\n]);",
    "v3.8 frontend protocol enum",
)
replace_once(
    "frontend/src/schemas/primitives/protocol.ts",
    "  AMNEZIAWG: 'amneziawg',\n  TUIC: 'tuic',\n});",
    "  AMNEZIAWG: 'amneziawg',\n  TUIC: 'tuic',\n  ANYTLS: 'anytls',\n  SHADOWTLS: 'shadowtls',\n  NAIVE: 'naive',\n});",
    "v3.8 frontend protocol map",
)

# 3x-ui v3.8 ships a native TUIC frontend schema whose settings shape targets
# its TUIC sidecar. 3Xpatcher intentionally keeps its established sing-box TUIC
# settings shape, so detach the native schema from the unified inbound union;
# v2-patch-frontend then wires TUIC + the other supplemental schemas from
# ./singbox. The upstream tuic.ts file itself is left untouched.
remove_once(
    "frontend/src/schemas/protocols/inbound/index.ts",
    "import { TuicInboundSettingsSchema } from './tuic';\n",
    "v3.8 native TUIC schema import",
)
remove_once(
    "frontend/src/schemas/protocols/inbound/index.ts",
    "export * from './tuic';\n",
    "v3.8 native TUIC schema export",
)
replace_once(
    "frontend/src/schemas/protocols/inbound/index.ts",
    "  z.object({ protocol: z.literal('amneziawg'), settings: AmneziawgInboundSettingsSchema }),\n  z.object({ protocol: z.literal('tuic'), settings: TuicInboundSettingsSchema }),\n]);",
    "  z.object({ protocol: z.literal('amneziawg'), settings: AmneziawgInboundSettingsSchema }),\n]);",
    "v3.8 native TUIC schema union",
)

# Sniffing already excludes native TUIC in v3.8. Normalize to the final
# supplemental exclusion set expected by 3Xpatcher.
replace_once(
    "frontend/src/lib/xray/protocol-capabilities.ts",
    "export function canEnableSniffing(values: { protocol: string }): boolean {\n  return (\n    values.protocol !== 'mtproto' && values.protocol !== 'amneziawg' && values.protocol !== 'tuic'\n  );\n}",
    "export function canEnableSniffing(values: { protocol: string }): boolean {\n  return !['mtproto', 'amneziawg', 'tuic', 'anytls', 'shadowtls', 'naive'].includes(values.protocol);\n}",
    "v3.8 TUIC sniffing capability",
)

print("V15 pre-compat normalization applied.")
