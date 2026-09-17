#!/usr/bin/env python3
from pathlib import Path
import sys

if len(sys.argv) != 2:
    raise SystemExit(f"Usage: {sys.argv[0]} /path/to/3x-ui-source")

root = Path(sys.argv[1]).resolve()


def p(rel: str) -> Path:
    return root / rel


def replace_once(text: str, old: str, new: str, label: str) -> str:
    count = text.count(old)
    if count != 1:
        raise SystemExit(f"v25 {label}: expected exactly one anchor, found {count}")
    return text.replace(old, new, 1)


def component_slice(text: str, start_marker: str, end_marker: str, label: str):
    start = text.find(start_marker)
    if start < 0:
        raise SystemExit(f"v25 {label}: start marker missing")
    end = text.find(end_marker, start + len(start_marker))
    if end < 0:
        raise SystemExit(f"v25 {label}: end marker missing")
    return start, end, text[start:end]


# ---------------------------------------------------------------------------
# Supplemental protocol form layout. Storage stays under settings.*; only the
# UI ownership is split into Protocol / Stream / Advanced.
# ---------------------------------------------------------------------------
rel = "frontend/src/pages/inbounds/form/protocols/singbox.tsx"
path = p(rel)
text = path.read_text(encoding="utf-8")

# AnyTLS: padding is protocol-owned; listener/socket tuning is transport-owned.
start, end, segment = component_slice(
    text,
    "export function AnyTlsFields() {",
    "export function ShadowTlsFields() {",
    "AnyTLS component",
)
if segment.count("      <ListenTuningFields />\n") != 1:
    raise SystemExit("v25 AnyTLS: expected one ListenTuningFields")
segment = segment.replace("      <ListenTuningFields />\n", "", 1)
segment += "export function AnyTlsTransportFields() {\n  return <ListenTuningFields />;\n}\n\n"
text = text[:start] + segment + text[end:]

# Snell: version/obfuscation are protocol-owned; socket tuning moves to Stream.
start, end, segment = component_slice(
    text,
    "export function SnellFields() {",
    "export function NaiveFields() {",
    "Snell component",
)
if segment.count("      <ListenTuningFields />\n") != 1:
    raise SystemExit(f"v25 Snell: expected one ListenTuningFields, found {segment.count('      <ListenTuningFields />' + chr(10))}")
segment = segment.replace("      <ListenTuningFields />\n", "", 1)
segment += "export function SnellTransportFields() {\n  return <ListenTuningFields />;\n}\n\n"
text = text[:start] + segment + text[end:]

# Naive has no remaining protocol-only knobs: network/QUIC/listener controls are
# all transport settings, while TLS/Reality already use the native Security tab.
start, end, segment = component_slice(
    text,
    "export function NaiveFields() {",
    "function MieruPortBindings() {",
    "Naive component",
)
transport_segment = segment.replace(
    "export function NaiveFields() {",
    "export function NaiveTransportFields() {",
    1,
)
segment = "export function NaiveFields() {\n  return null;\n}\n\n" + transport_segment
text = text[:start] + segment + text[end:]

# Mieru: split listener transport, server/user policy, and advanced networking.
text = replace_once(
    text,
    "export function MieruFields() {",
    "export function MieruFields({ section = 'protocol' }: { section?: 'protocol' | 'transport' | 'advanced' } = {}) {",
    "Mieru section signature",
)
text = replace_once(
    text,
    "      <FormField label=\"Primary Transport\" name={['settings', 'transport']}>\n",
    "      {section === 'transport' && (\n        <>\n      <FormField label=\"Primary Transport\" name={['settings', 'transport']}>\n",
    "Mieru transport open",
)
text = replace_once(
    text,
    "      <MieruPortBindings />\n",
    "      <MieruPortBindings />\n        </>\n      )}\n\n      {section === 'protocol' && (\n        <>\n",
    "Mieru transport/protocol boundary",
)
text = replace_once(
    text,
    "      <FormField label=\"Require User Hint\" name={['settings', 'userHintIsMandatory']} valueProp=\"checked\"><Switch /></FormField>\n",
    "      <FormField label=\"Require User Hint\" name={['settings', 'userHintIsMandatory']} valueProp=\"checked\"><Switch /></FormField>\n        </>\n      )}\n\n      {section === 'advanced' && (\n        <>\n",
    "Mieru protocol/advanced boundary",
)
needle = "      <FormField label=\"Client Traffic Pattern\" name={['settings', 'clientTrafficPattern']}>"
pos = text.find(needle)
if pos < 0:
    raise SystemExit("v25 Mieru advanced close: client traffic field missing")
close = text.find("      </FormField>", pos)
if close < 0:
    raise SystemExit("v25 Mieru advanced close: closing FormField missing")
close += len("      </FormField>")
text = text[:close] + "\n        </>\n      )}" + text[close:]

if "export function MieruTransportFields()" in text:
    raise SystemExit("v25 Mieru wrapper exports already exist")
text = text.rstrip() + '''\n\nexport function MieruTransportFields() {\n  return <MieruFields section="transport" />;\n}\n\nexport function MieruAdvancedFields() {\n  return <MieruFields section="advanced" />;\n}\n'''

text = replace_once(
    text,
    "{ value: '', label: 'Default (USE_FIRST_IP)' },",
    "{ value: '', label: 'Default (PREFER_IPv4)' },",
    "Mieru DNS label",
)
path.write_text(text, encoding="utf-8")

# The runtime fallback is already PREFER_IPv4. Make form validation/defaults say
# the same thing instead of presenting an empty value as USE_FIRST_IP.
path = p("frontend/src/schemas/protocols/inbound/singbox.ts")
text = path.read_text(encoding="utf-8")
text = replace_once(
    text,
    "      .default(''),\n    dnsHosts: z.array(MieruDNSHostSchema).default([]),",
    "      .default('PREFER_IPv4'),\n    dnsHosts: z.array(MieruDNSHostSchema).default([]),",
    "Mieru DNS schema default",
)
path.write_text(text, encoding="utf-8")

# V21 already restored native TUIC as its own export. Extend only the sing-box
# supplemental export line and leave the native TuicFields export untouched.
path = p("frontend/src/pages/inbounds/form/protocols/index.ts")
text = path.read_text(encoding="utf-8")
text = replace_once(
    text,
    "export { AnyTlsFields, MieruFields, NaiveFields, ShadowTlsFields, ShadowTlsSecurityFields, SnellFields } from './singbox';",
    "export { AnyTlsFields, AnyTlsTransportFields, MieruFields, MieruTransportFields, MieruAdvancedFields, NaiveFields, NaiveTransportFields, ShadowTlsFields, ShadowTlsSecurityFields, SnellFields, SnellTransportFields } from './singbox';",
    "form exports",
)
path.write_text(text, encoding="utf-8")

# Native modal: supplemental transport reuses the Stream tab slot; Mieru
# networking/egress/traffic/share knobs get a friendly page in Advanced.
path = p("frontend/src/pages/inbounds/form/InboundFormModal.tsx")
text = path.read_text(encoding="utf-8")
for old, new, label in [
    ("  AnyTlsFields,\n", "  AnyTlsFields,\n  AnyTlsTransportFields,\n", "AnyTLS transport import"),
    ("  NaiveFields,\n", "  NaiveTransportFields,\n", "Naive transport import"),
    ("  MieruFields,\n", "  MieruFields,\n  MieruTransportFields,\n  MieruAdvancedFields,\n", "Mieru imports"),
    ("  SnellFields,\n", "  SnellFields,\n  SnellTransportFields,\n", "Snell transport import"),
]:
    text = replace_once(text, old, new, label)

text = replace_once(
    text,
    "      {protocol === Protocols.NAIVE && <NaiveFields />}\n",
    "",
    "remove empty Naive Protocol renderer",
)

text = replace_once(
    text,
    "  if (path[0] === 'settings') return 'protocol';",
    '''  if (path[0] === 'settings') {\n    const key = String(path[1] ?? '');\n    if (\n      [\n        'bindInterface', 'routingMark', 'netns', 'reuseAddr', 'tcpFastOpen', 'tcpMultiPath',\n        'disableTCPKeepAlive', 'tcpKeepAlive', 'tcpKeepAliveInterval', 'udpFragment', 'udpTimeout',\n        'network', 'quicCongestionControl', 'transport', 'portRangeEnd', 'additionalPortBindings',\n      ].includes(key)\n    )\n      return 'stream';\n    if (\n      [\n        'dnsDualStack', 'dnsHosts', 'egressProxies', 'egressRules', 'trafficPatternEnabled',\n        'trafficSeed', 'trafficUnlockAll', 'tcpFragmentEnable', 'tcpFragmentMaxSleepMs', 'nonceType',\n        'nonceApplyToAllUDP', 'nonceMinLen', 'nonceMaxLen', 'nonceCustomHexStrings',\n        'paddingMaxMiddleLen', 'paddingMaxEndLen', 'lowEntropyMode', 'lowEntropyMaskRotation',\n        'clientMultiplexing', 'clientHandshakeMode', 'clientTrafficPattern',\n      ].includes(key)\n    )\n      return 'advanced';\n    return 'protocol';\n  }''',
    "validation tab routing",
)

text = replace_once(
    text,
    "  const tlsOk = canEnableTls({ protocol, streamSettings: { network, security } });",
    '''  const supplementalTransportTab =\n    protocol === Protocols.ANYTLS ? (\n      <AnyTlsTransportFields />\n    ) : protocol === Protocols.NAIVE ? (\n      <NaiveTransportFields />\n    ) : protocol === Protocols.SNELL ? (\n      <SnellTransportFields />\n    ) : protocol === Protocols.MIERU ? (\n      <MieruTransportFields />\n    ) : null;\n\n  const tlsOk = canEnableTls({ protocol, streamSettings: { network, security } });''',
    "supplemental stream content",
)

text = replace_once(
    text,
    '''          items={[\n            {\n              key: 'all',''',
    '''          items={[\n            ...(protocol === Protocols.MIERU\n              ? [\n                  {\n                    key: 'mieru',\n                    label: 'Mieru',\n                    children: <MieruAdvancedFields />,\n                  },\n                ]\n              : []),\n            {\n              key: 'all',''',
    "Mieru advanced editor page",
)

text = replace_once(
    text,
    '''                ...(streamEnabled\n                  ? [''',
    '''                ...(supplementalTransportTab\n                  ? [\n                      {\n                        key: 'stream',\n                        label: t('pages.inbounds.streamTab'),\n                        children: supplementalTransportTab,\n                        forceRender: true,\n                      },\n                    ]\n                  : []),\n                ...(streamEnabled\n                  ? [''',
    "supplemental stream tab",
)

text = replace_once(
    text,
    "                    Protocols.NAIVE,\n",
    "",
    "remove empty Naive Protocol tab",
)
path.write_text(text, encoding="utf-8")

print("V25 supplemental form layout + Mieru default alignment applied.")
