#!/usr/bin/env python3
from pathlib import Path
import sys

if len(sys.argv) != 2:
    raise SystemExit(f"Usage: {sys.argv[0]} /path/to/3x-ui-source")

root = Path(sys.argv[1]).resolve()
path = root / "internal/web/web.go"
text = path.read_text(encoding="utf-8")

# 3x-ui v3.8.x ships a native TUIC background job that periodically scans every
# enabled protocol=tuic row and starts its own UDP relay/tuic-server sidecar.
# 3Xpatcher deliberately owns TUIC through the supplemental sing-box runtime.
# Leaving the upstream job enabled makes x-ui bind the public TUIC UDP port first,
# then sing-box fails with EADDRINUSE and the whole supplemental core exits,
# taking AnyTLS/ShadowTLS/Snell/Naive down with it.
block = (
    "\ttuicJob := job.NewTuicJob()\n"
    "\t_, _ = s.cron.AddJob(cadenceTuic, tuicJob)\n"
    "\tgo tuicJob.Run()\n\n"
)

if block in text:
    text = text.replace(block, "", 1)
elif "job.NewTuicJob()" in text:
    raise SystemExit("native TUIC scheduler shape changed; refusing partial isolation patch")
# Older 3x-ui versions do not have the native TUIC scheduler. Treat that as an
# intentional no-op so the compatibility shim remains safe for archived builds.

path.write_text(text, encoding="utf-8")
print("Native 3x-ui TUIC background runtime disabled; TUIC is sing-box-owned.")
