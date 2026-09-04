#!/usr/bin/env bash
set -euo pipefail

SRC=${1:-}
ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
[[ -n "$SRC" && -f "$SRC/go.mod" ]] || { echo "Usage: $0 /path/to/3x-ui-source" >&2; exit 2; }
if ! grep -q '^module github.com/mhsanaei/3x-ui/v3$' "$SRC/go.mod"; then
  echo "Refusing to patch: target is not current 3x-ui v3 source." >&2
  exit 1
fi

modified=(
  internal/database/model/model.go
  internal/web/service/inbound.go
  internal/web/service/inbound_node.go
  internal/web/service/server.go
  internal/web/service/client_crud.go
  internal/web/service/client_inbound_apply.go
  internal/web/service/inbound_clients.go
  internal/web/service/xray.go
  internal/web/web.go
  internal/web/controller/server.go
  internal/web/runtime/local.go
  internal/sub/service.go
  internal/sub/json_service.go
  internal/sub/clash_service.go
  frontend/src/schemas/primitives/protocol.ts
  frontend/src/schemas/protocols/inbound/index.ts
  frontend/src/schemas/protocols/security/tls.ts
  frontend/src/lib/xray/inbound-defaults.ts
  frontend/src/lib/xray/protocol-capabilities.ts
  frontend/src/lib/xray/inbound-link.ts
  frontend/src/pages/inbounds/form/protocols/index.ts
  frontend/src/pages/inbounds/form/InboundFormModal.tsx
  frontend/src/pages/inbounds/form/security/tls.tsx
  frontend/src/pages/inbounds/list/helpers.ts
  frontend/src/pages/inbounds/list/RowActions.tsx
  frontend/src/pages/inbounds/list/types.ts
  frontend/src/pages/inbounds/list/useInboundColumns.tsx
  frontend/src/pages/inbounds/useInbounds.ts
  frontend/src/pages/clients/ClientFormModal.tsx
  frontend/src/pages/clients/ClientBulkAddModal.tsx
)
for f in "${modified[@]}"; do
  [[ -f "$SRC/$f" ]] || { echo "Refusing to patch: missing upstream file $f" >&2; exit 1; }
done

backup="$SRC/.dualcore-backup-$(date +%Y%m%d-%H%M%S)"
for f in "${modified[@]}"; do
  mkdir -p "$backup/$(dirname "$f")"
  cp "$SRC/$f" "$backup/$f"
done

mkdir -p "$SRC/internal/singbox" "$SRC/internal/mieru" "$SRC/internal/database/model" "$SRC/internal/sub" "$SRC/internal/web/controller" "$SRC/internal/web/job" "$SRC/internal/web/service" "$SRC/frontend/src/schemas/protocols/inbound" "$SRC/frontend/src/pages/inbounds/form/protocols" "$SRC/frontend/src/lib/xray"
cp "$ROOT/internal/singbox/config.go" "$SRC/internal/singbox/config.go"
cp "$ROOT/internal/singbox/runtime.go" "$SRC/internal/singbox/runtime.go"
cp "$ROOT/overlay/internal/singbox/integrated.go" "$SRC/internal/singbox/integrated.go"
cp "$ROOT/overlay/internal/singbox/reality.go" "$SRC/internal/singbox/reality.go"
cp "$ROOT/overlay/internal/singbox/selfsigned.go" "$SRC/internal/singbox/selfsigned.go"
cp "$ROOT/overlay/internal/singbox/selfsigned_util.go" "$SRC/internal/singbox/selfsigned_util.go"
cp "$ROOT/overlay/internal/singbox/stats.go" "$SRC/internal/singbox/stats.go"
cp "$ROOT/internal/mieru/config.go" "$SRC/internal/mieru/config.go"
cp "$ROOT/internal/mieru/runtime.go" "$SRC/internal/mieru/runtime.go"
cp "$ROOT/overlay/internal/mieru/integrated.go" "$SRC/internal/mieru/integrated.go"
cp "$ROOT/overlay/internal/mieru/stats.go" "$SRC/internal/mieru/stats.go"
cp "$ROOT/overlay/internal/database/model/singbox_protocols.go" "$SRC/internal/database/model/singbox_protocols.go"
cp "$ROOT/overlay/internal/sub/singbox_links.go" "$SRC/internal/sub/singbox_links.go"
cp "$ROOT/overlay/internal/sub/singbox_links_test.go" "$SRC/internal/sub/singbox_links_test.go"
cp "$ROOT/overlay/internal/sub/singbox_clash.go" "$SRC/internal/sub/singbox_clash.go"
cp "$ROOT/overlay/internal/sub/mieru_links.go" "$SRC/internal/sub/mieru_links.go"
cp "$ROOT/overlay/internal/sub/mieru_clash.go" "$SRC/internal/sub/mieru_clash.go"
cp "$ROOT/overlay/internal/web/controller/singbox_cert.go" "$SRC/internal/web/controller/singbox_cert.go"
cp "$ROOT/overlay/internal/web/job/supplemental_traffic_job.go" "$SRC/internal/web/job/supplemental_traffic_job.go"
cp "$ROOT/overlay/internal/web/service/supplemental_online.go" "$SRC/internal/web/service/supplemental_online.go"
cp "$ROOT/overlay/frontend/src/schemas/protocols/inbound/singbox.ts" "$SRC/frontend/src/schemas/protocols/inbound/singbox.ts"
cp "$ROOT/overlay/frontend/src/pages/inbounds/form/protocols/singbox.tsx" "$SRC/frontend/src/pages/inbounds/form/protocols/singbox.tsx"
cp "$ROOT/overlay/frontend/src/lib/xray/supplemental-links.ts" "$SRC/frontend/src/lib/xray/supplemental-links.ts"

python3 "$ROOT/scripts/apply-v2.py" "$SRC"
python3 "$ROOT/scripts/v3-patch.py" "$SRC"
python3 "$ROOT/scripts/v5-patch.py" "$SRC"
python3 "$ROOT/scripts/v6-patch.py" "$SRC"
python3 "$ROOT/scripts/v7-patch.py" "$SRC"
python3 "$ROOT/scripts/v8-patch.py" "$SRC"
python3 "$ROOT/scripts/v9-patch.py" "$SRC"
python3 "$ROOT/scripts/v10-patch.py" "$SRC"
python3 "$ROOT/scripts/v11-patch.py" "$SRC"

gofmt -w "$SRC/internal/singbox"/*.go "$SRC/internal/mieru"/*.go "$SRC/internal/database/model/singbox_protocols.go" "$SRC/internal/database/model/model.go" "$SRC/internal/web/controller/server.go" "$SRC/internal/web/controller/singbox_cert.go" "$SRC/internal/web/job/supplemental_traffic_job.go" "$SRC/internal/web/service/supplemental_online.go" "$SRC/internal/web/service/inbound_node.go" "$SRC/internal/web/service/server.go" "$SRC/internal/web/service/inbound.go" "$SRC/internal/web/service/client_crud.go" "$SRC/internal/web/service/client_inbound_apply.go" "$SRC/internal/web/service/inbound_clients.go" "$SRC/internal/web/service/xray.go" "$SRC/internal/web/web.go" "$SRC/internal/web/runtime/local.go" "$SRC/internal/sub/service.go" "$SRC/internal/sub/json_service.go" "$SRC/internal/sub/clash_service.go" "$SRC/internal/sub/singbox_links.go" "$SRC/internal/sub/singbox_links_test.go" "$SRC/internal/sub/singbox_clash.go" "$SRC/internal/sub/mieru_links.go" "$SRC/internal/sub/mieru_clash.go"

echo "3Xpatcher V11 integrated overlay applied."
echo "Backup: $backup"
echo "UI: native /panel/inbounds + full native client action, QR and export parity for supplemental protocols"
echo "Security: native 3x-ui TLS and Reality UI reused by supported sing-box protocols"
echo "Stats: Xray / sing-box / Mieru fold into native 3x-ui traffic + merged online state"
echo "SS2022: server/client keys auto-generate, heal legacy invalid rows, and Clash export is guarded"
echo "Mieru: complete v3.36 server config coverage including multi-bind, DNS, egress and subscription expansion"
echo "Runtime: Xray / sing-box / official Mieru mita remain isolated"
