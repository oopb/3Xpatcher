#!/usr/bin/env python3
from pathlib import Path
import sys

if len(sys.argv) != 2:
    raise SystemExit(f"Usage: {sys.argv[0]} /path/to/3x-ui-source")

root = Path(sys.argv[1]).resolve()


def path(rel):
    return root / rel


def replace_once(text: str, old: str, new: str, label: str) -> str:
    count = text.count(old)
    if count != 1:
        raise SystemExit(f"v25 {label}: expected exactly one anchor, found {count}")
    return text.replace(old, new, 1)


# ---------------------------------------------------------------------------
# Split protocol-owned UI by semantic layer without changing storage keys.
# ---------------------------------------------------------------------------
rel = "frontend/src/pages/inbounds/form/protocols/singbox.tsx"
p = path(rel)
text = p.read_text(encoding="utf-8")

text = replace_once(
    text,
    '''export function AnyTlsFields() {\n  return (\n    <>\n      <FormField label="Padding Scheme" name={['settings', 'paddingScheme']}>\n        <Select mode="tags" tokenSeparators={[',']} placeholder="Leave empty for sing-box defaults" style={{ width: '100%' }} />\n      </FormField>\n      <ListenTuningFields />\n    </>\n  );\n}''',
    '''export function AnyTlsFields() {\n  return (\n    <FormField label="Padding Scheme" name={['settings', 'paddingScheme']}>\n      <Select mode="tags" tokenSeparators={[',']} placeholder="Leave empty for sing-box defaults" style={{ width: '100%' }} />\n    </FormField>\n  );\n}\n\nexport function AnyTlsTransportFields() {\n  return <ListenTuningFields />;\n}''',
    "AnyTLS split",
)

text = replace_once(
    text,
    '''export function NaiveFields() {\n  return (\n    <>\n      <FormField label="Network" name={['settings', 'network']}><Select options={[{ value: '', label: 'TCP + UDP' }, { value: 'tcp', label: 'TCP' }, { value: 'udp', label: 'UDP / QUIC' }]} /></FormField>\n      <FormField label="QUIC Congestion Control" name={['settings', 'quicCongestionControl']}><Select options={['bbr', 'cubic', 'reno'].map((value) => ({ value, label: value }))} /></FormField>\n      <ListenTuningFields />\n    </>\n  );\n}''',
    '''export function NaiveFields() {\n  return null;\n}\n\nexport function NaiveTransportFields() {\n  return (\n    <>\n      <FormField label="Network" name={['settings', 'network']}><Select options={[{ value: '', label: 'TCP + UDP' }, { value: 'tcp', label: 'TCP' }, { value: 'udp', label: 'UDP / QUIC' }]} /></FormField>\n      <FormField label="QUIC Congestion Control" name={['settings', 'quicCongestionControl']}><Select options={['bbr', 'cubic', 'reno'].map((value) => ({ value, label: value }))} /></FormField>\n      <ListenTuningFields />\n    </>\n  );\n}''',
    "Naive split",
)

text = replace_once(
    text,
    '''      {obfsMode === 'http' && (\n        <FormField label="Obfs Host" name={['settings', 'obfsHost']}><Input placeholder="bing.com" /></FormField>\n      )}\n      <ListenTuningFields />\n    </>\n  );\n}\n\nexport function NaiveFields()''',
    '''      {obfsMode === 'http' && (\n        <FormField label="Obfs Host" name={['settings', 'obfsHost']}><Input placeholder="bing.com" /></FormField>\n      )}\n    </>\n  );\n}\n\nexport function SnellTransportFields() {\n  return <ListenTuningFields />;\n}\n\nexport function NaiveFields()''',
    "Snell split",
)

text = replace_once(
    text,
    "export function MieruFields() {",
    "export function MieruFields({ section = 'protocol' }: { section?: 'protocol' | 'transport' | 'advanced' } = {}) {",
    "Mieru section signature",
)

# Keep the runtime explanation at the top of each Mieru page, then expose only
# the fields owned by that semantic section.
text = replace_once(
    text,
    '''      <FormField label="Primary Transport" name={['settings', 'transport']}>''',
    '''      {section === 'transport' && (\n        <>\n      <FormField label="Primary Transport" name={['settings', 'transport']}>''',
    "Mieru transport open",
)
text = replace_once(
    text,
    '''      <MieruPortBindings />\n\n      <Divider orientation="start" plain>Server</Divider>''',
    '''      <MieruPortBindings />\n        </>\n      )}\n\n      {section === 'protocol' && (\n        <>\n      <Divider orientation="start" plain>Server</Divider>''',
    "Mieru transport/protocol boundary",
)
text = replace_once(
    text,
    '''      <FormField label="Require User Hint" name={['settings', 'userHintIsMandatory']} valueProp="checked"><Switch /></FormField>\n\n      <MieruDNSFields />''',
    '''      <FormField label="Require User Hint" name={['settings', 'userHintIsMandatory']} valueProp="checked"><Switch /></FormField>\n        </>\n      )}\n\n      {section === 'advanced' && (\n        <>\n      <MieruDNSFields />''',
    "Mieru protocol/advanced boundary",
)
text = replace_once(
    text,
    '''      <FormField label="Client Traffic Pattern" name={['settings', 'clientTrafficPattern']}>\n        <Input placeholder="optional official encoded traffic-pattern value" />\n      </FormField>\n    </>\n  );\n}''',
    '''      <FormField label="Client Traffic Pattern" name={['settings', 'clientTrafficPattern']}>\n        <Input placeholder="optional official encoded traffic-pattern value" />\n      </FormField>\n        </>\n      )}\n    </>\n  );\n}\n\nexport function MieruTransportFields() {\n  return <MieruFields section="transport" />;\n}\n\nexport function MieruAdvancedFields() {\n  return <MieruFields section="advanced" />;\n}''',
    "Mieru advanced close",
)

text = replace_once(
    text,
    "{ value: '', label: 'Default (USE_FIRST_IP)' },",
    "{ value: '', label: 'Default (PREFER_IPv4)' },",
    "Mieru DNS label",
)
p.write_text(text, encoding="utf-8")

# Production backend already defaults empty DNS dual-stack to PREFER_IPv4 via
# V18. Align the schema/default presentation so the UI cannot imply otherwise.
p = path("frontend/src/schemas/protocols/inbound/singbox.ts")
text = p.read_text(encoding="utf-8")
text = replace_once(
    text,
    "      .default(''),\n    dnsHosts: z.array(MieruDNSHostSchema).default([]),",
    "      .default('PREFER_IPv4'),\n    dnsHosts: z.array(MieruDNSHostSchema).default([]),",
    "Mieru DNS schema default",
)
p.write_text(text, encoding="utf-8")

# Export the semantic sub-forms.
p = path("frontend/src/pages/inbounds/form/protocols/index.ts")
text = p.read_text(encoding="utf-8")
text = replace_once(
    text,
    "export { AnyTlsFields, MieruFields, NaiveFields, ShadowTlsFields, ShadowTlsSecurityFields, SnellFields, TuicFields } from './singbox';",
    "export { AnyTlsFields, AnyTlsTransportFields, MieruFields, MieruTransportFields, MieruAdvancedFields, NaiveFields, NaiveTransportFields, ShadowTlsFields, ShadowTlsSecurityFields, SnellFields, SnellTransportFields, TuicFields } from './singbox';",
    "form exports",
)
p.write_text(text, encoding="utf-8")

# Native modal tabs: supplemental transport lives in the Stream tab slot, while
# Mieru DNS/routing/traffic/share controls live in the existing Advanced tab.
p = path("frontend/src/pages/inbounds/form/InboundFormModal.tsx")
text = p.read_text(encoding="utf-8")
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
p.write_text(text, encoding="utf-8")

print("V25 supplemental form layout + Mieru default alignment applied.")
