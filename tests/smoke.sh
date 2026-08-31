#!/usr/bin/env bash
set -euo pipefail
ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT"

echo '[1/6] renderer unit tests'
go test ./internal/singbox

echo '[2/6] script syntax'
bash -n install.sh rollback.sh scripts/*.sh tests/*.sh
python3 -m py_compile scripts/apply-v2.py scripts/v2_patchlib.py scripts/v2-patch-*.py

echo '[3/6] integrated protocol surface'
python3 - <<'PY'
from pathlib import Path
expected={'tuic','anytls','shadowtls','naive'}
settings=Path('overlay/frontend/src/schemas/protocols/inbound/singbox.ts').read_text()
fields=Path('overlay/frontend/src/pages/inbounds/form/protocols/singbox.tsx').read_text()
model=Path('overlay/internal/database/model/singbox_protocols.go').read_text()
renderer=Path('internal/singbox/config.go').read_text()
for p in expected:
    if p not in model or p not in renderer:
        raise SystemExit(f'missing backend protocol {p}')
for component in ('TuicFields','AnyTlsFields','ShadowTlsFields','NaiveFields'):
    if component not in fields:
        raise SystemExit(f'missing frontend component {component}')
if 'clients:' not in settings:
    raise SystemExit('supplemental frontend settings must retain native clients mirror')
print('protocol surface: OK')
PY

echo '[4/6] native identity/subscription integration guards'
grep -q 'JOIN client_inbounds AS ci' overlay/internal/singbox/integrated.go
grep -q 'Select("c.\*")' overlay/internal/singbox/integrated.go
grep -q 'genSingboxLink' overlay/internal/sub/singbox_links.go
grep -q "'tuic','anytls','shadowtls','naive')" scripts/v2-patch-subscription.py
grep -q 'case model.TUIC:' scripts/v2-patch-backend.py
grep -q "'frontend/src/pages/clients/ClientFormModal.tsx'" scripts/v2-patch-subscription.py
grep -q "'internal/web/service/client_inbound_apply.go'" scripts/v2-patch-backend.py
grep -q 'TUIC client requires UUID and password' scripts/v2-patch-backend.py
grep -q "AMNEZIAWG: 'amneziawg'" scripts/v2-patch-frontend.py
grep -q "internal/sub/json_service.go" scripts/v2-patch-subscription.py
grep -q 'model.IsSingboxProtocol(inbound.Protocol)' scripts/v2-patch-backend.py scripts/v2-patch-subscription.py

echo '[5/6] Xray isolation guards'
grep -q 'model.IsSingboxProtocol(inbound.Protocol)' scripts/v2-patch-backend.py scripts/v2-patch-subscription.py
grep -q 'model.IsSingboxProtocol(ib.Protocol)' scripts/v2-patch-backend.py
if grep -RniE --include='*.go' 'GenXrayInboundConfig\(' overlay/internal/singbox internal/singbox; then
  echo 'supplemental code must not render Xray inbounds' >&2
  exit 1
fi

echo '[6/6] overlay contract'
for f in \
  overlay/internal/singbox/integrated.go \
  overlay/internal/database/model/singbox_protocols.go \
  overlay/internal/sub/singbox_links.go \
  overlay/internal/sub/singbox_clash.go \
  overlay/frontend/src/schemas/protocols/inbound/singbox.ts \
  overlay/frontend/src/pages/inbounds/form/protocols/singbox.tsx; do
  test -s "$f"
done
# V2 must not inject the old standalone page/menu/API.
! grep -q "path: 'singbox'" scripts/apply-overlay.sh
! grep -q "key: '/singbox'" scripts/apply-overlay.sh
! grep -q 'api.Group("/singbox")' scripts/apply-overlay.sh

echo 'smoke: PASS'
