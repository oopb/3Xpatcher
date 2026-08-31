#!/usr/bin/env python3
import sys
from v2_patchlib import Patcher

rep = Patcher(sys.argv[1]).rep

# ---------------------------------------------------------------------------
# Frontend: protocol enum/schema/defaults/form fields inside native Inbounds UI.
# ---------------------------------------------------------------------------
rep(
    'frontend/src/schemas/primitives/protocol.ts',
    '''  'amneziawg',\n]);''',
    '''  'amneziawg',\n  'tuic',\n  'anytls',\n  'shadowtls',\n  'naive',\n]);''',
)
rep(
    'frontend/src/schemas/primitives/protocol.ts',
    '''  AMNEZIAWG: 'amneziawg',\n});''',
    '''  AMNEZIAWG: 'amneziawg',\n  TUIC: 'tuic',\n  ANYTLS: 'anytls',\n  SHADOWTLS: 'shadowtls',\n  NAIVE: 'naive',\n});''',
)

rep(
    'frontend/src/schemas/protocols/inbound/index.ts',
    "import { AmneziawgInboundSettingsSchema } from './amneziawg';\n",
    "import { AmneziawgInboundSettingsSchema } from './amneziawg';\nimport { AnyTlsInboundSettingsSchema, NaiveInboundSettingsSchema, ShadowTlsInboundSettingsSchema, TuicInboundSettingsSchema } from './singbox';\n",
)
rep(
    'frontend/src/schemas/protocols/inbound/index.ts',
    "export * from './shadowsocks';\n",
    "export * from './shadowsocks';\nexport * from './singbox';\n",
)
rep(
    'frontend/src/schemas/protocols/inbound/index.ts',
    '''  z.object({ protocol: z.literal('amneziawg'), settings: AmneziawgInboundSettingsSchema }),\n]);''',
    '''  z.object({ protocol: z.literal('amneziawg'), settings: AmneziawgInboundSettingsSchema }),\n  z.object({ protocol: z.literal('tuic'), settings: TuicInboundSettingsSchema }),\n  z.object({ protocol: z.literal('anytls'), settings: AnyTlsInboundSettingsSchema }),\n  z.object({ protocol: z.literal('shadowtls'), settings: ShadowTlsInboundSettingsSchema }),\n  z.object({ protocol: z.literal('naive'), settings: NaiveInboundSettingsSchema }),\n]);''',
)

# Default settings and factory dispatch.
rep(
    'frontend/src/lib/xray/inbound-defaults.ts',
    "import type { WireguardInboundSettings } from '@/schemas/protocols/inbound/wireguard';\n",
    "import type { WireguardInboundSettings } from '@/schemas/protocols/inbound/wireguard';\nimport type { AnyTlsInboundSettings, NaiveInboundSettings, ShadowTlsInboundSettings, TuicInboundSettings } from '@/schemas/protocols/inbound/singbox';\n",
)
rep(
    'frontend/src/lib/xray/inbound-defaults.ts',
    '''export function createDefaultAmneziawgInboundSettings(): AmneziawgInboundSettings {''',
    '''export function createDefaultTuicInboundSettings(): TuicInboundSettings {\n  return { clients: [], congestionControl: 'cubic', authTimeout: '3s', zeroRTTHandshake: false, heartbeat: '10s' };\n}\n\nexport function createDefaultAnyTlsInboundSettings(): AnyTlsInboundSettings {\n  return { clients: [], paddingScheme: [] };\n}\n\nexport function createDefaultShadowTlsInboundSettings(): ShadowTlsInboundSettings {\n  return {\n    clients: [],\n    version: 3,\n    handshakeServer: 'www.cloudflare.com',\n    handshakePort: 443,\n    strictMode: false,\n    wildcardSNI: 'off',\n    innerMethod: '2022-blake3-aes-128-gcm',\n    innerPassword: RandomUtil.randomShadowsocksPassword('2022-blake3-aes-128-gcm'),\n  };\n}\n\nexport function createDefaultNaiveInboundSettings(): NaiveInboundSettings {\n  return { clients: [], network: '', quicCongestionControl: 'bbr' };\n}\n\nexport function createDefaultAmneziawgInboundSettings(): AmneziawgInboundSettings {''',
)
rep(
    'frontend/src/lib/xray/inbound-defaults.ts',
    '''  | MtprotoInboundSettings\n  | AmneziawgInboundSettings;''',
    '''  | MtprotoInboundSettings\n  | AmneziawgInboundSettings\n  | TuicInboundSettings\n  | AnyTlsInboundSettings\n  | ShadowTlsInboundSettings\n  | NaiveInboundSettings;''',
)
rep(
    'frontend/src/lib/xray/inbound-defaults.ts',
    '''    case 'amneziawg':\n      return createDefaultAmneziawgInboundSettings();\n    default:''',
    '''    case 'amneziawg':\n      return createDefaultAmneziawgInboundSettings();\n    case 'tuic':\n      return createDefaultTuicInboundSettings();\n    case 'anytls':\n      return createDefaultAnyTlsInboundSettings();\n    case 'shadowtls':\n      return createDefaultShadowTlsInboundSettings();\n    case 'naive':\n      return createDefaultNaiveInboundSettings();\n    default:''',
)

rep(
    'frontend/src/pages/inbounds/form/protocols/index.ts',
    "export { default as AmneziawgFields } from './amneziawg';\n",
    "export { default as AmneziawgFields } from './amneziawg';\nexport { AnyTlsFields, NaiveFields, ShadowTlsFields, TuicFields } from './singbox';\n",
)

# Capability flags: no Xray transport/sniffing tabs, but the native TLS editor
# is available for TLS-based supplemental protocols.
rep(
    'frontend/src/lib/xray/protocol-capabilities.ts',
    '''export function canEnableTls(values: CapabilityProtocolSlice): boolean {\n  if (values.protocol === 'hysteria') return true;''',
    '''export function canEnableTls(values: CapabilityProtocolSlice): boolean {\n  if (values.protocol === 'hysteria' || ['tuic', 'anytls', 'naive'].includes(values.protocol)) return true;''',
)
rep(
    'frontend/src/lib/xray/protocol-capabilities.ts',
    '''export function canEnableSniffing(values: { protocol: string }): boolean {\n  return values.protocol !== 'mtproto' && values.protocol !== 'amneziawg';\n}''',
    '''export function canEnableSniffing(values: { protocol: string }): boolean {\n  return !['mtproto', 'amneziawg', 'tuic', 'anytls', 'shadowtls', 'naive'].includes(values.protocol);\n}''',
)

# Inbound modal component imports and protocol-specific fields.
rep(
    'frontend/src/pages/inbounds/form/InboundFormModal.tsx',
    '''  TunFields,\n  TunnelFields,\n  VlessFields,''',
    '''  TunFields,\n  TunnelFields,\n  TuicFields,\n  AnyTlsFields,\n  ShadowTlsFields,\n  NaiveFields,\n  VlessFields,''',
)
rep(
    'frontend/src/pages/inbounds/form/InboundFormModal.tsx',
    '''const SHARE_ADDR_STRATEGIES = ['node', 'listen', 'custom'] as const;''',
    '''const SHARE_ADDR_STRATEGIES = ['node', 'listen', 'custom'] as const;\nconst SINGBOX_TLS_PROTOCOLS = [Protocols.TUIC, Protocols.ANYTLS, Protocols.NAIVE] as string[];''',
)

rep(
    'frontend/src/pages/inbounds/form/InboundFormModal.tsx',
    "import { createHysteriaTlsSettingsWithDefaultCert } from '@/lib/xray/inbound-tls-defaults';\n",
    "import { createHysteriaTlsSettingsWithDefaultCert, createTlsSettingsWithDefaultCert } from '@/lib/xray/inbound-tls-defaults';\n",
)

# Protocol reset: seed native TLS settings for TUIC/AnyTLS/Naive and no Xray
# transport for ShadowTLS.
rep(
    'frontend/src/pages/inbounds/form/InboundFormModal.tsx',
    '''      } else if (next === Protocols.WIREGUARD || next === Protocols.TUNNEL) {\n        setV('streamSettings', { security: 'none' });\n      } else {''',
    '''      } else if (SINGBOX_TLS_PROTOCOLS.includes(next)) {\n        setV('streamSettings', {\n          security: 'tls',\n          tlsSettings: createTlsSettingsWithDefaultCert(),\n        });\n      } else if (next === Protocols.SHADOWTLS) {\n        setV('streamSettings', { security: 'none' });\n      } else if (next === Protocols.WIREGUARD || next === Protocols.TUNNEL) {\n        setV('streamSettings', { security: 'none' });\n      } else {''',
)

rep(
    'frontend/src/pages/inbounds/form/InboundFormModal.tsx',
    '''      {protocol === Protocols.MTPROTO && <MtprotoFields />}\n\n      {protocol === Protocols.SHADOWSOCKS && <ShadowsocksFields isSSWith2022={isSSWith2022} />}''',
    '''      {protocol === Protocols.MTPROTO && <MtprotoFields />}\n      {protocol === Protocols.TUIC && <TuicFields />}\n      {protocol === Protocols.ANYTLS && <AnyTlsFields />}\n      {protocol === Protocols.SHADOWTLS && <ShadowTlsFields />}\n      {protocol === Protocols.NAIVE && <NaiveFields />}\n\n      {protocol === Protocols.SHADOWSOCKS && <ShadowsocksFields isSSWith2022={isSSWith2022} />}''',
)

rep(
    'frontend/src/pages/inbounds/form/InboundFormModal.tsx',
    '''  const tlsOnly = protocol === Protocols.HYSTERIA;''',
    '''  const tlsOnly = protocol === Protocols.HYSTERIA || SINGBOX_TLS_PROTOCOLS.includes(protocol);''',
)

# Protocol tab must appear for all supplemental protocols.
rep(
    'frontend/src/pages/inbounds/form/InboundFormModal.tsx',
    '''                    Protocols.AMNEZIAWG,\n                  ] as string[]''',
    '''                    Protocols.AMNEZIAWG,\n                    Protocols.TUIC,\n                    Protocols.ANYTLS,\n                    Protocols.SHADOWTLS,\n                    Protocols.NAIVE,\n                  ] as string[]''',
)

# Native stream tab stays hidden for supplemental protocols, but TLS protocols
# get the native security/TLS tab as a standalone tab.
rep(
    'frontend/src/pages/inbounds/form/InboundFormModal.tsx',
    '''                ...(sniffingSupported\n                  ? [''',
    '''                ...(SINGBOX_TLS_PROTOCOLS.includes(protocol)\n                  ? [\n                      {\n                        key: 'security',\n                        label: t('pages.inbounds.securityTab'),\n                        children: securityTab,\n                        forceRender: true,\n                      },\n                    ]\n                  : []),\n                ...(sniffingSupported\n                  ? [''',
)
