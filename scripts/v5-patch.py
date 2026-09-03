#!/usr/bin/env python3
import sys
from v2_patchlib import Patcher

rep = Patcher(sys.argv[1]).rep

# AnyTLS is the supplemental protocol in our current set with end-to-end
# sing-box Reality support. TUIC is QUIC (custom TLS supports ECH only),
# ShadowTLS has no InboundTLSOptions container, and sing-box explicitly rejects
# Reality on Naive outbound. Reuse the existing 3x-ui Reality tab for AnyTLS.
rep(
    'frontend/src/lib/xray/protocol-capabilities.ts',
    '''export function canEnableReality(values: CapabilityProtocolSlice): boolean {\n  if (!REALITY_ELIGIBLE_PROTOCOLS.includes(values.protocol)) return false;\n  return REALITY_NETWORKS.includes(values.streamSettings?.network ?? '');\n}''',
    '''export function canEnableReality(values: CapabilityProtocolSlice): boolean {\n  if (values.protocol === 'anytls') return true;\n  if (!REALITY_ELIGIBLE_PROTOCOLS.includes(values.protocol)) return false;\n  return REALITY_NETWORKS.includes(values.streamSettings?.network ?? '');\n}''',
)
