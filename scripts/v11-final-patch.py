#!/usr/bin/env python3
import sys
from v2_patchlib import Patcher

rep = Patcher(sys.argv[1]).rep

# InboundInfoModal has an additional display whitelist after links are generated.
# Include every supplemental protocol so valid native links are not hidden.
rep(
    'frontend/src/pages/inbounds/info/helpers.ts',
    '''  Protocols.HYSTERIA,\n  Protocols.MTPROTO,\n]);''',
    '''  Protocols.HYSTERIA,\n  Protocols.MTPROTO,\n  Protocols.TUIC,\n  Protocols.ANYTLS,\n  Protocols.SHADOWTLS,\n  Protocols.NAIVE,\n  Protocols.MIERU,\n]);''',
)
