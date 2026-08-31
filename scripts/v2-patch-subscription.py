#!/usr/bin/env python3
import sys
from v2_patchlib import Patcher

rep = Patcher(sys.argv[1]).rep

# ---------------------------------------------------------------------------
# Subscription: same subId and ClientInbound relationships, new renderers.
# ---------------------------------------------------------------------------
rep(
    'internal/sub/service.go',
    "inbounds.protocol in ('vmess','vless','trojan','shadowsocks','hysteria','wireguard','amneziawg','mtproto')",
    "inbounds.protocol in ('vmess','vless','trojan','shadowsocks','hysteria','wireguard','amneziawg','mtproto','tuic','anytls','shadowtls','naive')",
)
rep(
    'internal/sub/service.go',
    '''\tcase "amneziawg":\n\t\treturn s.genAmneziaWGLink(inbound, email)\n\t}\n''',
    '''\tcase "amneziawg":\n\t\treturn s.genAmneziaWGLink(inbound, email)\n\tcase "tuic", "anytls", "shadowtls", "naive":\n\t\treturn s.genSingboxLink(inbound, email)\n\t}\n''',
)

# Xray-format JSON subscriptions cannot represent supplemental protocols.
# Skip them rather than emitting a document containing only default outbounds.
rep(
    'internal/sub/json_service.go',
    '''func (s *SubJsonService) getConfig(subReq *SubService, inbound *model.Inbound, client model.Client, host string) []json_util.RawMessage {
\tvar newJsonArray []json_util.RawMessage''',
    '''func (s *SubJsonService) getConfig(subReq *SubService, inbound *model.Inbound, client model.Client, host string) []json_util.RawMessage {
\tif model.IsSingboxProtocol(inbound.Protocol) {
\t\treturn nil
\t}
\tvar newJsonArray []json_util.RawMessage''',
)

# Mihomo/Clash path: use native proxy types where available.
rep(
    'internal/sub/clash_service.go',
    '''func (s *SubClashService) buildProxy(subReq *SubService, inbound *model.Inbound, client model.Client, stream map[string]any, ep map[string]any) map[string]any {\n\t// Hysteria has its own transport + TLS model, applyTransport /''',
    '''func (s *SubClashService) buildProxy(subReq *SubService, inbound *model.Inbound, client model.Client, stream map[string]any, ep map[string]any) map[string]any {\n\tif model.IsSingboxProtocol(inbound.Protocol) {\n\t\treturn s.buildSingboxProxy(subReq, inbound, client, stream, ep)\n\t}\n\t// Hysteria has its own transport + TLS model, applyTransport /''',
)

# Native Client manager and bulk add must list supplemental inbounds as attachable.
for rel in (
    'frontend/src/pages/clients/ClientFormModal.tsx',
    'frontend/src/pages/clients/ClientBulkAddModal.tsx',
):
    rep(
        rel,
        "  'amneziawg',\n]);",
        "  'amneziawg',\n  'tuic',\n  'anytls',\n  'shadowtls',\n  'naive',\n]);",
    )
