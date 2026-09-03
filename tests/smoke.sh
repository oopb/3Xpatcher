#!/usr/bin/env bash
set -euo pipefail
ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT"

echo '[1/8] renderer unit tests'
go test ./internal/singbox ./internal/mieru

echo '[2/8] script syntax'
bash -n install.sh rollback.sh scripts/*.sh tests/*.sh
python3 -m py_compile scripts/apply-v2.py scripts/v2_patchlib.py scripts/v2-patch-*.py scripts/v3-patch.py scripts/v5-patch.py scripts/v6-patch.py

echo '[3/8] integrated protocol surface'
python3 - <<'PY'
from pathlib import Path
singbox={'tuic','anytls','shadowtls','naive'}
settings=Path('overlay/frontend/src/schemas/protocols/inbound/singbox.ts').read_text()
fields=Path('overlay/frontend/src/pages/inbounds/form/protocols/singbox.tsx').read_text()
model=Path('overlay/internal/database/model/singbox_protocols.go').read_text()
renderer=Path('internal/singbox/config.go').read_text()
for p in singbox:
    if p not in model or p not in renderer:
        raise SystemExit(f'missing sing-box backend protocol {p}')
if 'mieru' not in model or 'MieruInboundSettingsSchema' not in settings or 'MieruFields' not in fields:
    raise SystemExit('missing Mieru integrated surface')
for component in ('TuicFields','AnyTlsFields','ShadowTlsFields','NaiveFields','MieruFields'):
    if component not in fields:
        raise SystemExit(f'missing frontend component {component}')
if 'clients:' not in settings:
    raise SystemExit('supplemental frontend settings must retain native clients mirror')
print('protocol surface: OK')
PY

echo '[4/8] native identity/subscription integration guards'
grep -q 'JOIN client_inbounds AS ci' overlay/internal/singbox/integrated.go
grep -q 'JOIN client_inbounds AS ci' overlay/internal/mieru/integrated.go
grep -q 'genSingboxLink' overlay/internal/sub/singbox_links.go
grep -q 'genMieruLink' overlay/internal/sub/mieru_links.go
grep -q 'buildMieruProxy' overlay/internal/sub/mieru_clash.go
grep -q 'model.Mieru' scripts/v6-patch.py
grep -q 'IsSupplementalProtocol' scripts/v6-patch.py
grep -q 'MieruFields' scripts/v6-patch.py
grep -q 'mierus' overlay/internal/sub/mieru_links.go
grep -q '"type": "mieru"' overlay/internal/sub/mieru_clash.go
grep -q 'hashedPassword' internal/mieru/config.go
grep -q 'MITA_CONFIG_JSON_FILE' scripts/install-mieru.sh

echo '[5/8] Mieru runtime isolation guards'
grep -q 'x-ui-mieru@%d.service' internal/mieru/runtime.go
grep -q 'x-ui-mieru@.service' scripts/install-mieru.sh
grep -q 'MITA_UDS_PATH=/run/x-ui-mieru/%i.sock' scripts/install-mieru.sh
grep -q 'DefaultConfigDir.*x-ui-mieru/config' internal/mieru/runtime.go
grep -q 'ConfiguredIDs' overlay/internal/mieru/integrated.go
grep -q 'mcore.Reconcile' scripts/v6-patch.py
grep -q 'IsMieruProtocol' scripts/v6-patch.py
grep -q 'MIERU_VERSION' install.sh
grep -q 'PURGE_MIERU' rollback.sh

echo '[6/8] Reality integration guards'
grep -q "values.protocol === 'anytls'" scripts/v5-patch.py
grep -q 'installTLSOrRealityFromStream' overlay/internal/singbox/integrated.go
grep -q 'installRealityFromNative3xui' overlay/internal/singbox/reality.go
grep -q 'Reality is not supported by TUIC/QUIC' internal/singbox/config.go
grep -q 'Reality is not supported by sing-box Naive outbound' internal/singbox/config.go
grep -q 'security.*reality' overlay/internal/sub/singbox_links.go
grep -q 'AnyTLS.*Reality' overlay/internal/sub/singbox_clash.go
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
  overlay/internal/mieru/integrated.go \
  overlay/internal/database/model/singbox_protocols.go \
  overlay/internal/sub/singbox_links.go \
  overlay/internal/sub/singbox_clash.go \
  overlay/internal/sub/mieru_links.go \
  overlay/internal/sub/mieru_clash.go \
  overlay/frontend/src/schemas/protocols/inbound/singbox.ts \
  overlay/frontend/src/pages/inbounds/form/protocols/singbox.tsx \
  internal/mieru/config.go \
  internal/mieru/runtime.go \
  scripts/install-mieru.sh \
  scripts/uninstall-mieru.sh; do
  test -s "$f"
done
! grep -q "path: 'singbox'" scripts/apply-overlay.sh
! grep -q "key: '/singbox'" scripts/apply-overlay.sh
! grep -q 'api.Group("/singbox")' scripts/apply-overlay.sh

echo 'smoke: PASS'
