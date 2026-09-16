#!/usr/bin/env python3
from pathlib import Path
import sys

if len(sys.argv) != 2:
    raise SystemExit(f"Usage: {sys.argv[0]} /path/to/3x-ui-source")

root = Path(sys.argv[1]).resolve()


def read(rel: str) -> str:
    return (root / rel).read_text(encoding="utf-8")


def write(rel: str, text: str) -> None:
    (root / rel).write_text(text, encoding="utf-8")


def replace_once(rel: str, old: str, new: str, label: str) -> None:
    text = read(rel)
    count = text.count(old)
    if count != 1:
        raise SystemExit(f"v24 {label}: expected exactly one anchor, found {count}")
    write(rel, text.replace(old, new, 1))


# ---------------------------------------------------------------------------
# Supplemental form layout: keep protocol-owned data under settings.*, but do
# not present transport/socket/routing controls as if they were protocol fields.
# ---------------------------------------------------------------------------
rel = "frontend/src/pages/inbounds/form/protocols/singbox.tsx"

replace_once(
    rel,
    '''export function AnyTlsFields() {\n  return (\n    <>\n      <FormField label="Padding Scheme" name={['settings', 'paddingScheme']}>\n        <Select mode="tags" tokenSeparators={[',']} placeholder="Leave empty for sing-box defaults" style={{ width: '100%' }} />\n      </FormField>\n      <ListenTuningFields />\n    </>\n  );\n}''',
    '''export function AnyTlsFields() {\n  return (\n    <FormField label="Padding Scheme" name={['settings', 'paddingScheme']}>\n      <Select mode="tags" tokenSeparators={[',']} placeholder="Leave empty for sing-box defaults" style={{ width: '100%' }} />\n    </FormField>\n  );\n}\n\nexport function AnyTlsTransportFields() {\n  return <ListenTuningFields />;\n}''',
    "AnyTLS protocol/transport split",
)

replace_once(
    rel,
    '''export function NaiveFields() {\n  return (\n    <>\n      <FormField label="Network" name={['settings', 'network']}><Select options={[{ value: '', label: 'TCP + UDP' }, { value: 'tcp', label: 'TCP' }, { value: 'udp', label: 'UDP / QUIC' }]} /></FormField>\n      <FormField label="QUIC Congestion Control" name={['settings', 'quicCongestionControl']}><Select options={['bbr', 'cubic', 'reno'].map((value) => ({ value, label: value }))} /></FormField>\n      <ListenTuningFields />\n    </>\n  );\n}''',
    '''export function NaiveFields() {\n  return null;\n}\n\nexport function NaiveTransportFields() {\n  return (\n    <>\n      <FormField label="Network" name={['settings', 'network']}><Select options={[{ value: '', label: 'TCP + UDP' }, { value: 'tcp', label: 'TCP' }, { value: 'udp', label: 'UDP / QUIC' }]} /></FormField>\n      <FormField label="QUIC Congestion Control" name={['settings', 'quicCongestionControl']}><Select options={['bbr', 'cubic', 'reno'].map((value) => ({ value, label: value }))} /></FormField>\n      <ListenTuningFields />\n    </>\n  );\n}''',
    "Naive transport split",
)

replace_once(
    rel,
    '''export function SnellFields() {\n  const { control } = useFormContext();\n  const obfsMode = useWatch({ control, name: 'settings.obfsMode' }) as string | undefined;\n  return (\n    <>\n      <Alert\n        type="info"\n        showIcon\n        title="Snell v5 compatibility mode"\n        description="One active 3x-ui client is allowed per Snell inbound. That client's generated password is used directly as the Snell PSK so Clash Verge/Mihomo and Surge-compatible clients can connect without sing-box-only userkey support."\n        style={{ marginBottom: 12 }}\n      />\n      <FormField label="Version" name={['settings', 'version']}><InputNumber value={5} disabled style={{ width: '100%' }} /></FormField>\n      <FormField label="HTTP Obfuscation" name={['settings', 'obfsMode']}>\n        <Select options={[{ value: 'none', label: 'None' }, { value: 'http', label: 'HTTP' }]} />\n      </FormField>\n      {obfsMode === 'http' && (\n        <FormField label="Obfs Host" name={['settings', 'obfsHost']}><Input placeholder="bing.com" /></FormField>\n      )}\n      <ListenTuningFields />\n    </>\n  );\n}''',
    '''export function SnellFields() {\n  const { control } = useFormContext();\n  const obfsMode = useWatch({ control, name: 'settings.obfsMode' }) as string | undefined;\n  return (\n    <>\n      <Alert\n        type="info"\n        showIcon\n        title="Snell v5 compatibility mode"\n        description="One active 3x-ui client is allowed per Snell inbound. That client's generated password is used directly as the Snell PSK so Clash Verge/Mihomo and Surge-compatible clients can connect without sing-box-only userkey support."\n        style={{ marginBottom: 12 }}\n      />\n      <FormField label="Version" name={['settings', 'version']}><InputNumber value={5} disabled style={{ width: '100%' }} /></FormField>\n      <FormField label="HTTP Obfuscation" name={['settings', 'obfsMode']}>\n        <Select options={[{ value: 'none', label: 'None' }, { value: 'http', label: 'HTTP' }]} />\n      </FormField>\n      {obfsMode === 'http' && (\n        <FormField label="Obfs Host" name={['settings', 'obfsHost']}><Input placeholder="bing.com" /></FormField>\n      )}\n    </>\n  );\n}\n\nexport function SnellTransportFields() {\n  return <ListenTuningFields />;\n}''',
    "Snell protocol/transport split",
)

# Mieru is the largest offender: one Protocol page previously contained the
# listener transport, server policy, DNS, routing, traffic pattern, and client
# share defaults. Keep storage unchanged but render those groups separately.
replace_once(
    rel,
    "export function MieruFields() {",
    "export function MieruFields({ section = 'protocol' }: { section?: 'protocol' | 'transport' | 'advanced' } = {}) {",
    "Mieru section parameter",
)

replace_once(
    rel,
    '''      <Alert\n        type="info"\n        showIcon\n        title="Official Mieru / mita runtime"\n        description="Each 3x-ui Mieru inbound runs in an isolated mita instance so attached clients cannot authenticate on other Mieru inbound ports. The main Port field is the primary single port or start of its range."\n        style={{ marginBottom: 12 }}\n      />\n      <FormField label="Primary Transport" name={['settings', 'transport']}>\n        <Select options={[{ value: 'TCP', label: 'TCP (recommended)' }, { value: 'UDP', label: 'UDP' }]} />\n      </FormField>\n      <FormField label="Primary Port Range End" name={['settings', 'portRangeEnd']}>\n        <InputNumber min={0} max={65535} style={{ width: '100%' }} placeholder="0 = single port" />\n      </FormField>\n      <MieruPortBindings />\n\n      <Divider orientation="start" plain>Server</Divider>''',
    '''      {section === 'transport' && (\n        <>\n          <Alert\n            type="info"\n            showIcon\n            title="Official Mieru listener transport"\n            description="The native 3x-ui Port field is the primary Mieru port or range start. TCP/UDP here controls the actual mita listener and exported client transport."\n            style={{ marginBottom: 12 }}\n          />\n          <FormField label="Primary Transport" name={['settings', 'transport']}>\n            <Select options={[{ value: 'TCP', label: 'TCP (recommended)' }, { value: 'UDP', label: 'UDP' }]} />\n          </FormField>\n          <FormField label="Primary Port Range End" name={['settings', 'portRangeEnd']}>\n            <InputNumber min={0} max={65535} style={{ width: '100%' }} placeholder="0 = single port" />\n          </FormField>\n          <MieruPortBindings />\n        </>\n      )}\n\n      {section === 'protocol' && (\n        <>\n      <Divider orientation="start" plain>Server</Divider>''',
    "Mieru transport opening",
)

replace_once(
    rel,
    '''      <FormField label="Require User Hint" name={['settings', 'userHintIsMandatory']} valueProp="checked"><Switch /></FormField>\n\n      <MieruDNSFields />''',
    '''      <FormField label="Require User Hint" name={['settings', 'userHintIsMandatory']} valueProp="checked"><Switch /></FormField>\n        </>\n      )}\n\n      {section === 'advanced' && (\n        <>\n      <MieruDNSFields />''',
    "Mieru protocol/advanced boundary",
)

replace_once(
    rel,
    '''      <FormField label="Client Traffic Pattern" name={['settings', 'clientTrafficPattern']}>\n        <Input placeholder="optional official encoded traffic-pattern value" />\n      </FormField>\n    </>\n  );\n}''',
    '''      <FormField label="Client Traffic Pattern" name={['settings', 'clientTrafficPattern']}>\n        <Input placeholder="optional official encoded traffic-pattern value" />\n      </FormField>\n        </>\n      )}\n    </>\n  );\n}\n\nexport function MieruTransportFields() {\n  return <MieruFields section="transport" />;\n}\n\nexport function MieruAdvancedFields() {\n  return <MieruFields section="advanced" />;\n}''',
    "Mieru advanced closing",
)

# The runtime already treats an empty DNS policy as PREFER_IPv4. Make the form
# say the same thing instead of advertising the old upstream USE_FIRST_IP text.
replace_once(
    rel,
    "{ value: '', label: 'Default (USE_FIRST_IP)' },",
    "{ value: '', label: 'Default (PREFER_IPv4)' },",
    "Mieru DNS default label",
)
replace_once(
    "frontend/src/schemas/protocols/inbound/singbox.ts",
    "      .default(''),\n    dnsHosts: z.array(MieruDNSHostSchema).default([]),",
    "      .default('PREFER_IPv4'),\n    dnsHosts: z.array(MieruDNSHostSchema).default([]),",
    "Mieru DNS schema default",
)

# ---------------------------------------------------------------------------
# Wire the split sections into native 3x-ui tabs. The settings remain settings.*
# so renderer/storage compatibility is unchanged.
# ---------------------------------------------------------------------------
rel = "frontend/src/pages/inbounds/form/protocols/index.ts"
replace_once(
    rel,
    "export { AnyTlsFields, MieruFields, NaiveFields, ShadowTlsFields, ShadowTlsSecurityFields, SnellFields, TuicFields } from './singbox';",
    "export { AnyTlsFields, AnyTlsTransportFields, MieruFields, MieruTransportFields, MieruAdvancedFields, NaiveFields, NaiveTransportFields, ShadowTlsFields, ShadowTlsSecurityFields, SnellFields, SnellTransportFields, TuicFields } from './singbox';",
    "protocol exports",
)

rel = "frontend/src/pages/inbounds/form/InboundFormModal.tsx"
text = read(rel)
for old, new, label in [
    ("  AnyTlsFields,\n", "  AnyTlsFields,\n  AnyTlsTransportFields,\n", "AnyTLS transport import"),
    ("  NaiveFields,\n", "  NaiveTransportFields,\n", "Naive transport import"),
    ("  MieruFields,\n", "  MieruFields,\n  MieruTransportFields,\n  MieruAdvancedFields,\n", "Mieru section imports"),
    ("  SnellFields,\n", "  SnellFields,\n  SnellTransportFields,\n", "Snell transport import"),
]:
    count = text.count(old)
    if count != 1:
        raise SystemExit(f"v24 {label}: expected exactly one anchor, found {count}")
    text = text.replace(old, new, 1)

# Naive has no protocol-only controls after transport and TLS are separated.
old = "      {protocol === Protocols.NAIVE && <NaiveFields />}\n"
if text.count(old) != 1:
    raise SystemExit(f"v24 Naive protocol render: expected exactly one anchor, found {text.count(old)}")
text = text.replace(old, "", 1)

# Validation should take users directly to the tab that owns each settings key.
old = "  if (path[0] === 'settings') return 'protocol';"
new = '''  if (path[0] === 'settings') {\n    const key = String(path[1] ?? '');\n    if (\n      [\n        'bindInterface', 'routingMark', 'netns', 'reuseAddr', 'tcpFastOpen', 'tcpMultiPath',\n        'disableTCPKeepAlive', 'tcpKeepAlive', 'tcpKeepAliveInterval', 'udpFragment', 'udpTimeout',\n        'network', 'quicCongestionControl', 'transport', 'portRangeEnd', 'additionalPortBindings',\n      ].includes(key)\n    )\n      return 'stream';\n    if (\n      [\n        'dnsDualStack', 'dnsHosts', 'egressProxies', 'egressRules', 'trafficPatternEnabled',\n        'trafficSeed', 'trafficUnlockAll', 'tcpFragmentEnable', 'tcpFragmentMaxSleepMs', 'nonceType',\n        'nonceApplyToAllUDP', 'nonceMinLen', 'nonceMaxLen', 'nonceCustomHexStrings',\n        'paddingMaxMiddleLen', 'paddingMaxEndLen', 'lowEntropyMode', 'lowEntropyMaskRotation',\n        'clientMultiplexing', 'clientHandshakeMode', 'clientTrafficPattern',\n      ].includes(key)\n    )\n      return 'advanced';\n    return 'protocol';\n  }'''
if text.count(old) != 1:
    raise SystemExit(f"v24 validation tab routing: expected exactly one anchor, found {text.count(old)}")
text = text.replace(old, new, 1)

# Build a native-looking Stream tab for protocol-owned transport fields. These
# protocols intentionally do not opt into generic Xray streamSettings.
anchor = "  const tlsOk = canEnableTls({ protocol, streamSettings: { network, security } });"
transport = '''  const supplementalTransportTab =\n    protocol === Protocols.ANYTLS ? (\n      <AnyTlsTransportFields />\n    ) : protocol === Protocols.NAIVE ? (\n      <NaiveTransportFields />\n    ) : protocol === Protocols.SNELL ? (\n      <SnellTransportFields />\n    ) : protocol === Protocols.MIERU ? (\n      <MieruTransportFields />\n    ) : null;\n\n'''
if text.count(anchor) != 1:
    raise SystemExit(f"v24 supplemental transport tab anchor: expected exactly one, found {text.count(anchor)}")
text = text.replace(anchor, transport + anchor, 1)

# Mieru gets a friendly first page inside the existing Advanced tab; the raw
# JSON editors remain available beside it.
old = '''          items={[\n            {\n              key: 'all','''
new = '''          items={[\n            ...(protocol === Protocols.MIERU\n              ? [\n                  {\n                    key: 'mieru',\n                    label: 'Mieru',\n                    children: <MieruAdvancedFields />,\n                  },\n                ]\n              : []),\n            {\n              key: 'all','''
if text.count(old) != 1:
    raise SystemExit(f"v24 Mieru advanced tab anchor: expected exactly one, found {text.count(old)}")
text = text.replace(old, new, 1)

# Add the supplemental Stream tab immediately before native streamEnabled tabs.
old = '''                ...(streamEnabled\n                  ? ['''
new = '''                ...(supplementalTransportTab\n                  ? [\n                      {\n                        key: 'stream',\n                        label: t('pages.inbounds.streamTab'),\n                        children: supplementalTransportTab,\n                        forceRender: true,\n                      },\n                    ]\n                  : []),\n                ...(streamEnabled\n                  ? ['''
if text.count(old) != 1:
    raise SystemExit(f"v24 supplemental stream tab list anchor: expected exactly one, found {text.count(old)}")
text = text.replace(old, new, 1)

# Do not show an empty Protocol tab for Naive. Other supplemental protocols
# retain real protocol-owned controls there.
old = "                    Protocols.NAIVE,\n"
if text.count(old) != 1:
    raise SystemExit(f"v24 Naive protocol tab list anchor: expected exactly one, found {text.count(old)}")
text = text.replace(old, "", 1)

write(rel, text)
print("V24 supplemental form layout + Mieru DNS default alignment applied.")
