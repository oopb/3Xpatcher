#!/usr/bin/env python3
import sys
from v2_patchlib import Patcher

rep = Patcher(sys.argv[1]).rep

# ---------------------------------------------------------------------------
# Complete Mieru defaults for every official server-side option represented by
# the integrated settings schema. Arrays must always start present so RHF
# useFieldArray can append without repairing undefined values first.
# ---------------------------------------------------------------------------
rep(
    'frontend/src/lib/xray/inbound-defaults.ts',
    "return { clients: [], transport: 'TCP', portRangeEnd: 0, mtu: 1400, loggingLevel: 'INFO', allowPrivateIP: false, allowLoopbackIP: false, quotaDays: 0, quotaMegabytes: 0, metricsLoggingInterval: '', userHintIsMandatory: false, trafficPatternEnabled: false, trafficSeed: 0, trafficUnlockAll: false, tcpFragmentEnable: false, tcpFragmentMaxSleepMs: 0, nonceType: '', nonceApplyToAllUDP: false, nonceMinLen: 0, nonceMaxLen: 0, nonceCustomHexStrings: [], lowEntropyMode: 'LOW_ENTROPY_MODE_OFF', lowEntropyMaskRotation: 'LOW_ENTROPY_MASK_NO_ROTATION', clientMultiplexing: 'MULTIPLEXING_LOW', clientHandshakeMode: 'HANDSHAKE_STANDARD', clientTrafficPattern: '' };",
    "return { clients: [], transport: 'TCP', portRangeEnd: 0, additionalPortBindings: [], mtu: 1400, loggingLevel: 'INFO', allowPrivateIP: false, allowLoopbackIP: false, quotaDays: 0, quotaMegabytes: 0, metricsLoggingInterval: '', userHintIsMandatory: false, dnsDualStack: '', dnsHosts: [], egressProxies: [], egressRules: [], trafficPatternEnabled: false, trafficSeed: 0, trafficUnlockAll: false, tcpFragmentEnable: false, tcpFragmentMaxSleepMs: 0, nonceType: '', nonceApplyToAllUDP: false, nonceMinLen: 0, nonceMaxLen: 0, nonceCustomHexStrings: [], lowEntropyMode: 'LOW_ENTROPY_MODE_OFF', lowEntropyMaskRotation: 'LOW_ENTROPY_MASK_NO_ROTATION', clientMultiplexing: 'MULTIPLEXING_LOW', clientHandshakeMode: 'HANDSHAKE_STANDARD', clientTrafficPattern: '' };",
)

# ---------------------------------------------------------------------------
# A Mieru simple-sharing profile may contain several port/protocol pairs, while
# one Mihomo Mieru proxy can represent only one single port or one range. When
# there is no host-level external endpoint override, expand one native inbound
# into one Mihomo proxy per official portBinding instead of dropping extras.
# ---------------------------------------------------------------------------
rep(
    'internal/sub/clash_service.go',
    '''\texternalProxies, ok := stream["externalProxy"].([]any)\n\thasExternalProxy := ok && len(externalProxies) > 0\n\tif !hasExternalProxy {''',
    '''\texternalProxies, ok := stream["externalProxy"].([]any)\n\thasExternalProxy := ok && len(externalProxies) > 0\n\tif inbound.Protocol == model.Mieru && !hasExternalProxy {\n\t\treturn s.buildMieruProxies(subReq, inbound, client)\n\t}\n\tif !hasExternalProxy {''',
)

# Make generic fallback names useful for port-range-only proxies too.
rep(
    'internal/sub/clash_service.go',
    '''\tif typ != "" && server != "" {\n\t\treturn fmt.Sprintf("%s-%s-%v", typ, server, proxy["port"])\n\t}''',
    '''\tif typ != "" && server != "" {\n\t\tif portRange, ok := proxy["port-range"]; ok {\n\t\t\treturn fmt.Sprintf("%s-%s-%v", typ, server, portRange)\n\t\t}\n\t\treturn fmt.Sprintf("%s-%s-%v", typ, server, proxy["port"])\n\t}''',
)
