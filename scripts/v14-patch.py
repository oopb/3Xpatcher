#!/usr/bin/env python3
from pathlib import Path
import sys

if len(sys.argv) != 2:
    raise SystemExit("usage: v14-patch.py /path/to/3x-ui-source")

root = Path(sys.argv[1])


def load(rel: str) -> str:
    path = root / rel
    if not path.is_file():
        raise SystemExit(f"v14 missing target file: {rel}")
    return path.read_text(encoding="utf-8")


def save(rel: str, text: str) -> None:
    (root / rel).write_text(text, encoding="utf-8")


def replace_once(text: str, old: str, new: str, label: str) -> str:
    if text.count(old) != 1:
        raise SystemExit(f"v14 {label}: expected exactly one anchor, found {text.count(old)}")
    return text.replace(old, new, 1)


# ---------------------------------------------------------------------------
# Repair the v13 Snell renderer placement.
# ---------------------------------------------------------------------------
rel = "internal/singbox/config.go"
text = load(rel)
render_marker = "func renderInbound(r InboundRecord) ([]any, []string, error) {\n"
render_pos = text.find(render_marker)
if render_pos < 0:
    raise SystemExit("v14 could not locate renderInbound")

snell_block = '''\tcase ProtocolSnell:\n\t\tvar s SnellSettings\n\t\tif err := json.Unmarshal(r.Settings, &s); err != nil {\n\t\t\treturn nil, nil, err\n\t\t}\n\t\tversion := s.Version\n\t\tif version == 0 {\n\t\t\tversion = 5\n\t\t}\n\t\tif version != 5 {\n\t\t\treturn nil, nil, fmt.Errorf("3Xpatcher Snell compatibility mode requires version 5, got %d", version)\n\t\t}\n\t\tpsk := strings.TrimSpace(s.PSK)\n\t\tif psk == "" {\n\t\t\treturn nil, nil, errors.New("Snell PSK is required")\n\t\t}\n\t\tobfsMode := strings.TrimSpace(s.ObfsMode)\n\t\tif obfsMode == "" {\n\t\t\tobfsMode = "none"\n\t\t}\n\t\tif obfsMode != "none" && obfsMode != "http" {\n\t\t\treturn nil, nil, fmt.Errorf("invalid Snell obfs mode %q", obfsMode)\n\t\t}\n\t\tm := baseInbound("snell", r)\n\t\tapplyListen(m, s.ListenSettings)\n\t\tm["version"] = 5\n\t\tm["psk"] = psk\n\t\tif obfsMode == "http" {\n\t\t\tm["obfs_mode"] = "http"\n\t\t}\n\t\treturn []any{m}, nil, nil\n'''

prefix = text[:render_pos]
suffix = text[render_pos:]
wrong = snell_block + "\tcase ProtocolShadowTLS:\n"
if wrong not in prefix:
    raise SystemExit("v14 did not find the misplaced v13 Snell renderer before renderInbound")
prefix = prefix.replace(wrong, "\tcase ProtocolShadowTLS:\n", 1)
needle = "\tcase ProtocolShadowTLS:\n"
if needle not in suffix:
    raise SystemExit("v14 could not locate ProtocolShadowTLS inside renderInbound")
if "\tcase ProtocolSnell:\n" in suffix:
    raise SystemExit("v14 found an unexpected pre-existing Snell renderer inside renderInbound")
suffix = suffix.replace(needle, snell_block + needle, 1)
out = prefix + suffix
render_pos = out.find(render_marker)
if "\tcase ProtocolSnell:\n" in out[:render_pos]:
    raise SystemExit("v14 invariant failed: Snell renderer remains before renderInbound")
render_tail = out[render_pos:]
if render_tail.count("\tcase ProtocolSnell:\n") != 1:
    raise SystemExit("v14 invariant failed: renderInbound must contain exactly one Snell case")
save(rel, out)


# ---------------------------------------------------------------------------
# Shadowrocket raw-link compatibility.
#
# Snell: Shadowrocket expects the established snell:// form whose payload is
# RawStdBase64("chacha20-ietf-poly1305:PSK@host:port"). Keep the existing
# Surge descriptor for non-Shadowrocket raw consumers.
#
# Naive: Shadowrocket maps Naive TCP to HTTP2 and Naive UDP/QUIC to HTTP3.
# v13 still exported HTTP2 unconditionally, so a UDP-only inbound imported as
# HTTP2 and could not connect. Emit http3:// and h3 when network=udp.
# ---------------------------------------------------------------------------
rel = "internal/sub/singbox_links.go"
links = load(rel)
func_marker = "func (s *SubService) buildSingboxEndpointLink("
func_pos = links.find(func_marker)
if func_pos < 0:
    raise SystemExit("v14 could not locate buildSingboxEndpointLink")
snell_pos = links.find("\tcase model.Snell:\n", func_pos)
naive_pos = links.find("\tcase model.Naive:\n", snell_pos)
if snell_pos < 0 or naive_pos < 0:
    raise SystemExit("v14 could not locate Snell/Naive subscription cases")

snell_case = '''\tcase model.Snell:\n\t\tif strings.TrimSpace(client.Password) == "" {\n\t\t\treturn ""\n\t\t}\n\t\tif isShadowrocketUserAgent(s.clientUserAgent) {\n\t\t\treturn buildShadowrocketSnellLink(settings, client.Password, host, port, remark)\n\t\t}\n\t\tname := strings.ReplaceAll(remark, "=", "-")\n\t\tif strings.TrimSpace(name) == "" {\n\t\t\tname = "Snell"\n\t\t}\n\t\tparts := []string{\n\t\t\tfmt.Sprintf("%s = snell", name),\n\t\t\tstrings.Trim(host, "[]"),\n\t\t\tfmt.Sprintf("%d", port),\n\t\t\t"psk=" + client.Password,\n\t\t\t"version=5",\n\t\t\t"reuse=false",\n\t\t}\n\t\tif mode, _ := settings["obfsMode"].(string); mode == "http" {\n\t\t\tparts = append(parts, "obfs=http")\n\t\t\tobfsHost, _ := settings["obfsHost"].(string)\n\t\t\tif strings.TrimSpace(obfsHost) == "" {\n\t\t\t\tobfsHost = "bing.com"\n\t\t\t}\n\t\t\tparts = append(parts, "obfs-host="+strings.TrimSpace(obfsHost))\n\t\t}\n\t\treturn strings.Join(parts, ", ")\n'''
links = links[:snell_pos] + snell_case + links[naive_pos:]
links = replace_once(
    links,
    "\t\t\treturn buildShadowrocketNaiveHTTP2Link(client.Email, client.Password, host, port, params, remark)\n",
    "\t\t\treturn buildShadowrocketNaiveLink(settings, client.Email, client.Password, host, port, params, remark)\n",
    "Shadowrocket Naive call",
)

helper_start = links.find("func buildShadowrocketNaiveHTTP2Link(")
helper_end = links.find("func buildSuiNaiveNativeLinks(", helper_start)
if helper_start < 0 or helper_end < 0:
    raise SystemExit("v14 could not locate Shadowrocket Naive helper section")
helpers = '''func buildShadowrocketSnellLink(settings map[string]any, psk, host string, port int, remark string) string {\n\tif strings.TrimSpace(psk) == "" || strings.TrimSpace(host) == "" || port < 1 || port > 65535 {\n\t\treturn ""\n\t}\n\tpayload := fmt.Sprintf("chacha20-ietf-poly1305:%s@%s", psk, joinHostPort(host, port))\n\tencoded := base64.RawStdEncoding.EncodeToString([]byte(payload))\n\tparams := map[string]string{"version": "5", "tfo": "0"}\n\tif tfo, _ := settings["tcpFastOpen"].(bool); tfo {\n\t\tparams["tfo"] = "1"\n\t}\n\tif mode, _ := settings["obfsMode"].(string); mode == "http" {\n\t\tparams["obfs"] = "http"\n\t\tobfsHost, _ := settings["obfsHost"].(string)\n\t\tif strings.TrimSpace(obfsHost) == "" {\n\t\t\tobfsHost = "bing.com"\n\t\t}\n\t\tparams["obfs-host"] = strings.TrimSpace(obfsHost)\n\t}\n\treturn buildLinkWithParams("snell://"+encoded, params, remark)\n}\n\nfunc buildShadowrocketNaiveLink(settings map[string]any, username, password, host string, port int, params map[string]string, remark string) string {\n\tif username == "" || password == "" || host == "" || port < 1 || port > 65535 {\n\t\treturn ""\n\t}\n\tscheme := "http2"\n\tif network, _ := settings["network"].(string); network == "udp" {\n\t\tscheme = "http3"\n\t\tparams["alpn"] = "h3"\n\t}\n\tauthority := fmt.Sprintf("%s:%s@%s", username, password, joinHostPort(host, port))\n\tencoded := base64.StdEncoding.EncodeToString([]byte(authority))\n\treturn buildLinkWithParams(scheme+"://"+encoded, params, remark)\n}\n\n'''
links = links[:helper_start] + helpers + links[helper_end:]
save(rel, links)


# Keep the panel QR/copy surface consistent with the subscription backend.
rel = "frontend/src/lib/xray/supplemental-links.ts"
front = load(rel)
naive_start = front.find("function buildNaiveLinks(")
naive_end = front.find("function buildMieruLink(", naive_start)
if naive_start < 0 or naive_end < 0:
    raise SystemExit("v14 could not locate frontend Naive link builder")
new_naive = '''function buildNaiveLinks(input: GenSupplementalLinksInput): SupplementalLinkVariant[] {\n  const settings = asRecord(input.inbound.settings);\n  const username = (input.client.email || '').trim();\n  const password = input.client.password || '';\n  const host = formatUrlHost(input.address);\n  if (!username || !password || !host) return [];\n\n  const params = new URLSearchParams();\n  applyTlsParams(input.inbound, input.externalProxy, params);\n  applySuiNaiveParams(settings, params);\n\n  const network = asString(settings.network);\n  const shadowrocketScheme = network === 'udp' ? 'http3' : 'http2';\n  if (network === 'udp') params.set('alpn', 'h3');\n  const rawAuthority = `${username}:${password}@${host}:${input.port}`;\n  const variants: SupplementalLinkVariant[] = [\n    {\n      link: buildLink(`${shadowrocketScheme}://${Base64.encode(rawAuthority)}`, params, input.remark),\n      label: network === 'udp' ? 'Naive / Shadowrocket HTTP3' : 'Naive / Shadowrocket HTTP2',\n    },\n  ];\n\n  const nativeSchemes = network === 'tcp'\n    ? ['naive+https']\n    : network === 'udp'\n      ? ['naive+quic']\n      : ['naive+https', 'naive+quic'];\n  const encodedAuthority = `${encodeUserinfo(username)}:${encodeUserinfo(password)}@${host}:${input.port}`;\n  for (const scheme of nativeSchemes) {\n    variants.push({\n      link: buildLink(`${scheme}://${encodedAuthority}`, params, input.remark),\n      label: scheme === 'naive+quic' ? 'Naive / QUIC' : 'Naive / HTTPS',\n    });\n  }\n  return variants;\n}\n\n'''
front = front[:naive_start] + new_naive + front[naive_end:]

gen_pos = front.find("export function genSupplementalLinks(")
if gen_pos < 0:
    raise SystemExit("v14 could not locate frontend genSupplementalLinks")
front_snell = front.find("    case 'snell': {\n", gen_pos)
front_mieru = front.find("    case 'mieru':\n", front_snell)
if front_snell < 0 or front_mieru < 0:
    raise SystemExit("v14 could not locate frontend Snell link case")
new_front_snell = '''    case 'snell': {\n      if (!client.password) return [];\n      const payload = `chacha20-ietf-poly1305:${client.password}@${host}:${port}`;\n      const encoded = Base64.encode(payload).replace(/=+$/g, '');\n      const params = new URLSearchParams();\n      params.set('version', '5');\n      params.set('tfo', asBoolean(settings.tcpFastOpen) ? '1' : '0');\n      if (asString(settings.obfsMode) === 'http') {\n        params.set('obfs', 'http');\n        const obfsHost = asString(settings.obfsHost).trim() || 'bing.com';\n        params.set('obfs-host', obfsHost);\n      }\n      return [{\n        link: buildLink(`snell://${encoded}`, params, remark),\n        label: 'Snell v5 / Shadowrocket',\n      }];\n    }\n'''
front = front[:front_snell] + new_front_snell + front[front_mieru:]
save(rel, front)

print("v14 Snell renderer + Shadowrocket Snell/Naive compatibility fixes applied")
