#!/usr/bin/env python3
import sys
from v2_patchlib import Patcher

rep = Patcher(sys.argv[1]).rep

# Ant Design 6 uses `orientation` for horizontal/vertical direction and
# `titlePlacement` for the old left/center/right title alignment. V11's first
# pass normalizes legacy left to logical start; finalize it using the v6 API.
rep(
    'frontend/src/pages/inbounds/form/protocols/singbox.tsx',
    'orientation="start"',
    'titlePlacement="start"',
    count=50,
)
rep(
    'frontend/src/pages/inbounds/form/security/tls.tsx',
    'orientation="start"',
    'titlePlacement="start"',
    count=50,
)

# InboundInfoModal has an additional display whitelist after links are generated.
# Include every supplemental protocol so valid native links are not hidden.
rep(
    'frontend/src/pages/inbounds/info/helpers.ts',
    '''  Protocols.HYSTERIA,\n  Protocols.MTPROTO,\n]);''',
    '''  Protocols.HYSTERIA,\n  Protocols.MTPROTO,\n  Protocols.TUIC,\n  Protocols.ANYTLS,\n  Protocols.SHADOWTLS,\n  Protocols.NAIVE,\n  Protocols.MIERU,\n]);''',
)
