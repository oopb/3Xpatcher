#!/usr/bin/env python3
import sys
from v2_patchlib import Patcher

rep = Patcher(sys.argv[1]).rep

# ShadowTLS v3 is rendered as two sing-box inbounds: an outer ShadowTLS
# listener and an inner loopback Shadowsocks transport. The payload bytes are
# counted on the generated "-inner" tag, not on the outer native 3x-ui tag.
# Track the inner tag in sing-box's v2ray stats API so the collector can fold
# those bytes back into the parent ShadowTLS inbound.
rep(
    'internal/singbox/config.go',
    '''\t\tseenTags[r.Tag] = struct{}{}\n\t\tstatsInbounds = append(statsInbounds, r.Tag)\n\t\tfor _, user := range statsUsersForRecord(r) {''',
    '''\t\tseenTags[r.Tag] = struct{}{}\n\t\tfor _, user := range statsUsersForRecord(r) {''',
)

rep(
    'internal/singbox/config.go',
    '''\t\trendered, extraTags, err := renderInbound(r)\n\t\tif err != nil {\n\t\t\treturn nil, fmt.Errorf("inbound %q: %w", r.Remark, err)\n\t\t}\n\t\tfor _, t := range extraTags {''',
    '''\t\trendered, extraTags, err := renderInbound(r)\n\t\tif err != nil {\n\t\t\treturn nil, fmt.Errorf("inbound %q: %w", r.Remark, err)\n\t\t}\n\t\tif r.Protocol == ProtocolShadowTLS && len(extraTags) > 0 {\n\t\t\tstatsInbounds = append(statsInbounds, extraTags...)\n\t\t} else {\n\t\t\tstatsInbounds = append(statsInbounds, r.Tag)\n\t\t}\n\t\tfor _, t := range extraTags {''',
)

print('V17 ShadowTLS inner traffic stats hotfix applied.')
