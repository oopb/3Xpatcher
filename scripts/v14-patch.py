#!/usr/bin/env python3
from pathlib import Path
import sys

if len(sys.argv) != 2:
    raise SystemExit("usage: v14-patch.py /path/to/3x-ui-source")

root = Path(sys.argv[1])
path = root / "internal/singbox/config.go"
if not path.is_file():
    raise SystemExit("v14 missing target file: internal/singbox/config.go")

text = path.read_text(encoding="utf-8")
render_marker = "func renderInbound(r InboundRecord) ([]any, []string, error) {\n"
render_pos = text.find(render_marker)
if render_pos < 0:
    raise SystemExit("v14 could not locate renderInbound")

snell_block = '''\tcase ProtocolSnell:\n\t\tvar s SnellSettings\n\t\tif err := json.Unmarshal(r.Settings, &s); err != nil {\n\t\t\treturn nil, nil, err\n\t\t}\n\t\tversion := s.Version\n\t\tif version == 0 {\n\t\t\tversion = 5\n\t\t}\n\t\tif version != 5 {\n\t\t\treturn nil, nil, fmt.Errorf("3Xpatcher Snell compatibility mode requires version 5, got %d", version)\n\t\t}\n\t\tpsk := strings.TrimSpace(s.PSK)\n\t\tif psk == "" {\n\t\t\treturn nil, nil, errors.New("Snell PSK is required")\n\t\t}\n\t\tobfsMode := strings.TrimSpace(s.ObfsMode)\n\t\tif obfsMode == "" {\n\t\t\tobfsMode = "none"\n\t\t}\n\t\tif obfsMode != "none" && obfsMode != "http" {\n\t\t\treturn nil, nil, fmt.Errorf("invalid Snell obfs mode %q", obfsMode)\n\t\t}\n\t\tm := baseInbound("snell", r)\n\t\tapplyListen(m, s.ListenSettings)\n\t\tm["version"] = 5\n\t\tm["psk"] = psk\n\t\tif obfsMode == "http" {\n\t\t\tm["obfs_mode"] = "http"\n\t\t}\n\t\treturn []any{m}, nil, nil\n'''

prefix = text[:render_pos]
suffix = text[render_pos:]

# v13 used a generic `case ProtocolShadowTLS:` anchor. After v7 added
# statsUsersForRecord(), that first match is outside renderInbound. Remove only
# that misplaced renderer block from the pre-renderInbound prefix.
wrong = snell_block + "\tcase ProtocolShadowTLS:\n"
if wrong not in prefix:
    raise SystemExit("v14 did not find the misplaced v13 Snell renderer before renderInbound")
prefix = prefix.replace(wrong, "\tcase ProtocolShadowTLS:\n", 1)

# Reinsert the exact same renderer in the protocol-rendering switch only.
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

path.write_text(out, encoding="utf-8")
print("v14 Snell renderer placement fix applied")
