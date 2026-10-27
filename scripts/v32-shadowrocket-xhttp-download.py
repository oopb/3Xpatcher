#!/usr/bin/env python3
"""Preserve standard Xray VLESS URI, add only Shadowrocket XHTTP downlink keys.

A native Shadowrocket TLS-up/REALITY-down XHTTP export carries downlink
publicKey/shortId as top-level pbk/sid, while its obfsParam carries the
target/SNI/HTTP Host. Do not rewrite VLESS userinfo, encryption, type,
TLS or flow. Most importantly, do not replace an upstream REALITY pbk/sid.
"""
from pathlib import Path
import sys
root=Path(sys.argv[1])
def patch(path,old,new,reason):
    p=root/path
    s=p.read_text()
    found=s.count(old)
    if found != 1:
        raise SystemExit(f"{reason}: expected one anchor in {path}, found {found}")
    p.write_text(s.replace(old,new,1))

# The upstream security is known here and has just been serialized. Only for
# VLESS with a TLS uplink, add Shadowrocket's separate REALITY credentials
# without changing any Xray standard fields or other protocols.
patch("internal/sub/service.go",
"""	if len(client.Flow) > 0 && !inbound.DisableFlow && vlessFlowAllowed(streamNetwork, security, settings) {
		params["flow"] = client.Flow
	}

	externalProxies, _ := stream["externalProxy"].([]any)""",
"""	if len(client.Flow) > 0 && !inbound.DisableFlow && vlessFlowAllowed(streamNetwork, security, settings) {
		params["flow"] = client.Flow
	}
	// A native Shadowrocket split-XHTTP TLS-up/REALITY-down export uses
	// top-level pbk/sid for its download-side handshake. Preserve Xray's
	// standard extra.downloadSettings and all TLS/Encryption/Flow fields.
	if streamNetwork == "xhttp" && security == "tls" {
		xhttp, _ := stream["xhttpSettings"].(map[string]any)
		shadowrocketXHTTPDownloadRealityQueryParams(xhttp, params)
	}

	externalProxies, _ := stream["externalProxy"].([]any)""",
"VLESS TLS-up/REALITY-down query bridge")

# Mirror the same fields in the panel's per-inbound QR/share link. This
# deliberately runs only inside genVlessLink, not the Trojan/VMess exporters.
patch("frontend/src/lib/xray/inbound-link.ts",
"""  // Same gate as the backend's vlessFlowAllowed and the form's flow field, so the
  // panel link, share link and subscription agree.""",
"""  // Shadowrocket's native split-XHTTP export reads the downlink REALITY
  // public key / short ID from the URI query. Do not rewrite the Xray
  // standard extra.downloadSettings or the VLESS Encryption userinfo.
  if (stream.network === 'xhttp' && security === 'tls') {
    const download = stream.xhttpSettings.downloadSettings;
    if (download?.security === 'reality' && download.realitySettings?.publicKey) {
      params.set('pbk', download.realitySettings.publicKey);
      if (download.realitySettings.shortId) params.set('sid', download.realitySettings.shortId);
    }
  }

  // Same gate as the backend's vlessFlowAllowed and the form's flow field, so the
  // panel link, share link and subscription agree.""",
"VLESS panel QR Shadowrocket REALITY keys")

# Native Shadowrocket exports use the parent 'path' when both halves share a
# single XHTTP session. Do not output a redundant downloadPath in obfsParam.
# The canonical Xray extra.downloadSettings.xhttpSettings.path stays intact.
patch("frontend/src/lib/xray/inbound-link.ts",
"""      downloadPath: xhttp.path || '',
""",
"",
"remove non-native redundant obfsParam downloadPath")

print("Shadowrocket XHTTP REALITY download keys mapped, Xray URI unchanged")
