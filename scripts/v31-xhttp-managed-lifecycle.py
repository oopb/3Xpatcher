#!/usr/bin/env python3
"""Route managed XHTTP listener changes through the native config reconciler.

Vanilla local runtime handles one inbound tag, whereas an opted-in XHTTP
gateway expands to 3 inbounds. A native DelInbound only removes the base
tag, leaving TLS and REALITY synthetic ports open. Mark gateway mutations
for the native RestartXray desired-config reconciliation instead.
"""
from pathlib import Path
import sys
path=Path(sys.argv[1])/"internal/web/service/inbound.go"
s=path.read_text()
def patch(before,after,why):
    global s
    count=s.count(before)
    if count!=1:
        raise SystemExit(f"XHTTP lifecycle anchor {why} found {count} times")
    s=s.replace(before,after,1)

patch(
"""\t\t\tif inbound.NodeID != nil {
\t\t\t\tmarkDirty = true
\t\t\t} else {
\t\t\t\trt, push, _, perr := s.nodePushPlan(inbound)""",
"""\t\t\tif inbound.NodeID != nil {
\t\t\t\tmarkDirty = true
\t\t\t} else if managedXHTTPNeedsReconcile(nil, inbound) {
\t\t\t\t// The generated triple, not this single DB tag, owns the sockets.
\t\t\t\tneedRestart = true
\t\t\t} else {
\t\t\t\trt, push, _, perr := s.nodePushPlan(inbound)""",
"add inbound")

patch(
"""\t\tif shouldPushToRuntime {
\t\t\tif ib.NodeID != nil {
\t\t\t\trt, push, _, perr := s.nodePushPlan(&ib)""",
"""\t\tif shouldPushToRuntime {
\t\t\tif ib.NodeID == nil && managedXHTTPNeedsReconcile(&ib, nil) {
\t\t\t\tneedRestart = true
\t\t\t} else if ib.NodeID != nil {
\t\t\t\trt, push, _, perr := s.nodePushPlan(&ib)""",
"delete inbound")

patch(
"""\tif mtprotoRoutesThroughXray(inbound) {
\t\tneedRestart = true
\t}

\tif !push {
\t\treturn true, nil
\t}""",
"""\tif managedXHTTPNeedsReconcile(inbound, nil) {
\t\treturn true, nil
\t}
\tif mtprotoRoutesThroughXray(inbound) {
\t\tneedRestart = true
\t}

\tif !push {
\t\treturn true, nil
\t}""",
"enable toggle")

patch(
"""\t\tif oldInbound.NodeID == nil {
\t\t\trt, push, _, perr := s.nodePushPlan(oldInbound)""",
"""\t\tif oldInbound.NodeID == nil && managedXHTTPNeedsReconcile(&running, oldInbound) {
\t\t\t// The native single-tag Del/Add would retain previously generated
\t\t\t// TLS and REALITY listeners. The controller's normal pending restart
\t\t\t// handles removing obsolete ports and deploying the full new triple.
\t\t\tneedRestart = true
\t\t} else if oldInbound.NodeID == nil {
\t\t\trt, push, _, perr := s.nodePushPlan(oldInbound)""",
"update inbound")

path.write_text(s)
print("Patched managed XHTTP create/edit/enable/delete lifecycle")
