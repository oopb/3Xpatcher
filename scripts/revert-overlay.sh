#!/usr/bin/env bash
set -euo pipefail

SRC=${1:-}
BACKUP=${2:-}
[[ -n "$SRC" && -n "$BACKUP" ]] || { echo "Usage: $0 /path/to/3x-ui-source /path/to/.dualcore-backup-*" >&2; exit 2; }

# Keep this list byte-for-byte aligned with apply-overlay.sh's modified list.
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
  frontend/src/pages/inbounds/InboundsPage.tsx
  frontend/src/pages/inbounds/form/protocols/index.ts
  frontend/src/pages/inbounds/form/InboundFormModal.tsx
  frontend/src/pages/inbounds/form/security/tls.tsx
  frontend/src/pages/inbounds/info/helpers.ts
  frontend/src/pages/inbounds/info/InboundInfoModal.tsx
  frontend/src/pages/inbounds/list/helpers.ts
  frontend/src/pages/inbounds/list/RowActions.tsx
  frontend/src/pages/inbounds/list/types.ts
  frontend/src/pages/inbounds/list/useInboundColumns.tsx
  frontend/src/pages/inbounds/qr/QrCodeModal.tsx
  frontend/src/pages/inbounds/useInbounds.ts
  frontend/src/pages/clients/ClientFormModal.tsx
  frontend/src/pages/clients/ClientBulkAddModal.tsx
)
for f in "${modified[@]}"; do
  [[ -f "$BACKUP/$f" ]] || { echo "Invalid backup; missing $f" >&2; exit 1; }
  cp "$BACKUP/$f" "$SRC/$f"
done

rm -rf "$SRC/internal/singbox" "$SRC/internal/mieru"
rm -f \
  "$SRC/internal/database/model/singbox_protocols.go" \
  "$SRC/internal/sub/singbox_links.go" \
  "$SRC/internal/sub/singbox_links_test.go" \
  "$SRC/internal/sub/singbox_clash.go" \
  "$SRC/internal/sub/mieru_links.go" \
  "$SRC/internal/sub/mieru_clash.go" \
  "$SRC/internal/web/controller/singbox_cert.go" \
  "$SRC/internal/web/job/supplemental_traffic_job.go" \
  "$SRC/internal/web/service/supplemental_online.go" \
  "$SRC/frontend/src/schemas/protocols/inbound/singbox.ts" \
  "$SRC/frontend/src/pages/inbounds/form/protocols/singbox.tsx" \
  "$SRC/frontend/src/lib/xray/supplemental-links.ts"

echo "3Xpatcher V11 source overlay reverted from: $BACKUP"
echo "Database rows/tables and generated runtime/certificate files are intentionally not dropped."
