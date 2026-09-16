#!/usr/bin/env python3
from pathlib import Path
import sys

if len(sys.argv) != 2:
    raise SystemExit(f"Usage: {sys.argv[0]} /path/to/3x-ui-source")

root = Path(sys.argv[1]).resolve()

# 3x-ui v3.8.5 performs a backend TLS certificate validation after
# normalizeStreamSettings() and before the inbound reaches the supplemental
# sing-box renderer. Materialize self-signed-SNI certificates into the native
# certificateFile/keyFile representation before both AddInbound and
# UpdateInbound validations.
path = root / "internal/web/service/inbound.go"
text = path.read_text(encoding="utf-8")
old = "\ts.normalizeStreamSettings(inbound)\n\tif !s.FromNodeSync {\n\t\tif err := validateInboundTLSCertificates(inbound.StreamSettings); err != nil {"
new = "\ts.normalizeStreamSettings(inbound)\n\tif err := materializeSupplementalSelfSignedTLS(inbound); err != nil {\n\t\treturn inbound, false, err\n\t}\n\tif !s.FromNodeSync {\n\t\tif err := validateInboundTLSCertificates(inbound.StreamSettings); err != nil {"
count = text.count(old)
if count != 1:
    raise SystemExit(f"v18 AddInbound TLS materialization: expected one anchor, found {count}")
text = text.replace(old, new, 1)

# UpdateInbound validates later than its normalization and has the grandfather
# compatibility block. Generate the file-backed certificate before that block.
old = "\t// Grandfather a row that was already stored incomplete so it stays editable;\n\t// only a save that breaks a previously valid TLS block is refused.\n\tif !s.FromNodeSync {\n\t\tif err := validateInboundTLSCertificates(inbound.StreamSettings); err != nil {"
new = "\tif err := materializeSupplementalSelfSignedTLS(inbound); err != nil {\n\t\treturn inbound, false, err\n\t}\n\n\t// Grandfather a row that was already stored incomplete so it stays editable;\n\t// only a save that breaks a previously valid TLS block is refused.\n\tif !s.FromNodeSync {\n\t\tif err := validateInboundTLSCertificates(inbound.StreamSettings); err != nil {"
count = text.count(old)
if count != 1:
    raise SystemExit(f"v18 UpdateInbound TLS materialization: expected one anchor, found {count}")
text = text.replace(old, new, 1)
path.write_text(text, encoding="utf-8")

# Mieru's official server guide recommends PREFER_IPv4, especially on hosts
# without reliable IPv6. Use it as the runtime default while still honoring an
# explicitly selected dual-stack policy.
path = root / "internal/mieru/config.go"
text = path.read_text(encoding="utf-8")
old = "func buildDNS(s Settings) (map[string]any, error) {\n\tdualStack := strings.TrimSpace(s.DNSDualStack)\n\tif dualStack != \"\" {"
new = "func buildDNS(s Settings) (map[string]any, error) {\n\tdualStack := strings.TrimSpace(s.DNSDualStack)\n\tif dualStack == \"\" {\n\t\tdualStack = \"PREFER_IPv4\"\n\t}\n\tif dualStack != \"\" {"
count = text.count(old)
if count != 1:
    raise SystemExit(f"v18 Mieru DNS default: expected one anchor, found {count}")
path.write_text(text.replace(old, new, 1), encoding="utf-8")

# Surface the same default in the native form so new Mieru inbounds clearly
# show what the server will use rather than silently relying on backend repair.
path = root / "frontend/src/lib/xray/inbound-defaults.ts"
text = path.read_text(encoding="utf-8")
old = "dnsDualStack: '', dnsHosts: []"
new = "dnsDualStack: 'PREFER_IPv4', dnsHosts: []"
count = text.count(old)
if count != 1:
    raise SystemExit(f"v18 Mieru frontend DNS default: expected one anchor, found {count}")
path.write_text(text.replace(old, new, 1), encoding="utf-8")

print("V18 TUIC native TLS path + Mieru IPv4 preference hotfix applied.")
