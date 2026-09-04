#!/usr/bin/env python3
import sys
from v2_patchlib import Patcher

rep = Patcher(sys.argv[1]).rep

# ---------------------------------------------------------------------------
# Native inbound list parity: supplemental protocols are multi-user and must
# participate in the same client rollup used by the upstream row-action menu.
# Without this, hasClients remains false and native detach/group/delete-all
# actions disappear even though client_inbounds contains attached clients.
# ---------------------------------------------------------------------------
rep(
    'frontend/src/pages/inbounds/useInbounds.ts',
    '''  Protocols.AMNEZIAWG,\n];''',
    '''  Protocols.AMNEZIAWG,\n  Protocols.TUIC,\n  Protocols.ANYTLS,\n  Protocols.SHADOWTLS,\n  Protocols.NAIVE,\n];''',
)

# Use the column's existing fallbackClientCount() as well as the computed map.
# This keeps the menu correct during slim-query hydration / transient rollup
# gaps instead of hiding actions until another client-count refresh completes.
rep(
    'frontend/src/pages/inbounds/list/useInboundColumns.tsx',
    '''            hasClients={(clientCount[record.id]?.clients || 0) > 0}''',
    '''            hasClients={clientTotal(record) > 0}''',
)
