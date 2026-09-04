#!/usr/bin/env python3
from pathlib import Path
import re
import sys

if len(sys.argv) != 2:
    raise SystemExit("usage: v13-patch.py /path/to/3x-ui-source")

root = Path(sys.argv[1])


def load(rel: str) -> str:
    path = root / rel
    if not path.is_file():
        raise SystemExit(f"v13 missing target file: {rel}")
    return path.read_text(encoding="utf-8")


def save(rel: str, text: str) -> None:
    (root / rel).write_text(text, encoding="utf-8")


def rep(rel: str, old: str, new: str, count: int = 1) -> None:
    text = load(rel)
    hits = text.count(old)
    if hits < count:
        raise SystemExit(f"v13 anchor missing in {rel}: wanted {count}, found {hits}: {old[:140]!r}")
    save(rel, text.replace(old, new, count))


def sub(rel: str, pattern: str, repl: str, count: int = 1, flags: int = 0) -> None:
    text = load(rel)
    out, n = re.subn(pattern, repl, text, count=count, flags=flags)
    if n != count:
        raise SystemExit(f"v13 regex anchor mismatch in {rel}: wanted {count}, got {n}: {pattern[:140]!r}")
    save(rel, out)


# ---------------------------------------------------------------------------
# Snell v5 policy
# ---------------------------------------------------------------------------
# sing-box 1.14 supports a non-standard multi-user extension via userkey, but
# current Mihomo/Clash Verge and Surge-compatible clients expose only the
# ordinary Snell PSK. To keep exported nodes usable, 3Xpatcher intentionally
# runs Snell v5 in one-client-per-inbound compatibility mode. The canonical
# 3x-ui client password is the Snell PSK, so normal client expiry/quota/enable
# state remains authoritative and no extra secret is stored in settings.

# Backend protocol model / validation.
rep(
    "internal/database/model/singbox_protocols.go",
    '\tNaive     Protocol = "naive"\n\tMieru     Protocol = "mieru"',
    '\tNaive     Protocol = "naive"\n\tSnell     Protocol = "snell"\n\tMieru     Protocol = "mieru"',
)
rep(
    "internal/database/model/singbox_protocols.go",
    "\tcase TUIC, AnyTLS, ShadowTLS, Naive:",
    "\tcase TUIC, AnyTLS, ShadowTLS, Naive, Snell:",
)
rep(
    "internal/database/model/model.go",
    'oneof=vmess vless trojan shadowsocks wireguard hysteria http mixed tunnel tun mtproto amneziawg tuic anytls shadowtls naive mieru"',
    'oneof=vmess vless trojan shadowsocks wireguard hysteria http mixed tunnel tun mtproto amneziawg tuic anytls shadowtls naive snell mieru"',
)
rep(
    "internal/web/service/inbound.go",
    "\t\tmodel.Mieru:       true,",
    "\t\tmodel.Snell:       true,\n\t\tmodel.Mieru:       true,",
)
rep(
    "internal/web/service/inbound.go",
    'case "anytls", "shadowtls", "naive", "mieru":',
    'case "anytls", "shadowtls", "naive", "snell", "mieru":',
)
rep(
    "internal/web/service/client_crud.go",
    "case model.AnyTLS, model.ShadowTLS, model.Naive, model.Mieru:",
    "case model.AnyTLS, model.ShadowTLS, model.Naive, model.Snell, model.Mieru:",
)
rep(
    "internal/web/service/client_inbound_apply.go",
    'case "anytls", "shadowtls", "naive", "mieru":',
    'case "anytls", "shadowtls", "naive", "snell", "mieru":',
    count=2,
)
rep(
    "internal/web/service/inbound_clients.go",
    "case model.AnyTLS, model.ShadowTLS, model.Naive, model.Mieru:",
    "case model.AnyTLS, model.ShadowTLS, model.Naive, model.Snell, model.Mieru:",
)

# Core protocol renderer.
rep(
    "internal/singbox/config.go",
    '\tProtocolNaive     Protocol = "naive"\n)',
    '\tProtocolNaive     Protocol = "naive"\n\tProtocolSnell     Protocol = "snell"\n)',
)
rep(
    "internal/singbox/config.go",
    "\tProtocolTUIC: {}, ProtocolAnyTLS: {}, ProtocolShadowTLS: {}, ProtocolNaive: {},",
    "\tProtocolTUIC: {}, ProtocolAnyTLS: {}, ProtocolShadowTLS: {}, ProtocolNaive: {}, ProtocolSnell: {},",
)
rep(
    "internal/singbox/config.go",
    "\treturn []Protocol{ProtocolTUIC, ProtocolAnyTLS, ProtocolShadowTLS, ProtocolNaive}",
    "\treturn []Protocol{ProtocolTUIC, ProtocolAnyTLS, ProtocolShadowTLS, ProtocolNaive, ProtocolSnell}",
)
rep(
    "internal/singbox/config.go",
    '''type NaiveSettings struct {\n\tListenSettings\n\tNetwork               string      `json:"network,omitempty"`\n\tUsers                 []NaiveUser `json:"users"`\n\tQUICCongestionControl string      `json:"quicCongestionControl,omitempty"`\n\tTLS                   TLSSettings `json:"tls"`\n}\n''',
    '''type NaiveSettings struct {\n\tListenSettings\n\tNetwork               string      `json:"network,omitempty"`\n\tUsers                 []NaiveUser `json:"users"`\n\tQUICCongestionControl string      `json:"quicCongestionControl,omitempty"`\n\tTLS                   TLSSettings `json:"tls"`\n}\n\ntype SnellSettings struct {\n\tListenSettings\n\tVersion  int    `json:"version,omitempty"`\n\tPSK      string `json:"psk"`\n\tObfsMode string `json:"obfsMode,omitempty"`\n\tObfsHost string `json:"obfsHost,omitempty"`\n}\n''',
)
rep(
    "internal/singbox/config.go",
    "\tcase ProtocolShadowTLS:\n",
    '''\tcase ProtocolSnell:\n\t\tvar s SnellSettings\n\t\tif err := json.Unmarshal(r.Settings, &s); err != nil {\n\t\t\treturn nil, nil, err\n\t\t}\n\t\tversion := s.Version\n\t\tif version == 0 {\n\t\t\tversion = 5\n\t\t}\n\t\tif version != 5 {\n\t\t\treturn nil, nil, fmt.Errorf("3Xpatcher Snell compatibility mode requires version 5, got %d", version)\n\t\t}\n\t\tpsk := strings.TrimSpace(s.PSK)\n\t\tif psk == "" {\n\t\t\treturn nil, nil, errors.New("Snell PSK is required")\n\t\t}\n\t\tobfsMode := strings.TrimSpace(s.ObfsMode)\n\t\tif obfsMode == "" {\n\t\t\tobfsMode = "none"\n\t\t}\n\t\tif obfsMode != "none" && obfsMode != "http" {\n\t\t\treturn nil, nil, fmt.Errorf("invalid Snell obfs mode %q", obfsMode)\n\t\t}\n\t\tm := baseInbound("snell", r)\n\t\tapplyListen(m, s.ListenSettings)\n\t\tm["version"] = 5\n\t\tm["psk"] = psk\n\t\tif obfsMode == "http" {\n\t\t\tm["obfs_mode"] = "http"\n\t\t}\n\t\treturn []any{m}, nil, nil\n\tcase ProtocolShadowTLS:\n''',
)

# Canonical 3x-ui Client -> Snell PSK. Enforce one active client because the
# public Mihomo/Surge client formats cannot carry sing-box's userkey extension.
rep(
    "internal/singbox/integrated.go",
    "\tprotocols := []model.Protocol{model.TUIC, model.AnyTLS, model.ShadowTLS, model.Naive}",
    "\tprotocols := []model.Protocol{model.TUIC, model.AnyTLS, model.ShadowTLS, model.Naive, model.Snell}",
)
rep(
    "internal/singbox/integrated.go",
    "\tcase model.Naive:\n",
    '''\tcase model.Snell:\n\t\tif len(clients) != 1 {\n\t\t\treturn InboundRecord{}, false, fmt.Errorf("Snell v5 compatibility mode requires exactly one active client; found %d", len(clients))\n\t\t}\n\t\tpsk := strings.TrimSpace(clients[0].Password)\n\t\tif psk == "" {\n\t\t\treturn InboundRecord{}, false, fmt.Errorf("client %q has no Snell PSK/password", clients[0].Email)\n\t\t}\n\t\tsettings["version"] = 5\n\t\tsettings["psk"] = psk\n\t\tdelete(settings, "users")\n\tcase model.Naive:\n''',
)

# Subscription route + Clash/Mihomo export.
rep(
    "internal/sub/service.go",
    "'mtproto','tuic','anytls','shadowtls','naive','mieru')",
    "'mtproto','tuic','anytls','shadowtls','naive','snell','mieru')",
)
rep(
    "internal/sub/service.go",
    'case "tuic", "anytls", "shadowtls", "naive":',
    'case "tuic", "anytls", "shadowtls", "naive", "snell":',
)
rep(
    "internal/sub/singbox_clash.go",
    "\tcase model.Naive:\n\t\treturn nil",
    '''\tcase model.Snell:\n\t\tif strings.TrimSpace(client.Password) == "" {\n\t\t\treturn nil\n\t\t}\n\t\tproxy["type"] = "snell"\n\t\tproxy["psk"] = client.Password\n\t\tproxy["version"] = 5\n\t\tproxy["udp"] = true\n\t\tif mode, _ := settings["obfsMode"].(string); mode == "http" {\n\t\t\thost, _ := settings["obfsHost"].(string)\n\t\t\tif strings.TrimSpace(host) == "" {\n\t\t\t\thost = "bing.com"\n\t\t\t}\n\t\t\tproxy["obfs-opts"] = map[string]any{"mode": "http", "host": strings.TrimSpace(host)}\n\t\t}\n\t\treturn proxy\n\tcase model.Naive:\n\t\treturn nil''',
)

# Raw/browser share representation. Snell has no universal URI scheme, so use
# the official Surge-style descriptor rather than inventing a snell:// URL.
rep(
    "internal/sub/singbox_links.go",
    "\tcase model.Naive:\n",
    '''\tcase model.Snell:\n\t\tif strings.TrimSpace(client.Password) == "" {\n\t\t\treturn ""\n\t\t}\n\t\tname := strings.ReplaceAll(remark, "=", "-")\n\t\tif strings.TrimSpace(name) == "" {\n\t\t\tname = "Snell"\n\t\t}\n\t\tparts := []string{\n\t\t\tfmt.Sprintf("%s = snell", name),\n\t\t\tstrings.Trim(host, "[]"),\n\t\t\tfmt.Sprintf("%d", port),\n\t\t\t"psk=" + client.Password,\n\t\t\t"version=5",\n\t\t\t"reuse=false",\n\t\t}\n\t\tif mode, _ := settings["obfsMode"].(string); mode == "http" {\n\t\t\tparts = append(parts, "obfs=http")\n\t\t\tif obfsHost, _ := settings["obfsHost"].(string); strings.TrimSpace(obfsHost) != "" {\n\t\t\t\tparts = append(parts, "obfs-host="+strings.TrimSpace(obfsHost))\n\t\t\t}\n\t\t}\n\t\treturn strings.Join(parts, ", ")\n\tcase model.Naive:\n''',
)

# Attribute Snell's inbound-only stats to its sole canonical client. Without
# sing-box `users`, V2Ray API correctly exposes the inbound counter but cannot
# name a user; one-client mode lets us map it without ambiguity.
rep(
    "internal/web/job/supplemental_traffic_job.go",
    '"github.com/mhsanaei/3x-ui/v3/internal/logger"\n',
    '"github.com/mhsanaei/3x-ui/v3/internal/logger"\n\t"github.com/mhsanaei/3x-ui/v3/internal/database/model"\n',
)
rep(
    "internal/web/job/supplemental_traffic_job.go",
    '''\tif ts, cs, emails, tags, err := sbox.CollectTraffic(); err != nil {\n\t\tlogger.Debug("supplemental sing-box stats unavailable:", err)\n\t} else {\n\t\tmerge(ts, cs, emails, tags)\n\t}\n''',
    '''\tif ts, cs, emails, tags, err := sbox.CollectTraffic(); err != nil {\n\t\tlogger.Debug("supplemental sing-box stats unavailable:", err)\n\t} else {\n\t\tsnellClients, snellEmails := snellSingleClientTraffic(database.GetDB(), ts, tags)\n\t\tcs = append(cs, snellClients...)\n\t\temails = append(emails, snellEmails...)\n\t\tmerge(ts, cs, emails, tags)\n\t}\n''',
)
rep(
    "internal/web/job/supplemental_traffic_job.go",
    "type SupplementalTrafficJob struct {",
    '''func snellSingleClientTraffic(db interface {\n\tTable(string, ...any) *gorm.DB\n}, traffics []*xray.Traffic, activeTags []string) ([]*xray.ClientTraffic, []string) {\n\t_ = db\n\treturn nil, nil\n}\n\ntype SupplementalTrafficJob struct {''',
)
# Replace the small compile-time placeholder above with an implementation that
# uses *gorm.DB. Keeping it as a second anchored rewrite makes failures obvious.
rep(
    "internal/web/job/supplemental_traffic_job.go",
    '''func snellSingleClientTraffic(db interface {\n\tTable(string, ...any) *gorm.DB\n}, traffics []*xray.Traffic, activeTags []string) ([]*xray.ClientTraffic, []string) {\n\t_ = db\n\treturn nil, nil\n}\n''',
    '''func snellSingleClientTraffic(db *gorm.DB, traffics []*xray.Traffic, activeTags []string) ([]*xray.ClientTraffic, []string) {\n\tif db == nil {\n\t\treturn nil, nil\n\t}\n\ttagSet := make(map[string]struct{}, len(activeTags)+len(traffics))\n\tfor _, tag := range activeTags {\n\t\tif tag != "" {\n\t\t\ttagSet[tag] = struct{}{}\n\t\t}\n\t}\n\tfor _, traffic := range traffics {\n\t\tif traffic != nil && traffic.Tag != "" {\n\t\t\ttagSet[traffic.Tag] = struct{}{}\n\t\t}\n\t}\n\tif len(tagSet) == 0 {\n\t\treturn nil, nil\n\t}\n\ttags := make([]string, 0, len(tagSet))\n\tfor tag := range tagSet {\n\t\ttags = append(tags, tag)\n\t}\n\tvar inbounds []model.Inbound\n\tif err := db.Where("protocol = ? AND tag IN ?", model.Snell, tags).Find(&inbounds).Error; err != nil {\n\t\tlogger.Debug("snell stats inbound lookup failed:", err)\n\t\treturn nil, nil\n\t}\n\tbyTag := make(map[string]string, len(inbounds))\n\tfor _, inbound := range inbounds {\n\t\tvar clients []model.ClientRecord\n\t\tif err := db.Table("clients AS c").\n\t\t\tSelect("c.*").\n\t\t\tJoins("JOIN client_inbounds AS ci ON ci.client_id = c.id").\n\t\t\tWhere("ci.inbound_id = ? AND c.enable = ?", inbound.Id, true).\n\t\t\tLimit(2).Scan(&clients).Error; err != nil || len(clients) != 1 {\n\t\t\tcontinue\n\t\t}\n\t\tbyTag[inbound.Tag] = clients[0].Email\n\t}\n\tclientDelta := make(map[string]*xray.ClientTraffic)\n\tactiveEmailSet := make(map[string]struct{})\n\tfor _, tag := range activeTags {\n\t\tif email := byTag[tag]; email != "" {\n\t\t\tactiveEmailSet[email] = struct{}{}\n\t\t}\n\t}\n\tfor _, traffic := range traffics {\n\t\tif traffic == nil {\n\t\t\tcontinue\n\t\t}\n\t\temail := byTag[traffic.Tag]\n\t\tif email == "" {\n\t\t\tcontinue\n\t\t}\n\t\tdelta := clientDelta[email]\n\t\tif delta == nil {\n\t\t\tdelta = &xray.ClientTraffic{Email: email}\n\t\t\tclientDelta[email] = delta\n\t\t}\n\t\tdelta.Up += traffic.Up\n\t\tdelta.Down += traffic.Down\n\t}\n\tclients := make([]*xray.ClientTraffic, 0, len(clientDelta))\n\tfor _, client := range clientDelta {\n\t\tclients = append(clients, client)\n\t}\n\temails := make([]string, 0, len(activeEmailSet))\n\tfor email := range activeEmailSet {\n\t\temails = append(emails, email)\n\t}\n\treturn clients, emails\n}\n''',
)
rep(
    "internal/web/job/supplemental_traffic_job.go",
    '"github.com/mhsanaei/3x-ui/v3/internal/xray"\n',
    '"github.com/mhsanaei/3x-ui/v3/internal/xray"\n\t"gorm.io/gorm"\n',
)

# Frontend protocol schema / enum / defaults.
rep(
    "frontend/src/schemas/primitives/protocol.ts",
    "  'mieru',\n]);",
    "  'snell',\n  'mieru',\n]);",
)
rep(
    "frontend/src/schemas/primitives/protocol.ts",
    "  MIERU: 'mieru',\n});",
    "  SNELL: 'snell',\n  MIERU: 'mieru',\n});",
)
rep(
    "frontend/src/schemas/protocols/inbound/singbox.ts",
    "const MieruPortBindingSchema = z.object({",
    '''export const SnellInboundSettingsSchema = z\n  .object({\n    clients: z\n      .array(IntegratedClientSchema)\n      .max(1, 'Snell v5 Clash/Surge compatibility mode supports one client per inbound')\n      .default([]),\n    version: z.literal(5).default(5),\n    obfsMode: z.enum(['none', 'http']).default('none'),\n    obfsHost: z.string().default('bing.com'),\n    ...listenTuning,\n  })\n  .loose();\n\nconst MieruPortBindingSchema = z.object({''',
)
rep(
    "frontend/src/schemas/protocols/inbound/singbox.ts",
    "export type MieruInboundSettings = z.infer<typeof MieruInboundSettingsSchema>;",
    "export type SnellInboundSettings = z.infer<typeof SnellInboundSettingsSchema>;\nexport type MieruInboundSettings = z.infer<typeof MieruInboundSettingsSchema>;",
)
sub(
    "frontend/src/schemas/protocols/inbound/index.ts",
    r"import \{ ([^\n]*MieruInboundSettingsSchema[^\n]*) \} from './singbox';",
    lambda m: "import { " + (m.group(1) + ", SnellInboundSettingsSchema" if "SnellInboundSettingsSchema" not in m.group(1) else m.group(1)) + " } from './singbox';",
)
rep(
    "frontend/src/schemas/protocols/inbound/index.ts",
    "  z.object({ protocol: z.literal('mieru'), settings: MieruInboundSettingsSchema }),",
    "  z.object({ protocol: z.literal('snell'), settings: SnellInboundSettingsSchema }),\n  z.object({ protocol: z.literal('mieru'), settings: MieruInboundSettingsSchema }),",
)
sub(
    "frontend/src/lib/xray/inbound-defaults.ts",
    r"import type \{ ([^\n]*MieruInboundSettings[^\n]*) \} from '@/schemas/protocols/inbound/singbox';",
    lambda m: "import type { " + (m.group(1) + ", SnellInboundSettings" if "SnellInboundSettings" not in m.group(1) else m.group(1)) + " } from '@/schemas/protocols/inbound/singbox';",
)
rep(
    "frontend/src/lib/xray/inbound-defaults.ts",
    "export function createDefaultMieruInboundSettings(): MieruInboundSettings {",
    '''export function createDefaultSnellInboundSettings(): SnellInboundSettings {\n  return {\n    clients: [],\n    version: 5,\n    obfsMode: 'none',\n    obfsHost: 'bing.com',\n    bindInterface: '',\n    routingMark: 0,\n    reuseAddr: false,\n    netns: '',\n    tcpFastOpen: false,\n    tcpMultiPath: false,\n    disableTCPKeepAlive: false,\n    tcpKeepAlive: '',\n    tcpKeepAliveInterval: '',\n    udpTimeout: '',\n  };\n}\n\nexport function createDefaultMieruInboundSettings(): MieruInboundSettings {''',
)
rep(
    "frontend/src/lib/xray/inbound-defaults.ts",
    "  | NaiveInboundSettings\n  | MieruInboundSettings;",
    "  | NaiveInboundSettings\n  | SnellInboundSettings\n  | MieruInboundSettings;",
)
rep(
    "frontend/src/lib/xray/inbound-defaults.ts",
    "    case 'mieru':\n      return createDefaultMieruInboundSettings();",
    "    case 'snell':\n      return createDefaultSnellInboundSettings();\n    case 'mieru':\n      return createDefaultMieruInboundSettings();",
)

# Snell form lives beside the other supplemental forms.
rep(
    "frontend/src/pages/inbounds/form/protocols/singbox.tsx",
    "export function NaiveFields() {",
    '''export function SnellFields() {\n  const { control } = useFormContext();\n  const obfsMode = useWatch({ control, name: 'settings.obfsMode' }) as string | undefined;\n  return (\n    <>\n      <Alert\n        type="info"\n        showIcon\n        title="Snell v5 compatibility mode"\n        description="One active 3x-ui client is allowed per Snell inbound. That client's generated password is used directly as the Snell PSK so Clash Verge/Mihomo and Surge-compatible clients can connect without sing-box-only userkey support."\n        style={{ marginBottom: 12 }}\n      />\n      <FormField label="Version" name={['settings', 'version']}><InputNumber value={5} disabled style={{ width: '100%' }} /></FormField>\n      <FormField label="HTTP Obfuscation" name={['settings', 'obfsMode']}>\n        <Select options={[{ value: 'none', label: 'None' }, { value: 'http', label: 'HTTP' }]} />\n      </FormField>\n      {obfsMode === 'http' && (\n        <FormField label="Obfs Host" name={['settings', 'obfsHost']}><Input placeholder="bing.com" /></FormField>\n      )}\n      <ListenTuningFields />\n    </>\n  );\n}\n\nexport function NaiveFields() {''',
)
rep(
    "frontend/src/pages/inbounds/form/protocols/index.ts",
    "export { AnyTlsFields, MieruFields, NaiveFields, ShadowTlsFields, TuicFields } from './singbox';",
    "export { AnyTlsFields, MieruFields, NaiveFields, ShadowTlsFields, SnellFields, TuicFields } from './singbox';",
)
rep(
    "frontend/src/pages/inbounds/form/InboundFormModal.tsx",
    "  MieruFields,\n  VlessFields,",
    "  MieruFields,\n  SnellFields,\n  VlessFields,",
)
rep(
    "frontend/src/pages/inbounds/form/InboundFormModal.tsx",
    "} else if (next === Protocols.SHADOWTLS || next === Protocols.MIERU) {",
    "} else if (next === Protocols.SHADOWTLS || next === Protocols.SNELL || next === Protocols.MIERU) {",
)
rep(
    "frontend/src/pages/inbounds/form/InboundFormModal.tsx",
    "      {protocol === Protocols.MIERU && <MieruFields />}\n",
    "      {protocol === Protocols.SNELL && <SnellFields />}\n      {protocol === Protocols.MIERU && <MieruFields />}\n",
)
rep(
    "frontend/src/pages/inbounds/form/InboundFormModal.tsx",
    "                    Protocols.MIERU,\n",
    "                    Protocols.SNELL,\n                    Protocols.MIERU,\n",
)
rep(
    "frontend/src/pages/inbounds/useInbounds.ts",
    "  Protocols.MIERU,\n];",
    "  Protocols.SNELL,\n  Protocols.MIERU,\n];",
)
rep(
    "frontend/src/pages/inbounds/info/helpers.ts",
    "  Protocols.MIERU,\n]);",
    "  Protocols.SNELL,\n  Protocols.MIERU,\n]);",
)
rep(
    "frontend/src/pages/inbounds/list/helpers.ts",
    "    case 'mieru':\n      return true;",
    "    case 'snell':\n    case 'mieru':\n      return true;",
)
for rel in ("frontend/src/pages/clients/ClientFormModal.tsx", "frontend/src/pages/clients/ClientBulkAddModal.tsx"):
    rep(rel, "  'mieru',\n]);", "  'snell',\n  'mieru',\n]);")

# Treat Snell as a supplemental/non-Xray capability wherever the final protocol
# capability list currently ends at Mieru.
text = load("frontend/src/lib/xray/protocol-capabilities.ts")
if "'snell'" not in text:
    if "'naive', 'mieru'" in text:
        text = text.replace("'naive', 'mieru'", "'naive', 'snell', 'mieru'")
    elif "'naive',\n  'mieru'" in text:
        text = text.replace("'naive',\n  'mieru'", "'naive',\n  'snell',\n  'mieru'")
    else:
        raise SystemExit("v13 could not locate protocol-capabilities supplemental list")
    save("frontend/src/lib/xray/protocol-capabilities.ts", text)

# Browser link surface: expose a Surge-style descriptor rather than a fake URL.
rep(
    "frontend/src/lib/xray/supplemental-links.ts",
    "    case 'mieru': {",
    "    case 'snell':\n    case 'mieru': {",
)
rep(
    "frontend/src/lib/xray/supplemental-links.ts",
    "    case 'mieru':\n      return buildMieruLink(input);",
    '''    case 'snell': {\n      if (!client.password) return [];\n      const name = (remark || 'Snell').replace(/=/g, '-');\n      const parts = [\n        `${name} = snell`,\n        address.replace(/^\\[|\\]$/g, ''),\n        String(port),\n        `psk=${client.password}`,\n        'version=5',\n        'reuse=false',\n      ];\n      if (asString(settings.obfsMode) === 'http') {\n        parts.push('obfs=http');\n        const obfsHost = asString(settings.obfsHost).trim();\n        if (obfsHost) parts.push(`obfs-host=${obfsHost}`);\n      }\n      return [{ link: parts.join(', '), label: 'Snell v5 / Surge descriptor' }];\n    }\n    case 'mieru':\n      return buildMieruLink(input);''',
)

print("v13 Snell v5 compatibility patch applied")
