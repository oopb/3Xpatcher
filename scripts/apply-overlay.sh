#!/usr/bin/env bash
set -euo pipefail

SRC=${1:-}
ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
if [[ -z "$SRC" \
  || ! -f "$SRC/go.mod" \
  || ! -f "$SRC/internal/database/db.go" \
  || ! -f "$SRC/internal/web/controller/api.go" \
  || ! -f "$SRC/frontend/src/routes.tsx" \
  || ! -f "$SRC/frontend/src/layouts/AppSidebar.tsx" ]]; then
  echo "Usage: $0 /path/to/3x-ui-source" >&2
  echo "Target must contain the current v3 backend and React frontend source tree." >&2
  exit 2
fi

if ! grep -q '^module github.com/mhsanaei/3x-ui/v3$' "$SRC/go.mod"; then
  echo "Refusing to patch: target is not current 3x-ui v3 source." >&2
  exit 1
fi

# Validate upstream patch points before copying anything. Fail closed if the
# upstream structure moved, rather than making a half-applied patch.
python3 - "$SRC" <<'PYCODE'
from pathlib import Path
import re, sys
root = Path(sys.argv[1])
db = (root / "internal/database/db.go").read_text()
api = (root / "internal/web/controller/api.go").read_text()
routes = (root / "frontend/src/routes.tsx").read_text()
sidebar = (root / "frontend/src/layouts/AppSidebar.tsx").read_text()

if "&model.SingboxInbound{}" not in db and not re.search(r'(?m)^\s*&model\.Inbound\{\},\s*$', db):
    raise SystemExit("db.go patch point not found")
if 'api.Group("/singbox")' not in api and not re.search(r'(?m)^\s*a\.inboundController\s*=\s*NewInboundController\(inbounds\)\s*$', api):
    raise SystemExit("api.go patch point not found")
if "SingboxInboundsPage" not in routes and "const InboundsPage = lazy(() => import('@/pages/inbounds/InboundsPage'));" not in routes:
    raise SystemExit("frontend routes lazy-import patch point not found")
if "path: 'singbox'" not in routes and "{ path: 'inbounds', element: withSuspense(<InboundsPage />) }," not in routes:
    raise SystemExit("frontend routes child patch point not found")
if "key: '/singbox'" not in sidebar and "{ key: '/inbounds', icon: 'inbound', title: t('menu.inbounds') }," not in sidebar:
    raise SystemExit("AppSidebar.tsx patch point not found")
PYCODE

backup="$SRC/.dualcore-backup-$(date +%Y%m%d-%H%M%S)"
mkdir -p \
  "$backup/internal/database" \
  "$backup/internal/web/controller" \
  "$backup/frontend/src/layouts" \
  "$backup/frontend/src"
cp "$SRC/internal/database/db.go" "$backup/internal/database/db.go"
cp "$SRC/internal/web/controller/api.go" "$backup/internal/web/controller/api.go"
cp "$SRC/frontend/src/routes.tsx" "$backup/frontend/src/routes.tsx"
cp "$SRC/frontend/src/layouts/AppSidebar.tsx" "$backup/frontend/src/layouts/AppSidebar.tsx"

mkdir -p \
  "$SRC/internal/singbox" \
  "$SRC/internal/database/model" \
  "$SRC/internal/web/service" \
  "$SRC/internal/web/controller" \
  "$SRC/frontend/src/pages/singbox"
cp "$ROOT/internal/singbox/"*.go "$SRC/internal/singbox/"
# Do not copy standalone renderer tests into the production overlay.
rm -f "$SRC/internal/singbox/config_test.go"
cp "$ROOT/internal/database/model/singbox.go" "$SRC/internal/database/model/singbox.go"
cp "$ROOT/internal/web/service/singbox.go" "$SRC/internal/web/service/singbox.go"
cp "$ROOT/internal/web/controller/singbox.go" "$SRC/internal/web/controller/singbox.go"
cp "$ROOT/frontend/src/pages/singbox/SingboxInboundsPage.tsx" "$SRC/frontend/src/pages/singbox/SingboxInboundsPage.tsx"

python3 - "$SRC" <<'PYCODE'
from pathlib import Path
import re, sys
root = Path(sys.argv[1])

db_path = root / "internal/database/db.go"
api_path = root / "internal/web/controller/api.go"
routes_path = root / "frontend/src/routes.tsx"
sidebar_path = root / "frontend/src/layouts/AppSidebar.tsx"
db = db_path.read_text()
api = api_path.read_text()
routes = routes_path.read_text()
sidebar = sidebar_path.read_text()

if "&model.SingboxInbound{}" not in db:
    m = re.search(r'(?m)^(\s*)&model\.Inbound\{\},\s*$', db)
    if not m:
        raise SystemExit("db.go patch point disappeared")
    indent = m.group(1)
    db = db[:m.end()] + f"\n{indent}&model.SingboxInbound{{}}," + db[m.end():]

if 'api.Group("/singbox")' not in api:
    m = re.search(r'(?m)^(\s*)a\.inboundController\s*=\s*NewInboundController\(inbounds\)\s*$', api)
    if not m:
        raise SystemExit("api.go patch point disappeared")
    indent = m.group(1)
    addition = (
        f"\n\n{indent}// Supplemental sing-box core. Kept separate from Xray inbounds."
        f"\n{indent}singboxGroup := api.Group(\"/singbox\")"
        f"\n{indent}NewSingboxController(singboxGroup)"
    )
    api = api[:m.end()] + addition + api[m.end():]

lazy_line = "const InboundsPage = lazy(() => import('@/pages/inbounds/InboundsPage'));"
if "const SingboxInboundsPage" not in routes:
    if lazy_line not in routes:
        raise SystemExit("routes import patch point disappeared")
    routes = routes.replace(
        lazy_line,
        lazy_line + "\nconst SingboxInboundsPage = lazy(() => import('@/pages/singbox/SingboxInboundsPage'));",
        1,
    )

route_line = "      { path: 'inbounds', element: withSuspense(<InboundsPage />) },"
if "path: 'singbox'" not in routes:
    if route_line not in routes:
        raise SystemExit("routes child patch point disappeared")
    routes = routes.replace(
        route_line,
        route_line + "\n      { path: 'singbox', element: withSuspense(<SingboxInboundsPage />) },",
        1,
    )

sidebar_line = "      { key: '/inbounds', icon: 'inbound', title: t('menu.inbounds') },"
if "key: '/singbox'" not in sidebar:
    if sidebar_line not in sidebar:
        raise SystemExit("sidebar patch point disappeared")
    # Reuse an existing icon key so this patch does not alter the sidebar icon
    # type registry or import list.
    sidebar = sidebar.replace(
        sidebar_line,
        sidebar_line + "\n      { key: '/singbox', icon: 'cluster', title: 'Sing-box' },",
        1,
    )

db_path.write_text(db)
api_path.write_text(api)
routes_path.write_text(routes)
sidebar_path.write_text(sidebar)
PYCODE

gofmt -w \
  "$SRC/internal/singbox"/*.go \
  "$SRC/internal/database/model/singbox.go" \
  "$SRC/internal/web/service/singbox.go" \
  "$SRC/internal/web/controller/singbox.go" \
  "$SRC/internal/database/db.go" \
  "$SRC/internal/web/controller/api.go"

echo "Dual-core overlay applied."
echo "Backup: $backup"
echo "Added UI route: /panel/singbox"
echo "Next: build/test the 3x-ui source with its normal upstream workflow."
