#!/usr/bin/env python3
from pathlib import Path
import sys

if len(sys.argv) != 2:
    raise SystemExit(f"Usage: {sys.argv[0]} /path/to/3x-ui-source")

root = Path(sys.argv[1]).resolve()
path = root / "frontend/src/pages/inbounds/form/security/tls.tsx"
text = path.read_text(encoding="utf-8")
old = """        setValue('streamSettings.tlsSettings.selfSignedKeyPath', info.keyPath, { shouldDirty: true });
        setValue('streamSettings.tlsSettings.selfSignedNotAfter', info.notAfter, { shouldDirty: true });"""
new = """        setValue('streamSettings.tlsSettings.selfSignedKeyPath', info.keyPath, { shouldDirty: true });
        setValue('streamSettings.tlsSettings.selfSignedNotAfter', info.notAfter, { shouldDirty: true });
        // Store the generated credential in the exact native 3x-ui file-backed
        // certificate shape. This makes the first save pass the same frontend
        // and backend validation as a certificate selected through the built-in
        // path editor, while retaining self_signed_sni metadata for link export.
        setValue('streamSettings.tlsSettings.certificates', [{
          useFile: true,
          certificateFile: info.certificatePath,
          keyFile: info.keyPath,
          certificate: [],
          key: [],
          usage: 'encipherment',
          ocspStapling: 0,
          oneTimeLoading: false,
          buildChain: false,
        }], { shouldDirty: true, shouldValidate: true });"""
count = text.count(old)
if count != 1:
    raise SystemExit(f"v19 native certificate form: expected one anchor, found {count}")
path.write_text(text.replace(old, new, 1), encoding="utf-8")
print("V19 self-signed SNI native 3x-ui certificate-path form hotfix applied.")
