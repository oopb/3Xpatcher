#!/usr/bin/env python3
from pathlib import Path
import subprocess
import sys

if len(sys.argv) != 2:
    raise SystemExit(f"Usage: {sys.argv[0]} /path/to/3x-ui-source")

repo = Path(__file__).resolve().parent.parent
target = Path(sys.argv[1]).resolve()

# V15/V2/V11 temporarily replace the native TUIC settings factory/type with the
# historical 3Xpatcher sing-box shape. Restore the upstream-native shape first;
# V21 then adds only the runtime selector on top of it. Match by function
# boundaries instead of exact formatting because V11 expands the factory.
defaults = target / "frontend/src/lib/xray/inbound-defaults.ts"
text = defaults.read_text(encoding="utf-8")
lines = []
for line in text.splitlines():
    if "from '@/schemas/protocols/inbound/singbox'" in line and "TuicInboundSettings" in line:
        inner = line[line.find('{') + 1: line.find('}')]
        names = [n.strip() for n in inner.split(',') if n.strip() and n.strip() != 'TuicInboundSettings']
        line = "import type { " + ", ".join(names) + " } from '@/schemas/protocols/inbound/singbox';"
    lines.append(line)
text = "\n".join(lines) + "\n"
if "import type { TuicInboundSettings } from '@/schemas/protocols/inbound/tuic';" not in text:
    marker = "import type { TuicClient } from '@/schemas/protocols/inbound/tuic';\n"
    if marker not in text:
        raise SystemExit("v21-run: native TUIC client import anchor missing")
    text = text.replace(marker, marker + "import type { TuicInboundSettings } from '@/schemas/protocols/inbound/tuic';\n", 1)

native_factory = """export function createDefaultTuicInboundSettings(): TuicInboundSettings {
  return {
    server: {
      certificate: '',
      private_key: '',
      congestion_control: 'bbr',
      alpn: ['h3', 'spdy/3.1'],
      udp_relay_mode: 'native',
      zero_rtt_handshake: true,
      log_level: 'info',
      max_idle_time: 15,
      authentication_timeout: 3,
      max_udp_relay_packet_size: 1500,
      sni: '',
    },
    clients: [],
  };
}

"""
start_marker = "export function createDefaultTuicInboundSettings(): TuicInboundSettings {"
end_marker = "export function createDefaultAnyTlsInboundSettings(): AnyTlsInboundSettings {"
start = text.find(start_marker)
end = text.find(end_marker, start + 1) if start >= 0 else -1
if start < 0 or end < 0:
    raise SystemExit(
        f"v21-run: TUIC defaults function boundary missing (start={start >= 0}, end={end >= 0})"
    )
text = text[:start] + native_factory + text[end:]
defaults.write_text(text, encoding="utf-8")

for src_rel, dst_rel in (
    ("overlay/internal/database/model/tuic_runtime.go", "internal/database/model/tuic_runtime.go"),
    ("overlay/internal/singbox/tuic_runtime.go", "internal/singbox/tuic_runtime.go"),
    ("overlay/internal/sub/tuic_runtime.go", "internal/sub/tuic_runtime.go"),
):
    src = repo / src_rel
    dst = target / dst_rel
    dst.parent.mkdir(parents=True, exist_ok=True)
    dst.write_bytes(src.read_bytes())

subprocess.run(
    [sys.executable, str(repo / "scripts/v21-tuic-runtime-selector.py"), str(target)],
    check=True,
)

# TUIC now owns certificate/SNI fields in its native protocol editor for both
# runtimes; do not show the generic Xray TLS Security editor for TUIC.
cap = target / "frontend/src/lib/xray/protocol-capabilities.ts"
text = cap.read_text(encoding="utf-8")
text = text.replace(
    "if (values.protocol === 'hysteria' || ['tuic', 'anytls', 'naive'].includes(values.protocol)) return true;",
    "if (values.protocol === 'hysteria' || ['anytls', 'naive'].includes(values.protocol)) return true;",
)
cap.write_text(text, encoding="utf-8")

modal = target / "frontend/src/pages/inbounds/form/InboundFormModal.tsx"
text = modal.read_text(encoding="utf-8")
text = text.replace(
    "const SINGBOX_TLS_PROTOCOLS = [Protocols.TUIC, Protocols.ANYTLS, Protocols.NAIVE] as string[];",
    "const SINGBOX_TLS_PROTOCOLS = [Protocols.ANYTLS, Protocols.NAIVE] as string[];",
)
anchor = "      } else if (SINGBOX_TLS_PROTOCOLS.includes(next)) {\n"
if anchor not in text:
    raise SystemExit("v21-run: TUIC stream reset anchor missing")
text = text.replace(
    anchor,
    "      } else if (next === Protocols.TUIC) {\n        setV('streamSettings', { security: 'none' });\n      } else if (SINGBOX_TLS_PROTOCOLS.includes(next)) {\n",
    1,
)
modal.write_text(text, encoding="utf-8")

# TUIC always uses TLS, but display the security badge only when the native
# certificate/key paths are actually configured.
columns = target / "frontend/src/pages/inbounds/list/useInboundColumns.tsx"
text = columns.read_text(encoding="utf-8")
old = """          if (record.protocol === 'tuic') {
            tags.push(<Tag key="n" color="green">UDP</Tag>);
            pushSupplementalSecurity();
          } else if (record.protocol === 'anytls') {"""
new = """          if (record.protocol === 'tuic') {
            tags.push(<Tag key="n" color="green">UDP</Tag>);
            const tuicServer = (supplementalSettings.server || {}) as Record<string, unknown>;
            if (String(tuicServer.certificate || '').trim() && String(tuicServer.private_key || '').trim()) {
              tags.push(<Tag key="tls" color="blue">TLS</Tag>);
            }
          } else if (record.protocol === 'anytls') {"""
if old not in text:
    raise SystemExit("v21-run: TUIC badge anchor missing")
columns.write_text(text.replace(old, new, 1), encoding="utf-8")
