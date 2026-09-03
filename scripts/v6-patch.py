#!/usr/bin/env python3
import sys
from v2_patchlib import Patcher

rep = Patcher(sys.argv[1]).rep

# Backend protocol acceptance / credentials.
rep('internal/database/model/model.go',
    'oneof=vmess vless trojan shadowsocks wireguard hysteria http mixed tunnel tun mtproto amneziawg tuic anytls shadowtls naive"',
    'oneof=vmess vless trojan shadowsocks wireguard hysteria http mixed tunnel tun mtproto amneziawg tuic anytls shadowtls naive mieru"')
rep('internal/web/service/inbound.go',
    '\t\tmodel.Naive:       true,\n\t}',
    '\t\tmodel.Naive:       true,\n\t\tmodel.Mieru:       true,\n\t}')
rep('internal/web/service/inbound.go',
    'case "anytls", "shadowtls", "naive":',
    'case "anytls", "shadowtls", "naive", "mieru":')
rep('internal/web/service/client_crud.go',
    'case model.AnyTLS, model.ShadowTLS, model.Naive:',
    'case model.AnyTLS, model.ShadowTLS, model.Naive, model.Mieru:')
rep('internal/web/service/client_inbound_apply.go',
    'case "anytls", "shadowtls", "naive":',
    'case "anytls", "shadowtls", "naive", "mieru":')
rep('internal/web/service/client_inbound_apply.go',
    'case "anytls", "shadowtls", "naive":\n\t\tnewClientId = clients[0].Password',
    'case "anytls", "shadowtls", "naive", "mieru":\n\t\tnewClientId = clients[0].Password')
rep('internal/web/service/inbound_clients.go',
    'case model.AnyTLS, model.ShadowTLS, model.Naive:',
    'case model.AnyTLS, model.ShadowTLS, model.Naive, model.Mieru:')

# Xray generation owns neither sing-box nor Mieru. Reconcile both supplemental runtimes.
rep('internal/web/service/xray.go',
    'sbox "github.com/mhsanaei/3x-ui/v3/internal/singbox"\n',
    'mcore "github.com/mhsanaei/3x-ui/v3/internal/mieru"\n\tsbox "github.com/mhsanaei/3x-ui/v3/internal/singbox"\n')
rep('internal/web/service/xray.go',
    '\tif err := sbox.Reconcile(); err != nil {\n\t\tlogger.Warning("supplemental sing-box reconcile failed:", err)\n\t}\n',
    '\tif err := sbox.Reconcile(); err != nil {\n\t\tlogger.Warning("supplemental sing-box reconcile failed:", err)\n\t}\n\tif err := mcore.Reconcile(); err != nil {\n\t\tlogger.Warning("supplemental Mieru reconcile failed:", err)\n\t}\n')
rep('internal/web/service/xray.go',
    'model.IsSingboxProtocol(inbound.Protocol)',
    'model.IsSupplementalProtocol(inbound.Protocol)')

# Local runtime dispatch. Mieru is a separate official mita core, not sing-box.
rep('internal/web/runtime/local.go',
    'sbox "github.com/mhsanaei/3x-ui/v3/internal/singbox"\n',
    'mcore "github.com/mhsanaei/3x-ui/v3/internal/mieru"\n\tsbox "github.com/mhsanaei/3x-ui/v3/internal/singbox"\n')
rep('internal/web/runtime/local.go',
    'func (l *Local) AddInbound(_ context.Context, ib *model.Inbound) error {\n\tif model.IsSingboxProtocol(ib.Protocol) {',
    'func (l *Local) AddInbound(_ context.Context, ib *model.Inbound) error {\n\tif model.IsMieruProtocol(ib.Protocol) {\n\t\treturn mcore.Reconcile()\n\t}\n\tif model.IsSingboxProtocol(ib.Protocol) {')
rep('internal/web/runtime/local.go',
    'func (l *Local) DelInbound(_ context.Context, ib *model.Inbound) error {\n\tif model.IsSingboxProtocol(ib.Protocol) {',
    'func (l *Local) DelInbound(_ context.Context, ib *model.Inbound) error {\n\tif model.IsMieruProtocol(ib.Protocol) {\n\t\treturn mcore.Reconcile()\n\t}\n\tif model.IsSingboxProtocol(ib.Protocol) {')
rep('internal/web/runtime/local.go',
    'func (l *Local) UpdateInbound(ctx context.Context, oldIb, newIb *model.Inbound) error {\n\toldSingbox := model.IsSingboxProtocol(oldIb.Protocol)',
    'func (l *Local) UpdateInbound(ctx context.Context, oldIb, newIb *model.Inbound) error {\n\toldMieru := model.IsMieruProtocol(oldIb.Protocol)\n\tnewMieru := model.IsMieruProtocol(newIb.Protocol)\n\tif oldMieru || newMieru {\n\t\tif oldMieru && newMieru {\n\t\t\treturn mcore.Reconcile()\n\t\t}\n\t\tif oldMieru {\n\t\t\tif err := mcore.Reconcile(); err != nil {\n\t\t\t\treturn err\n\t\t\t}\n\t\t\tif !newIb.Enable {\n\t\t\t\treturn nil\n\t\t\t}\n\t\t\treturn l.AddInbound(ctx, newIb)\n\t\t}\n\t\t_ = l.DelInbound(ctx, oldIb)\n\t\treturn mcore.Reconcile()\n\t}\n\toldSingbox := model.IsSingboxProtocol(oldIb.Protocol)')
rep('internal/web/runtime/local.go',
    'func (l *Local) AddUser(_ context.Context, ib *model.Inbound, userMap map[string]any) error {\n\tif model.IsSingboxProtocol(ib.Protocol) {',
    'func (l *Local) AddUser(_ context.Context, ib *model.Inbound, userMap map[string]any) error {\n\tif model.IsMieruProtocol(ib.Protocol) {\n\t\treturn mcore.Reconcile()\n\t}\n\tif model.IsSingboxProtocol(ib.Protocol) {')
rep('internal/web/runtime/local.go',
    'func (l *Local) RemoveUser(_ context.Context, ib *model.Inbound, email string) error {\n\tif model.IsSingboxProtocol(ib.Protocol) {',
    'func (l *Local) RemoveUser(_ context.Context, ib *model.Inbound, email string) error {\n\tif model.IsMieruProtocol(ib.Protocol) {\n\t\treturn mcore.Reconcile()\n\t}\n\tif model.IsSingboxProtocol(ib.Protocol) {')

# Frontend enum/schema/defaults/native Inbound form.
rep('frontend/src/schemas/primitives/protocol.ts', "  'naive',\n]);", "  'naive',\n  'mieru',\n]);")
rep('frontend/src/schemas/primitives/protocol.ts', "  NAIVE: 'naive',\n});", "  NAIVE: 'naive',\n  MIERU: 'mieru',\n});")
rep('frontend/src/schemas/protocols/inbound/index.ts',
    "import { AnyTlsInboundSettingsSchema, NaiveInboundSettingsSchema, ShadowTlsInboundSettingsSchema, TuicInboundSettingsSchema } from './singbox';",
    "import { AnyTlsInboundSettingsSchema, MieruInboundSettingsSchema, NaiveInboundSettingsSchema, ShadowTlsInboundSettingsSchema, TuicInboundSettingsSchema } from './singbox';")
rep('frontend/src/schemas/protocols/inbound/index.ts',
    "  z.object({ protocol: z.literal('naive'), settings: NaiveInboundSettingsSchema }),\n]);",
    "  z.object({ protocol: z.literal('naive'), settings: NaiveInboundSettingsSchema }),\n  z.object({ protocol: z.literal('mieru'), settings: MieruInboundSettingsSchema }),\n]);")
rep('frontend/src/lib/xray/inbound-defaults.ts',
    "import type { AnyTlsInboundSettings, NaiveInboundSettings, ShadowTlsInboundSettings, TuicInboundSettings } from '@/schemas/protocols/inbound/singbox';",
    "import type { AnyTlsInboundSettings, MieruInboundSettings, NaiveInboundSettings, ShadowTlsInboundSettings, TuicInboundSettings } from '@/schemas/protocols/inbound/singbox';")
rep('frontend/src/lib/xray/inbound-defaults.ts',
    "export function createDefaultNaiveInboundSettings(): NaiveInboundSettings {\n  return { clients: [], network: '', quicCongestionControl: 'bbr' };\n}",
    "export function createDefaultNaiveInboundSettings(): NaiveInboundSettings {\n  return { clients: [], network: '', quicCongestionControl: 'bbr' };\n}\n\nexport function createDefaultMieruInboundSettings(): MieruInboundSettings {\n  return { clients: [], transport: 'TCP', portRangeEnd: 0, mtu: 1400, loggingLevel: 'INFO', allowPrivateIP: false, allowLoopbackIP: false, quotaDays: 0, quotaMegabytes: 0, metricsLoggingInterval: '', userHintIsMandatory: false, trafficPatternEnabled: false, trafficSeed: 0, trafficUnlockAll: false, tcpFragmentEnable: false, tcpFragmentMaxSleepMs: 0, nonceType: '', nonceApplyToAllUDP: false, nonceMinLen: 0, nonceMaxLen: 0, nonceCustomHexStrings: [], lowEntropyMode: 'LOW_ENTROPY_MODE_OFF', lowEntropyMaskRotation: 'LOW_ENTROPY_MASK_NO_ROTATION', clientMultiplexing: 'MULTIPLEXING_LOW', clientHandshakeMode: 'HANDSHAKE_STANDARD', clientTrafficPattern: '' };\n}")
rep('frontend/src/lib/xray/inbound-defaults.ts',
    '  | NaiveInboundSettings;',
    '  | NaiveInboundSettings\n  | MieruInboundSettings;')
rep('frontend/src/lib/xray/inbound-defaults.ts',
    "    case 'naive':\n      return createDefaultNaiveInboundSettings();\n    default:",
    "    case 'naive':\n      return createDefaultNaiveInboundSettings();\n    case 'mieru':\n      return createDefaultMieruInboundSettings();\n    default:")
rep('frontend/src/pages/inbounds/form/protocols/index.ts',
    "export { AnyTlsFields, NaiveFields, ShadowTlsFields, TuicFields } from './singbox';",
    "export { AnyTlsFields, MieruFields, NaiveFields, ShadowTlsFields, TuicFields } from './singbox';")
rep('frontend/src/lib/xray/protocol-capabilities.ts',
    "'mtproto', 'amneziawg', 'tuic', 'anytls', 'shadowtls', 'naive']",
    "'mtproto', 'amneziawg', 'tuic', 'anytls', 'shadowtls', 'naive', 'mieru']")
rep('frontend/src/pages/inbounds/form/InboundFormModal.tsx',
    '  NaiveFields,\n  VlessFields,',
    '  NaiveFields,\n  MieruFields,\n  VlessFields,')
rep('frontend/src/pages/inbounds/form/InboundFormModal.tsx',
    '} else if (next === Protocols.SHADOWTLS) {',
    '} else if (next === Protocols.SHADOWTLS || next === Protocols.MIERU) {')
rep('frontend/src/pages/inbounds/form/InboundFormModal.tsx',
    '      {protocol === Protocols.NAIVE && <NaiveFields />}\n',
    '      {protocol === Protocols.NAIVE && <NaiveFields />}\n      {protocol === Protocols.MIERU && <MieruFields />}\n')
rep('frontend/src/pages/inbounds/form/InboundFormModal.tsx',
    '                    Protocols.NAIVE,\n                  ] as string[]',
    '                    Protocols.NAIVE,\n                    Protocols.MIERU,\n                  ] as string[]')

# Native Client manager: Mieru is multi-user and uses email/password.
rep('frontend/src/pages/inbounds/list/helpers.ts',
    "    case 'naive':\n      return true;",
    "    case 'naive':\n    case 'mieru':\n      return true;")
for rel in ('frontend/src/pages/clients/ClientFormModal.tsx', 'frontend/src/pages/clients/ClientBulkAddModal.tsx'):
    rep(rel, "  'naive',\n]);", "  'naive',\n  'mieru',\n]);")

# Subscription: official mierus:// raw link + native Mihomo Mieru proxy.
rep('internal/sub/service.go',
    "'mtproto','tuic','anytls','shadowtls','naive')",
    "'mtproto','tuic','anytls','shadowtls','naive','mieru')")
rep('internal/sub/service.go',
    'case "tuic", "anytls", "shadowtls", "naive":\n\t\treturn s.genSingboxLink(inbound, email)\n\t}',
    'case "tuic", "anytls", "shadowtls", "naive":\n\t\treturn s.genSingboxLink(inbound, email)\n\tcase "mieru":\n\t\treturn s.genMieruLink(inbound, email)\n\t}')
rep('internal/sub/json_service.go', 'model.IsSingboxProtocol(inbound.Protocol)', 'model.IsSupplementalProtocol(inbound.Protocol)')
rep('internal/sub/clash_service.go',
    'if model.IsSingboxProtocol(inbound.Protocol) {\n\t\treturn s.buildSingboxProxy(subReq, inbound, client, stream, ep)\n\t}',
    'if model.IsMieruProtocol(inbound.Protocol) {\n\t\treturn s.buildMieruProxy(subReq, inbound, client, ep)\n\t}\n\tif model.IsSingboxProtocol(inbound.Protocol) {\n\t\treturn s.buildSingboxProxy(subReq, inbound, client, stream, ep)\n\t}')
