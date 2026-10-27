#!/usr/bin/env python3
from pathlib import Path
import sys

path = Path(sys.argv[1]) / "internal/web/service/xray.go"
s = path.read_text()
old = """\tfor _, inbound := range inbounds {
\t\tif !inbound.Enable {"""
new = """\t// Preserve managed download metadata before v26 strips client-only
\t// keys from the runtime inlet StreamSettings.
\txhttpDownloadSnapshots := make(map[string]map[string]any)
\tfor _, row := range inbounds {
\t\tif row == nil || !row.Enable || row.NodeID != nil { continue }
\t\tstream := xhttpMap([]byte(row.StreamSettings))
\t\txhttp, _ := stream["xhttpSettings"].(map[string]any)
\t\tdown, _ := xhttp["downloadSettings"].(map[string]any)
\t\tmanaged, _ := down["managedListener"].(map[string]any)
\t\tif managed["enabled"] == true { xhttpDownloadSnapshots[row.Tag] = down }
\t}
\tfor _, inbound := range inbounds {
\t\tif !inbound.Enable {"""
assert s.count(old) == 1, "failed locating GetXrayConfig inbound loop"
s=s.replace(old,new,1)
old="injectManagedXHTTPGateways(xrayConfig, inbounds)"
assert s.count(old)==1,"failed locating managed injection"
s=s.replace(old,"injectManagedXHTTPGateways(xrayConfig, inbounds, xhttpDownloadSnapshots)",1)
path.write_text(s)
print("Retain client-only managed metadata through server listener generation")
