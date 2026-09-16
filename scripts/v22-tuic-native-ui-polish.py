#!/usr/bin/env python3
from pathlib import Path
import subprocess
import sys

if len(sys.argv) != 2:
    raise SystemExit(f"Usage: {sys.argv[0]} /path/to/3x-ui-source")

root = Path(sys.argv[1]).resolve()

# TUIC now reuses the upstream frontend link generator for both server runtimes.
# Runtime selection only chooses the server implementation; it must not create
# a second client/share-link format in the browser. v21-run.py already owns the
# native TUIC TLS badge and native certificate controls.
path = root / "frontend/src/lib/xray/supplemental-links.ts"
text = path.read_text(encoding="utf-8")
old = '''    case 'tuic': {
      if (!client.id || !client.password) return [];
      const params = new URLSearchParams();
      applyTlsParams(inbound, externalProxy, params);
      const congestion = asString(settings.congestionControl);
      if (congestion) params.set('congestion_control', congestion);
      if (asBoolean(settings.zeroRTTHandshake)) params.set('zero_rtt_handshake', '1');
      const authority = `${encodeUserinfo(client.id)}:${encodeUserinfo(client.password)}@${host}:${port}`;
      return [{ link: buildLink(`tuic://${authority}`, params, remark), label: 'TUIC' }];
    }
'''
new = '''    case 'tuic':
      // Native and sing-box runtimes intentionally share the upstream TUIC
      // settings/client shape, so fall through to inbound-link.ts genTuicLink.
      return [];
'''
count = text.count(old)
if count != 1:
    raise SystemExit(f"v22 TUIC browser link ownership: expected one anchor, found {count}")
path.write_text(text.replace(old, new, 1), encoding="utf-8")

print("V22 native TUIC browser-link ownership polish applied.")

subprocess.run(
    [sys.executable, str(Path(__file__).resolve().parent / 'v23-tuic-selfsigned-export.py'), sys.argv[1]],
    check=True,
)
subprocess.run(
    [sys.executable, str(Path(__file__).resolve().parent / 'v24-supplemental-form-layout.py'), sys.argv[1]],
    check=True,
)
