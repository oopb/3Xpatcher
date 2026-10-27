#!/usr/bin/env python3
"""Prevent Shadowrocket from importing JSON obfsParam as the HTTP Host.

Standard Xray URIs express the transport with type=xhttp. Shadowrocket's
native split-XHTTP obfsParam is a JSON object and expects obfs=xhttp as
the transport marker. Exporting the object *without* the matching marker
causes clients to treat the JSON string as a generic HTTP Host.

Add only the extra marker where the split-download obfsParam actually exists.
Keep VLESS UUID/encryption/flow/TLS and all Xray extra untouched.
"""
from pathlib import Path
import sys

root=Path(sys.argv[1])
def patch(path,old,new):
    target=root/path
    s=target.read_text()
    n=s.count(old)
    if n != 1:
        raise SystemExit(f"Shadowrocket XHTTP Host fix: expected one anchor in {path}, got {n}")
    target.write_text(s.replace(old,new,1))

# VLESS only. No change for plain XHTTP nodes without split download,
# nor for unrelated transports or protocols. The v32 patch above already
# emits Shadowrocket REALITY download key aliases for TLS uplinks.
patch("internal/sub/service.go",
'''	// A native Shadowrocket split-XHTTP TLS-up/REALITY-down export uses''',
'''	// Recognize JSON obfsParam as XHTTP options, not as a generic HTTP Host.
	// type=xhttp / host=<hostname> / extra remain the canonical Xray URI.
	if streamNetwork == "xhttp" {
		shadowrocketXHTTPMarkNativeTransport(params)
	}
	// A native Shadowrocket split-XHTTP TLS-up/REALITY-down export uses''')

patch("frontend/src/lib/xray/inbound-link.ts",
'''  // Shadowrocket's native split-XHTTP export reads the downlink REALITY''',
'''  // Shadowrocket treats obfsParam as a generic Host string unless the
  // native transport alias is declared alongside Xray's type=xhttp.
  // Emit it only for valid split-download VLESS links.
  if (stream.network === 'xhttp' && params.has('obfsParam')) {
    params.set('obfs', 'xhttp');
  }

  // Shadowrocket's native split-XHTTP export reads the downlink REALITY''')
print("XHTTP split obfsParam is now explicitly marked as Shadowrocket XHTTP")
