#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT"

echo '[1/5] Go renderer tests'
go test ./internal/singbox

echo '[2/5] Shell syntax'
bash -n scripts/*.sh tests/*.sh

echo '[3/5] Example JSON + V1 protocol boundary'
python3 - <<'PY'
from pathlib import Path
import json
root = Path('.')
expected = {'tuic', 'anytls', 'shadowtls', 'naive'}
seen = set()
for path in sorted((root / 'examples').glob('*.json')):
    obj = json.loads(path.read_text())
    proto = obj.get('protocol')
    if proto not in expected:
        raise SystemExit(f'{path}: unexpected V1 protocol {proto!r}')
    seen.add(proto)
if seen != expected:
    raise SystemExit(f'example protocol set mismatch: {seen!r}')
page = (root / 'frontend/src/pages/singbox/SingboxInboundsPage.tsx').read_text()
for p in ('tuic', 'anytls', 'shadowtls', 'naive'):
    if f"value: '{p}'" not in page:
        raise SystemExit(f'UI is missing protocol: {p}')
if "value: 'snell'" in page:
    raise SystemExit('Snell must stay hidden in V1')
print('examples/UI protocol boundary: OK')
PY

echo '[4/5] Xray isolation static guard'
# Supplemental runtime code may mention Xray in comments/documentation, but it
# must not invoke known Xray lifecycle/update operations or binary paths.
if grep -RniE --include='*.go' --include='*.sh' \
  '(UpdateXray|RestartXray|StopXray|systemctl[^\n]*(xray|x-ui\.service)|/usr/local/x-ui/bin/xray|xray-linux-(amd64|arm64))' \
  internal scripts systemd; then
  echo 'Found a forbidden Xray lifecycle/binary operation in supplemental code.' >&2
  exit 1
fi

echo '[5/5] Overlay + frontend route + rollback smoke'
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
src="$tmp/3x-ui"
mkdir -p \
  "$src/internal/database/model" \
  "$src/internal/database" \
  "$src/internal/web/controller" \
  "$src/internal/web/service" \
  "$src/frontend/src/layouts" \
  "$src/frontend/src/pages/inbounds"
cat > "$src/go.mod" <<'GO'
module github.com/mhsanaei/3x-ui/v3

go 1.23
GO
cat > "$src/internal/database/db.go" <<'GO'
package database

import "github.com/mhsanaei/3x-ui/v3/internal/database/model"

func allModels() []any {
	return []any{
		&model.Inbound{},
	}
}
GO
cat > "$src/internal/database/model/model.go" <<'GO'
package model

type Inbound struct{}
GO
cat > "$src/internal/web/controller/api.go" <<'GO'
package controller

type RouterGroup struct{}
func (r *RouterGroup) Group(string) *RouterGroup { return r }
type InboundController struct{}
func NewInboundController(*RouterGroup) *InboundController { return &InboundController{} }
type API struct{ inboundController *InboundController }
func (a *API) init(api *RouterGroup) {
	inbounds := api.Group("/inbounds")
	a.inboundController = NewInboundController(inbounds)
}
GO
cat > "$src/frontend/src/routes.tsx" <<'TS'
const InboundsPage = lazy(() => import('@/pages/inbounds/InboundsPage'));
const routes = [
      { path: 'inbounds', element: withSuspense(<InboundsPage />) },
];
TS
cat > "$src/frontend/src/layouts/AppSidebar.tsx" <<'TS'
const tabs = [
      { key: '/inbounds', icon: 'inbound', title: t('menu.inbounds') },
];
TS

before_db=$(sha256sum "$src/internal/database/db.go" | awk '{print $1}')
before_api=$(sha256sum "$src/internal/web/controller/api.go" | awk '{print $1}')
before_routes=$(sha256sum "$src/frontend/src/routes.tsx" | awk '{print $1}')
before_sidebar=$(sha256sum "$src/frontend/src/layouts/AppSidebar.tsx" | awk '{print $1}')

out=$(scripts/apply-overlay.sh "$src")
backup=$(printf '%s\n' "$out" | sed -n 's/^Backup: //p')
[[ -n "$backup" && -d "$backup" ]]
grep -q '&model.SingboxInbound{}' "$src/internal/database/db.go"
grep -q 'api.Group("/singbox")' "$src/internal/web/controller/api.go"
grep -q "path: 'singbox'" "$src/frontend/src/routes.tsx"
grep -q "key: '/singbox'" "$src/frontend/src/layouts/AppSidebar.tsx"
test -f "$src/frontend/src/pages/singbox/SingboxInboundsPage.tsx"

scripts/revert-overlay.sh "$src" "$backup" >/dev/null
[[ $(sha256sum "$src/internal/database/db.go" | awk '{print $1}') == "$before_db" ]]
[[ $(sha256sum "$src/internal/web/controller/api.go" | awk '{print $1}') == "$before_api" ]]
[[ $(sha256sum "$src/frontend/src/routes.tsx" | awk '{print $1}') == "$before_routes" ]]
[[ $(sha256sum "$src/frontend/src/layouts/AppSidebar.tsx" | awk '{print $1}') == "$before_sidebar" ]]
test ! -e "$src/internal/singbox"
test ! -e "$src/frontend/src/pages/singbox"

echo 'smoke: PASS'
