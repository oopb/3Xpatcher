#!/usr/bin/env python3
from pathlib import Path
import re
import sys

if len(sys.argv) != 2:
    raise SystemExit(f"Usage: {sys.argv[0]} /path/to/3x-ui-source")

root = Path(sys.argv[1]).resolve()


def read(rel):
    return (root / rel).read_text(encoding="utf-8")


def write(rel, text):
    (root / rel).write_text(text, encoding="utf-8")


def replace_once(rel, old, new, label):
    text = read(rel)
    count = text.count(old)
    if count != 1:
        raise SystemExit(f"v21 {label}: expected exactly one anchor, found {count}")
    write(rel, text.replace(old, new, 1))


# ---------------------------------------------------------------------------
# Native TUIC schema/UI is authoritative again. Add only a runtime selector.
# ---------------------------------------------------------------------------
replace_once(
    "frontend/src/schemas/protocols/inbound/tuic.ts",
    "export const TuicInboundSettingsSchema = z.object({\n",
    "export const TuicInboundSettingsSchema = z.object({\n  runtime: z.enum(['native', 'singbox']).default('native'),\n",
    "native TUIC runtime schema",
)
replace_once(
    "frontend/src/lib/xray/inbound-defaults.ts",
    "export function createDefaultTuicInboundSettings(): TuicInboundSettings {\n  return {\n    server:",
    "export function createDefaultTuicInboundSettings(): TuicInboundSettings {\n  return {\n    runtime: 'native',\n    server:",
    "native TUIC runtime default",
)

path = root / "frontend/src/pages/inbounds/form/protocols/tuic.tsx"
text = path.read_text(encoding="utf-8")
if "Segmented," not in text:
    text = text.replace("  Select,\n", "  Select,\n  Segmented,\n", 1)
anchor = "  return (\n    <>\n      <Form.Item label={t('pages.xray.tuic.sni')}>"
insert = """  return (\n    <>\n      <FormField name={['settings', 'runtime']} label=\"Runtime\">\n        <Segmented\n          block\n          options={[\n            { label: 'Native', value: 'native' },\n            { label: 'sing-box', value: 'singbox' },\n          ]}\n        />\n      </FormField>\n\n      <Form.Item label={t('pages.xray.tuic.sni')}>"""
if anchor not in text:
    raise SystemExit("v21 native TUIC runtime control: render anchor not found")
text = text.replace(anchor, insert, 1)
path.write_text(text, encoding="utf-8")

# Reattach the native TUIC inbound schema to the unified union. Previous
# compatibility layers intentionally detached it in favor of the old sing-box
# shape; V21 reverses only TUIC while keeping the other supplemental protocols.
path = root / "frontend/src/schemas/protocols/inbound/index.ts"
text = path.read_text(encoding="utf-8")
lines = text.splitlines()
out = []
for line in lines:
    if "from './singbox'" in line and "TuicInboundSettingsSchema" in line:
        inner = line[line.find('{') + 1: line.find('}')]
        names = [n.strip() for n in inner.split(',') if n.strip() and n.strip() != 'TuicInboundSettingsSchema']
        line = "import { " + ", ".join(names) + " } from './singbox';"
    if line in ("export { TuicClientSchema } from './tuic';", "export type { TuicClient } from './tuic';"):
        continue
    out.append(line)
text = "\n".join(out) + "\n"
if "import { TuicInboundSettingsSchema } from './tuic';" not in text:
    marker = "import { AmneziawgInboundSettingsSchema } from './amneziawg';\n"
    if marker not in text:
        raise SystemExit("v21 native TUIC schema import anchor missing")
    text = text.replace(marker, marker + "import { TuicInboundSettingsSchema } from './tuic';\n", 1)
if "export * from './tuic';" not in text:
    marker = "export * from './trojan';\n"
    if marker not in text:
        raise SystemExit("v21 native TUIC schema export anchor missing")
    text = text.replace(marker, marker + "export * from './tuic';\n", 1)
path.write_text(text, encoding="utf-8")

# Re-export native TuicFields while keeping AnyTLS/ShadowTLS/Naive/Mieru/Snell
# on the supplemental editor.
path = root / "frontend/src/pages/inbounds/form/protocols/index.ts"
text = path.read_text(encoding="utf-8")
lines = text.splitlines()
out = []
for line in lines:
    if "from './singbox'" in line and "TuicFields" in line:
        inner = line[line.find('{') + 1: line.find('}')]
        names = [n.strip() for n in inner.split(',') if n.strip() and n.strip() != 'TuicFields']
        line = "export { " + ", ".join(names) + " } from './singbox';"
    out.append(line)
text = "\n".join(out) + "\n"
if "export { default as TuicFields } from './tuic';" not in text:
    text += "export { default as TuicFields } from './tuic';\n"
path.write_text(text, encoding="utf-8")

# ---------------------------------------------------------------------------
# Runtime ownership: native rows stay on 3x-ui's tuic-server; sing-box rows are
# owned by x-ui-singbox. Switching runtime tears down the previous owner first.
# ---------------------------------------------------------------------------
path = root / "internal/singbox/integrated.go"
text = path.read_text(encoding="utf-8")
anchor = "\tfor i := range rows {\n\t\trec, active, err := integratedRecord(db, &rows[i])"
replacement = "\tfor i := range rows {\n\t\tif rows[i].Protocol == model.TUIC && model.TUICRuntimeFromSettings(rows[i].Settings) != model.TUICRuntimeSingbox {\n\t\t\tcontinue\n\t\t}\n\t\trec, active, err := integratedRecord(db, &rows[i])"
if anchor not in text:
    raise SystemExit("v21 integrated TUIC ownership anchor missing")
text = text.replace(anchor, replacement, 1)
old = "\t\tif err := installTLSFromStream(settings, inbound.StreamSettings); err != nil {"
new = "\t\tif err := installTUICRuntimeTLS(settings, inbound.StreamSettings); err != nil {"
idx = text.find("\tcase model.TUIC:")
if idx < 0:
    raise SystemExit("v21 integrated TUIC case missing")
pos = text.find(old, idx)
if pos < 0:
    raise SystemExit("v21 integrated TUIC TLS anchor missing")
text = text[:pos] + text[pos:].replace(old, new, 1)
path.write_text(text, encoding="utf-8")

path = root / "internal/web/runtime/local.go"
text = path.read_text(encoding="utf-8")
text = text.replace("model.IsSingboxProtocol(ib.Protocol)", "model.IsSingboxOwnedInbound(ib)")
text = text.replace("oldSingbox := model.IsSingboxProtocol(oldIb.Protocol)", "oldSingbox := model.IsSingboxOwnedInbound(oldIb)")
text = text.replace("newSingbox := model.IsSingboxProtocol(newIb.Protocol)", "newSingbox := model.IsSingboxOwnedInbound(newIb)")
needle = "\tif ib.Protocol == model.MTProto || ib.Protocol == model.AmneziaWG {\n\t\treturn nil\n\t}\n"
if text.count(needle) != 2:
    raise SystemExit(f"v21 native TUIC AddUser/RemoveUser guard expected 2 anchors, found {text.count(needle)}")
text = text.replace(needle, "\tif ib.Protocol == model.MTProto || ib.Protocol == model.AmneziaWG || ib.Protocol == model.TUIC {\n\t\treturn nil\n\t}\n")
path.write_text(text, encoding="utf-8")

path = root / "internal/web/service/inbound_tuic.go"
text = path.read_text(encoding="utf-8")
anchor = "\tfor _, ib := range inbounds {\n\t\tinst, ok := tuic.InstanceFromInbound(ib)"
replacement = "\tfor _, ib := range inbounds {\n\t\tif model.TUICRuntimeFromSettings(ib.Settings) == model.TUICRuntimeSingbox {\n\t\t\tcontinue\n\t\t}\n\t\tinst, ok := tuic.InstanceFromInbound(ib)"
if anchor not in text:
    raise SystemExit("v21 native TUIC scheduler filter anchor missing")
path.write_text(text.replace(anchor, replacement, 1), encoding="utf-8")

path = root / "internal/sub/json_service.go"
text = path.read_text(encoding="utf-8")
text = text.replace(
    "model.IsSupplementalProtocol(inbound.Protocol)",
    "model.IsMieruProtocol(inbound.Protocol) || model.IsSingboxOwnedInbound(inbound)",
)
path.write_text(text, encoding="utf-8")

# By V13 the supplemental raw-link switch also contains Snell. Split TUIC out
# while preserving Snell and all other supplemental renderers.
path = root / "internal/sub/service.go"
text = path.read_text(encoding="utf-8")
old = 'case "tuic", "anytls", "shadowtls", "naive", "snell":\n\t\treturn s.genSingboxLink(inbound, email)'
new = 'case "tuic":\n\t\tif model.TUICRuntimeFromSettings(inbound.Settings) == model.TUICRuntimeNative {\n\t\t\treturn s.genTuicLink(inbound, email)\n\t\t}\n\t\treturn s.genSingboxLink(inbound, email)\n\tcase "anytls", "shadowtls", "naive", "snell":\n\t\treturn s.genSingboxLink(inbound, email)'
if old not in text:
    raise SystemExit("v21 raw TUIC subscription dispatcher anchor missing")
path.write_text(text.replace(old, new, 1), encoding="utf-8")

path = root / "internal/sub/clash_service.go"
text = path.read_text(encoding="utf-8")
old = "\tif model.IsSingboxProtocol(inbound.Protocol) {\n\t\treturn s.buildSingboxProxy(subReq, inbound, client, stream, ep)\n\t}\n"
new = "\tif inbound.Protocol == model.TUIC && model.TUICRuntimeFromSettings(inbound.Settings) == model.TUICRuntimeNative {\n\t\treturn s.buildTuicProxy(subReq, inbound, client, ep)\n\t}\n\tif model.IsSingboxOwnedInbound(inbound) {\n\t\treturn s.buildSingboxProxy(subReq, inbound, client, stream, ep)\n\t}\n"
if old not in text:
    raise SystemExit("v21 Clash TUIC dispatcher anchor missing")
path.write_text(text.replace(old, new, 1), encoding="utf-8")

path = root / "internal/sub/singbox_links.go"
text = path.read_text(encoding="utf-8")
old = "\t\tif v, _ := settings[\"congestionControl\"].(string); v != \"\" {\n\t\t\tparams[\"congestion_control\"] = v\n\t\t}\n\t\tif v, _ := settings[\"zeroRTTHandshake\"].(bool); v {\n\t\t\tparams[\"zero_rtt_handshake\"] = \"1\"\n\t\t}\n"
new = "\t\tif !applyNativeTUICLinkSettings(settings, params) {\n\t\t\tif v, _ := settings[\"congestionControl\"].(string); v != \"\" {\n\t\t\t\tparams[\"congestion_control\"] = v\n\t\t\t}\n\t\t\tif v, _ := settings[\"zeroRTTHandshake\"].(bool); v {\n\t\t\t\tparams[\"zero_rtt_handshake\"] = \"1\"\n\t\t\t}\n\t\t}\n"
if old not in text:
    raise SystemExit("v21 sing-box TUIC link settings anchor missing")
path.write_text(text.replace(old, new, 1), encoding="utf-8")

path = root / "internal/sub/singbox_clash.go"
text = path.read_text(encoding="utf-8")
old = "\t\tapplyTLS()\n\t\tproxy[\"skip-cert-verify\"] = true\n\t\treturn proxy\n"
new = "\t\tif !applyNativeTUICClashSettings(settings, proxy) {\n\t\t\tapplyTLS()\n\t\t\tproxy[\"skip-cert-verify\"] = true\n\t\t}\n\t\treturn proxy\n"
if old not in text:
    raise SystemExit("v21 sing-box TUIC Clash settings anchor missing")
path.write_text(text.replace(old, new, 1), encoding="utf-8")

path = root / "internal/web/service/supplemental_tls.go"
text = path.read_text(encoding="utf-8")
text = text.replace("!model.IsSingboxProtocol(inbound.Protocol)", "!model.IsSingboxOwnedInbound(inbound)")
path.write_text(text, encoding="utf-8")

print("V21 TUIC native/sing-box runtime selector applied.")
