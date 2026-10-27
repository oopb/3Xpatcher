#!/usr/bin/env python3
"""Pass the actual upload security into Mihomo XHTTP download conversion.

The uplink type decides whether a TLS downlink needs to clear inherited
REALITY credentials. A TLS/TLS split never has any REALITY inheritance.
"""
from pathlib import Path
import sys
p=Path(sys.argv[1])/"internal/sub/clash_service.go"
s=p.read_text()
a="func buildXhttpClashOpts(xhttp map[string]any) map[string]any {"
b="func buildXhttpClashOpts(xhttp map[string]any, uplinkSecurity ...string) map[string]any {"
assert s.count(a)==1,"missing upstream buildXhttpClashOpts signature"
s=s.replace(a,b,1)
a="if converted := xhttpDownloadMihomo(download); len(converted) > 0 {"
b='if converted := xhttpDownloadMihomo(download, uplinkSecurity...); len(converted) > 0 {'
assert s.count(a)==1,"missing injected download renderer"
s=s.replace(a,b,1)
a='opts := buildXhttpClashOpts(xhttp)'
b='uplinkSecurity, _ := stream["security"].(string)\n\t\topts := buildXhttpClashOpts(xhttp, uplinkSecurity)'
assert s.count(a)==1,"missing upstream XHTTP applyTransport"
s=s.replace(a,b,1)
p.write_text(s)
print("Mihomo XHTTP now derives REALITY inheritance from actual uplink security")
