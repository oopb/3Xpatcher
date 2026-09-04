#!/usr/bin/env bash
set -euo pipefail
ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT"

echo '[1/8] renderer unit tests'
go test ./internal/singbox ./internal/mieru

echo '[2/8] script syntax'
bash -n install.sh rollback.sh scripts/*.sh tests/*.sh
python3 -m py_compile scripts/apply-v2.py scripts/v2_patchlib.py scripts/v2-patch-*.py scripts/v3-patch.py scripts/v5-patch.py scripts/v6-patch.py scripts/v7-patch.py scripts/v8-patch.py scripts/v9-patch.py scripts/v10-patch.py scripts/v11-patch.py scripts/v11-final-patch.py scripts/v12-patch.py

echo '[3/8] integrated protocol surface'
python3 - <<'PY'
from pathlib import Path
singbox={'tuic','anytls','shadowtls','naive'}
settings=Path('overlay/frontend/src/schemas/protocols/inbound/singbox.ts').read_text()
fields=Path('overlay/frontend/src/pages/inbounds/form/protocols/singbox.tsx').read_text()
model=Path('overlay/internal/database/model/singbox_protocols.go').read_text()
renderer=Path('internal/singbox/config.go').read_text()
links=Path('overlay/frontend/src/lib/xray/supplemental-links.ts').read_text()
for p in singbox:
    if p not in model or p not in renderer:
        raise SystemExit(f'missing sing-box backend protocol {p}')
for p in ('tuic','anytls','shadowtls','naive','mieru'):
    if f"case '{p}':" not in links:
        raise SystemExit(f'missing browser share-link protocol {p}')
if 'mieru' not in model or 'MieruInboundSettingsSchema' not in settings or 'MieruFields' not in fields:
    raise SystemExit('missing Mieru integrated surface')
for token in ('additionalPortBindings','dnsDualStack','dnsHosts','egressProxies','egressRules'):
    if token not in settings or token not in fields:
        raise SystemExit(f'missing complete Mieru field {token}')
for component in ('TuicFields','AnyTlsFields','ShadowTlsFields','NaiveFields','MieruFields'):
    if component not in fields:
        raise SystemExit(f'missing frontend component {component}')
if 'clients:' not in settings:
    raise SystemExit('supplemental frontend settings must retain native clients mirror')
print('protocol surface: OK')
PY

echo '[4/8] native identity/subscription/stats integration guards'
grep -q 'JOIN client_inbounds AS ci' overlay/internal/singbox/integrated.go
grep -q 'JOIN client_inbounds AS ci' overlay/internal/mieru/integrated.go
grep -q 'genSingboxLink' overlay/internal/sub/singbox_links.go
grep -q 'genMieruLink' overlay/internal/sub/mieru_links.go
grep -q 'additionalPortBindings' overlay/internal/sub/mieru_links.go
grep -q 'buildMieruProxy' overlay/internal/sub/mieru_clash.go
grep -q 'buildMieruProxies' overlay/internal/sub/mieru_clash.go
grep -q 'model.Mieru' scripts/v6-patch.py
grep -q 'IsSupplementalProtocol' scripts/v6-patch.py
grep -q 'MieruFields' scripts/v6-patch.py
grep -q 'mierus' overlay/internal/sub/mieru_links.go
grep -q '"type": *"mieru"' overlay/internal/sub/mieru_clash.go
grep -q 'hashedPassword' internal/mieru/config.go
grep -q 'AdditionalPortBindings' internal/mieru/config.go
grep -q 'buildDNS' internal/mieru/config.go
grep -q 'buildEgress' internal/mieru/config.go
grep -q 'SOCKS5_PROXY_PROTOCOL' internal/mieru/config.go
grep -q 'MITA_CONFIG_JSON_FILE' scripts/install-mieru.sh
grep -q 'CollectTraffic' overlay/internal/singbox/stats.go
grep -q 'CollectTraffic' overlay/internal/mieru/stats.go
grep -q 'NewSupplementalTrafficJob' overlay/internal/web/job/supplemental_traffic_job.go
grep -q 'cadenceSupplementalTraffic' scripts/v7-patch.py
grep -q 'v2ray_api' scripts/v7-patch.py
grep -q 'with_v2ray_api' .github/workflows/prebuild.yml
grep -q 'SINGBOX_VERSION' .github/workflows/prebuild.yml
grep -q 'buildMieruProxies' scripts/v10-patch.py
for token in 'Protocols.TUIC' 'Protocols.ANYTLS' 'Protocols.SHADOWTLS' 'Protocols.NAIVE' 'Protocols.MIERU'; do
  grep -q "$token" scripts/v11-patch.py
done
grep -q 'hasClients={clientTotal(record) > 0}' scripts/v11-patch.py

# Client formats are based on S-UI/sing-box behavior rather than a fabricated
# universal URI. Shadowrocket gets its established descriptor ShadowTLS form;
# Naive gets S-UI's http2 compatibility form.
grep -q "label: 'ShadowTLS / Shadowrocket'" overlay/frontend/src/lib/xray/supplemental-links.ts
grep -q "params.set('shadow-tls'" overlay/frontend/src/lib/xray/supplemental-links.ts
grep -q 'buildShadowrocketShadowTLSLink' overlay/internal/sub/singbox_links.go
grep -q 'shadow-tls.*base64' overlay/internal/sub/singbox_links.go
grep -q 'buildShadowrocketNaiveHTTP2Link' overlay/internal/sub/singbox_links.go
grep -q '"http2://"' overlay/internal/sub/singbox_links.go
grep -q 'params\["peer"\]' overlay/internal/sub/singbox_links.go
grep -q 'params\["padding"\] = "1"' overlay/internal/sub/singbox_links.go
grep -q 'mierus://' overlay/frontend/src/lib/xray/supplemental-links.ts
grep -q 'naive+https' overlay/frontend/src/lib/xray/supplemental-links.ts
grep -q 'supplemental-links.ts' scripts/apply-overlay.sh
grep -q 'supplemental-links.ts' scripts/revert-overlay.sh
grep -q 'frontend/src/lib/xray/inbound-link.ts' scripts/apply-overlay.sh
grep -q 'frontend/src/lib/xray/inbound-link.ts' scripts/revert-overlay.sh
grep -q 'scripts/v11-patch.py' scripts/apply-overlay.sh
grep -q 'scripts/v12-patch.py' scripts/apply-overlay.sh
grep -q 'clientUserAgent' scripts/v12-patch.py
grep -q 'internal/sub/controller.go' scripts/apply-overlay.sh
grep -q 'internal/sub/controller.go' scripts/revert-overlay.sh

# V11.5: TUIC dedicated Clash output must match the last known-good pre-Mieru
# generator. Preserve the inbound TLS ALPN when configured and never synthesize
# h3 when it was absent. Server-side sing-box knobs must not leak into Mihomo.
! grep -q 'proxy\["alpn"\] = \[\]string{"h3"}' overlay/internal/sub/singbox_clash.go
grep -q 'applyTLS()' overlay/internal/sub/singbox_clash.go
grep -q 'TestTUICClashRestoresExactPreMieruShape' overlay/internal/sub/singbox_links_test.go
grep -q 'TestTUICClashDoesNotInventALPN' overlay/internal/sub/singbox_links_test.go
! grep -q 'heartbeat-interval' overlay/internal/sub/singbox_clash.go
! grep -q 'udp-relay-mode' overlay/internal/sub/singbox_clash.go
! grep -q 'max-open-streams' overlay/internal/sub/singbox_clash.go
! grep -q 'disable-mtu-discovery' overlay/internal/sub/singbox_clash.go
! grep -q 'disable-sni' overlay/internal/sub/singbox_clash.go

# Native 3x-ui TLS files are often under /root, and restricted LXC/NAT VPSes may
# reject ProtectHome mount namespacing. Do not hide/remount home in this unit.
grep -q '^ProtectHome=false$' scripts/install-singbox.sh
! grep -q '^ProtectHome=true$' scripts/install-singbox.sh
! grep -q '^ProtectHome=read-only$' scripts/install-singbox.sh
# A Type=simple restart can look successful before the process exits. The
# installer must print diagnostics and restore the previous runtime/unit.
grep -q 'rollback_runtime' scripts/install-singbox.sh
grep -q 'journalctl -u x-ui-singbox.service -n 100' scripts/install-singbox.sh
grep -q 'startup stabilization' scripts/install-singbox.sh
grep -q 'x-ui-singbox.service.old' scripts/install-singbox.sh
# Clash/Mihomo standalone ShadowTLS remains SS + shadow-tls plugin; Naive has
# no Mihomo proxy type and must not be faked as HTTPS.
grep -q 'proxy\["plugin"\] = "shadow-tls"' overlay/internal/sub/singbox_clash.go
grep -q 'client-fingerprint' overlay/internal/sub/singbox_clash.go
grep -q 'case model.Naive:' overlay/internal/sub/singbox_clash.go

echo '[5/8] Mieru runtime isolation guards'
grep -q 'x-ui-mieru@%d.service' internal/mieru/runtime.go
grep -q 'x-ui-mieru@.service' scripts/install-mieru.sh
grep -q 'MITA_UDS_PATH=/run/x-ui-mieru/%i.sock' scripts/install-mieru.sh
grep -q '^User=mita$' scripts/install-mieru.sh
grep -q '^Group=mita$' scripts/install-mieru.sh
grep -q '^AmbientCapabilities=CAP_NET_BIND_SERVICE$' scripts/install-mieru.sh
grep -q '^StateDirectory=x-ui-mieru/%i$' scripts/install-mieru.sh
grep -q '^BindPaths=/var/lib/x-ui-mieru/%i:/var/lib/mita$' scripts/install-mieru.sh
grep -q 'install -d -m 2750 -o root -g mita' scripts/install-mieru.sh
grep -q 'tmp.Chmod(0o640)' internal/mieru/runtime.go
grep -q 'DefaultConfigDir.*x-ui-mieru/config' internal/mieru/runtime.go
grep -q 'ConfiguredIDs' overlay/internal/mieru/integrated.go
grep -q 'mcore.Reconcile' scripts/v6-patch.py
grep -q 'IsMieruProtocol' scripts/v6-patch.py
grep -q 'MIERU_VERSION' install.sh
grep -q 'PURGE_MIERU' rollback.sh

echo '[6/8] Reality + automatic SS2022 key guards'
grep -q "values.protocol === 'anytls'" scripts/v5-patch.py
grep -q 'installTLSOrRealityFromStream' overlay/internal/singbox/integrated.go
grep -q 'installRealityFromNative3xui' overlay/internal/singbox/reality.go
grep -q 'Reality is not supported by TUIC/QUIC' internal/singbox/config.go
grep -q 'Reality is not supported by sing-box Naive outbound' internal/singbox/config.go
grep -q 'security.*reality' overlay/internal/sub/singbox_links.go
grep -q 'security.*reality' overlay/internal/sub/singbox_clash.go
grep -q 'normalizeShadowsocksServerKey' scripts/v7-patch.py
grep -q 'normalizeShadowsocks2022Keys' scripts/v7-patch.py
grep -q 'autoComplete="new-password"' scripts/v7-patch.py
grep -q 'shouldValidate: true' scripts/v7-patch.py
grep -q 'failed to persist healed Shadowsocks settings' scripts/v8-patch.py
grep -q 'validSS2022Key' scripts/v8-patch.py
grep -q 'isShadowsocks2022Password' overlay/frontend/src/pages/inbounds/form/protocols/singbox.tsx
grep -q 'Form.Item label="Inner Shadowsocks Password"' overlay/frontend/src/pages/inbounds/form/protocols/singbox.tsx
grep -q 'scripts/v8-patch.py' scripts/apply-overlay.sh
grep -q 'scripts/v10-patch.py' scripts/apply-overlay.sh
! grep -q "values.protocol === 'tuic'" scripts/v5-patch.py
! grep -q "values.protocol === 'naive'" scripts/v5-patch.py
! grep -q "values.protocol === 'shadowtls'" scripts/v5-patch.py

echo '[7/8] Xray isolation guards'
grep -q 'IsSupplementalProtocol' scripts/v6-patch.py
if grep -RniE --include='*.go' 'GenXrayInboundConfig\(' overlay/internal/singbox overlay/internal/mieru internal/singbox internal/mieru; then
  echo 'supplemental code must not render Xray inbounds' >&2
  exit 1
fi

echo '[8/8] overlay contract'
for f in \
  overlay/internal/singbox/integrated.go \
  overlay/internal/singbox/reality.go \
  overlay/internal/singbox/stats.go \
  overlay/internal/singbox/stats_test.go \
  overlay/internal/mieru/integrated.go \
  overlay/internal/mieru/stats.go \
  overlay/internal/mieru/stats_test.go \
  overlay/internal/web/job/supplemental_traffic_job.go \
  overlay/internal/web/service/shadowsocks_2022_key_test.go \
  overlay/internal/web/service/supplemental_online.go \
  overlay/internal/web/service/supplemental_online_test.go \
  overlay/internal/database/model/singbox_protocols.go \
  overlay/internal/sub/singbox_links.go \
  overlay/internal/sub/singbox_links_test.go \
  overlay/internal/sub/singbox_clash.go \
  overlay/internal/sub/mieru_links.go \
  overlay/internal/sub/mieru_clash.go \
  overlay/frontend/src/schemas/protocols/inbound/singbox.ts \
  overlay/frontend/src/pages/inbounds/form/protocols/singbox.tsx \
  overlay/frontend/src/lib/xray/supplemental-links.ts \
  internal/mieru/config.go \
  internal/mieru/runtime.go \
  scripts/install-mieru.sh \
  scripts/uninstall-mieru.sh \
  scripts/install-singbox.sh \
  scripts/v10-patch.py \
  scripts/v11-patch.py \
  scripts/v12-patch.py \
  SINGBOX_VERSION; do
  test -s "$f"
done
! grep -q "path: 'singbox'" scripts/apply-overlay.sh
! grep -q "key: '/singbox'" scripts/apply-overlay.sh
! grep -q 'api.Group("/singbox")' scripts/apply-overlay.sh

echo 'smoke: PASS'
