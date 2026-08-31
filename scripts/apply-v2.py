#!/usr/bin/env python3
from pathlib import Path
import subprocess
import sys

if len(sys.argv) != 2:
    raise SystemExit(f"Usage: {sys.argv[0]} /path/to/3x-ui-source")

base = Path(__file__).resolve().parent
target = sys.argv[1]
for name in (
    "v2-patch-backend.py",
    "v2-patch-subscription.py",
    "v2-patch-frontend.py",
):
    subprocess.run([sys.executable, str(base / name), target], check=True)

print("V2 integrated patch points applied.")
