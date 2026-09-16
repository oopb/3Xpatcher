#!/usr/bin/env python3
import sys
from v2_patchlib import Patcher

p = Patcher(sys.argv[1])
rep = p.rep

# ---------------------------------------------------------------------------
# ShadowTLS: keep protocol-owned fields in settings, but present the handshake
# controls in the native Security tab instead of mixing them into Protocol.
# ShadowTLS does not expose sing-box InboundTLSOptions, so these must NOT be
# rewritten into streamSettings.tlsSettings.
# ---------------------------------------------------------------------------
rep(
    'frontend/src/pages/inbounds/form/protocols/singbox.tsx',
    '''      <FormField label="Version" name={['settings', 'version']}><InputNumber value={3} disabled style={{ width: '100%' }} /></FormField>\n      <FormField label="Handshake Server" name={['settings', 'handshakeServer']}><Input placeholder="www.cloudflare.com" /></FormField>\n      <FormField label="Handshake Port" name={['settings', 'handshakePort']}><InputNumber min={1} max={65535} style={{ width: '100%' }} /></FormField>\n      <FormField label="Handshake by SNI (JSON)" name={['settings', 'handshakeForServerNameJson']}>\n        <Input.TextArea autoSize={{ minRows: 3, maxRows: 8 }} placeholder={'{"example.com":{"server":"example.com","server_port":443}}'} />\n      </FormField>\n      <FormField label="Strict Mode" name={['settings', 'strictMode']} valueProp="checked"><Switch /></FormField>\n      <FormField label="Wildcard SNI" name={['settings', 'wildcardSNI']}><Select options={['off', 'authed', 'all'].map((value) => ({ value, label: value }))} /></FormField>''',
    '''      <FormField label="Version" name={['settings', 'version']}><InputNumber value={3} disabled style={{ width: '100%' }} /></FormField>''',
)

rep(
    'frontend/src/pages/inbounds/form/protocols/singbox.tsx',
    '''export function NaiveFields() {''',
    '''export function ShadowTlsSecurityFields() {\n  return (\n    <>\n      <FormField label="Handshake Server" name={['settings', 'handshakeServer']}><Input placeholder="www.cloudflare.com" /></FormField>\n      <FormField label="Handshake Port" name={['settings', 'handshakePort']}><InputNumber min={1} max={65535} style={{ width: '100%' }} /></FormField>\n      <FormField label="Handshake by SNI (JSON)" name={['settings', 'handshakeForServerNameJson']}>\n        <Input.TextArea autoSize={{ minRows: 3, maxRows: 8 }} placeholder={'{"example.com":{"server":"example.com","server_port":443}}'} />\n      </FormField>\n      <FormField label="Strict Mode" name={['settings', 'strictMode']} valueProp="checked"><Switch /></FormField>\n      <FormField label="Wildcard SNI" name={['settings', 'wildcardSNI']}><Select options={['off', 'authed', 'all'].map((value) => ({ value, label: value }))} /></FormField>\n    </>\n  );\n}\n\nexport function NaiveFields() {''',
)

rep(
    'frontend/src/pages/inbounds/form/protocols/index.ts',
    "export { AnyTlsFields, NaiveFields, ShadowTlsFields, TuicFields } from './singbox';",
    "export { AnyTlsFields, NaiveFields, ShadowTlsFields, ShadowTlsSecurityFields, TuicFields } from './singbox';",
)

rep(
    'frontend/src/pages/inbounds/form/InboundFormModal.tsx',
    '''  ShadowTlsFields,\n  NaiveFields,\n  VlessFields,''',
    '''  ShadowTlsFields,\n  ShadowTlsSecurityFields,\n  NaiveFields,\n  VlessFields,''',
)

# Validation failures for fields moved out of Protocol should open Security.
rep(
    'frontend/src/pages/inbounds/form/InboundFormModal.tsx',
    '''  if (path[0] === 'settings') return 'protocol';''',
    '''  if (\n    path[0] === 'settings' &&\n    ['handshakeServer', 'handshakePort', 'handshakeForServerNameJson', 'strictMode', 'wildcardSNI'].includes(\n      String(path[1] ?? ''),\n    )\n  )\n    return 'security';\n  if (path[0] === 'settings') return 'protocol';''',
)

# Supplemental TLS protocols already get native Security. ShadowTLS gets a
# Security tab too, but with its own protocol-native handshake controls.
rep(
    'frontend/src/pages/inbounds/form/InboundFormModal.tsx',
    '''                ...(SINGBOX_TLS_PROTOCOLS.includes(protocol)\n                  ? [\n                      {\n                        key: 'security',\n                        label: t('pages.inbounds.securityTab'),\n                        children: securityTab,\n                        forceRender: true,\n                      },\n                    ]\n                  : []),''',
    '''                ...(SINGBOX_TLS_PROTOCOLS.includes(protocol)\n                  ? [\n                      {\n                        key: 'security',\n                        label: t('pages.inbounds.securityTab'),\n                        children: securityTab,\n                        forceRender: true,\n                      },\n                    ]\n                  : protocol === Protocols.SHADOWTLS\n                    ? [\n                        {\n                          key: 'security',\n                          label: t('pages.inbounds.securityTab'),\n                          children: <ShadowTlsSecurityFields />,\n                          forceRender: true,\n                        },\n                      ]\n                    : []),''',
)

# ---------------------------------------------------------------------------
# Inbound list: derive every supplemental badge from the stored configuration
# whenever the protocol has a selectable setting. Fixed transport protocols
# still show their real fixed L4 transport.
# ---------------------------------------------------------------------------
rep(
    'frontend/src/pages/inbounds/list/useInboundColumns.tsx',
    '''          const supplementalSettings = coerceInboundJsonField(record.settings) as Record<string, unknown>;\n          if (record.protocol === 'tuic') {\n            tags.push(\n              <Tag key="n" color="green">UDP</Tag>,\n              <Tag key="tls" color="blue">TLS</Tag>,\n            );\n          } else if (record.protocol === 'anytls') {\n            tags.push(\n              <Tag key="n" color="green">TCP</Tag>,\n              <Tag key="tls" color="blue">TLS</Tag>,\n            );\n          } else if (record.protocol === 'shadowtls') {\n            tags.push(\n              <Tag key="n" color="green">TCP</Tag>,\n              <Tag key="tls" color="blue">TLS</Tag>,\n            );\n          } else if (record.protocol === 'naive') {\n            tags.push(\n              <Tag key="n" color="green">TCP</Tag>,\n              <Tag key="tls" color="blue">TLS</Tag>,\n            );\n          } else if (record.protocol === 'snell') {\n            tags.push(\n              <Tag key="n" color="green">TCP</Tag>,\n            );\n          } else if (record.protocol === 'mieru') {\n            const transport = String(supplementalSettings.transport || 'TCP').toUpperCase();\n            tags.push(\n              <Tag key="n" color="green">{transport === 'UDP' ? 'UDP' : 'TCP'}</Tag>,\n            );\n          } else if (record.isWireguard || record.isAmneziawg || record.isHysteria || record.isTuic) {''',
    '''          const supplementalSettings = coerceInboundJsonField(record.settings) as Record<string, unknown>;\n          const supplementalStream = readStreamHints(record.streamSettings);\n          const pushSupplementalSecurity = () => {\n            if (supplementalStream.isReality) {\n              tags.push(<Tag key="reality" color="blue">Reality</Tag>);\n            } else if (supplementalStream.isTls) {\n              tags.push(<Tag key="tls" color="blue">TLS</Tag>);\n            }\n          };\n          if (record.protocol === 'tuic') {\n            tags.push(<Tag key="n" color="green">UDP</Tag>);\n            pushSupplementalSecurity();\n          } else if (record.protocol === 'anytls') {\n            tags.push(<Tag key="n" color="green">TCP</Tag>);\n            pushSupplementalSecurity();\n          } else if (record.protocol === 'shadowtls') {\n            // ShadowTLS owns its handshake security internally; it is not a\n            // generic streamSettings TLS layer. Show the real listener L4 only.\n            tags.push(<Tag key="n" color="green">TCP</Tag>);\n          } else if (record.protocol === 'naive') {\n            const naiveNetwork = String(supplementalSettings.network || '').toLowerCase();\n            if (naiveNetwork === 'tcp') {\n              tags.push(<Tag key="n-tcp" color="green">TCP</Tag>);\n            } else if (naiveNetwork === 'udp') {\n              tags.push(<Tag key="n-udp" color="green">UDP</Tag>);\n            } else {\n              tags.push(\n                <Tag key="n-tcp" color="green">TCP</Tag>,\n                <Tag key="n-udp" color="green">UDP</Tag>,\n              );\n            }\n            pushSupplementalSecurity();\n          } else if (record.protocol === 'snell') {\n            tags.push(<Tag key="n" color="green">TCP</Tag>);\n          } else if (record.protocol === 'mieru') {\n            const transport = String(supplementalSettings.transport || 'TCP').toUpperCase();\n            tags.push(<Tag key="n" color="green">{transport === 'UDP' ? 'UDP' : 'TCP'}</Tag>);\n          } else if (record.isWireguard || record.isAmneziawg || record.isHysteria || record.isTuic) {''',
)

print('V20 dynamic supplemental badges + ShadowTLS Security tab applied.')
