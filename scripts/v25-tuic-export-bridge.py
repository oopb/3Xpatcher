#!/usr/bin/env python3
from pathlib import Path
import sys

root = Path(sys.argv[1]).resolve()
path = root / 'frontend/src/pages/inbounds/form/protocols/index.ts'
text = path.read_text(encoding='utf-8')
old = "export { AnyTlsFields, MieruFields, NaiveFields, ShadowTlsFields, ShadowTlsSecurityFields, SnellFields } from './singbox';"
new = "export { AnyTlsFields, MieruFields, NaiveFields, ShadowTlsFields, ShadowTlsSecurityFields, SnellFields, TuicFields } from './singbox';"
if text.count(old) != 1:
    raise SystemExit(f'v25 export bridge: expected final singbox export once, found {text.count(old)}')
path.write_text(text.replace(old, new, 1), encoding='utf-8')
