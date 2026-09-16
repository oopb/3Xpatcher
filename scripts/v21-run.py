#!/usr/bin/env python3
from pathlib import Path
import subprocess
import sys

if len(sys.argv) != 2:
    raise SystemExit(f"Usage: {sys.argv[0]} /path/to/3x-ui-source")

repo = Path(__file__).resolve().parent.parent
target = Path(sys.argv[1]).resolve()

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
