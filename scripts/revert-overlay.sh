#!/usr/bin/env bash
set -euo pipefail

SRC=${1:-}
BACKUP=${2:-}
[[ -n "$SRC" && -n "$BACKUP" ]] || { echo "Usage: $0 /path/to/3x-ui-source /path/to/.dualcore-backup-*" >&2; exit 2; }
modified=(
  internal/database/model/model.go
  internal/web/service/inbound.go
  internal/web/service/client_crud.go
  internal/web/service/client_inbound_apply.go
  internal/web/service/inbound_clients.go
  internal/web/service/xray.go
  internal/web/runtime/local.go
  internal/sub/service.go
  internal/sub/json_service.go
  internal/sub/clash_service.go
  frontend/src/schemas/primitives/protocol.ts
  frontend/src/schemas/protocols/inbound/index.ts
  frontend/src/lib/xray/inbound-defaults.ts
  frontend/src/lib/xray/protocol-capabilities.ts
  frontend/src/pages/inbounds/form/protocols/index.ts
  frontend/src/pages/inbounds/form/InboundFormModal.tsx
  frontend/src/pages/clients/ClientFormModal.tsx
  frontend/src/pages/clients/ClientBulkAddModal.tsx
)
for f in "${modified[@]}"; do
  [[ -f "$BACKUP/$f" ]] || { echo "Invalid backup; missing $f" >&2; exit 1; }
  cp "$BACKUP/$f" "$SRC/$f"
done
rm -rf "$SRC/internal/singbox"
rm -f \
  "$SRC/internal/database/model/singbox_protocols.go" \
  "$SRC/internal/sub/singbox_links.go" \
  "$SRC/internal/sub/singbox_clash.go" \
  "$SRC/frontend/src/schemas/protocols/inbound/singbox.ts" \
  "$SRC/frontend/src/pages/inbounds/form/protocols/singbox.tsx"

echo "3Xpatcher V2 source overlay reverted from: $BACKUP"
echo "Database rows/tables are intentionally not dropped."
