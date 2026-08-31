#!/usr/bin/env bash
set -euo pipefail

SRC=${1:-}
BACKUP=${2:-}
if [[ -z "$SRC" || -z "$BACKUP" ]]; then
  echo "Usage: $0 /path/to/3x-ui-source /path/to/.dualcore-backup-YYYYMMDD-HHMMSS" >&2
  exit 2
fi
for file in \
  internal/database/db.go \
  internal/web/controller/api.go \
  frontend/src/routes.tsx \
  frontend/src/layouts/AppSidebar.tsx; do
  if [[ ! -f "$BACKUP/$file" ]]; then
    echo "Invalid backup; missing $file" >&2
    exit 1
  fi
done

cp "$BACKUP/internal/database/db.go" "$SRC/internal/database/db.go"
cp "$BACKUP/internal/web/controller/api.go" "$SRC/internal/web/controller/api.go"
cp "$BACKUP/frontend/src/routes.tsx" "$SRC/frontend/src/routes.tsx"
cp "$BACKUP/frontend/src/layouts/AppSidebar.tsx" "$SRC/frontend/src/layouts/AppSidebar.tsx"
rm -rf "$SRC/internal/singbox"
rm -f "$SRC/internal/database/model/singbox.go"
rm -f "$SRC/internal/web/service/singbox.go"
rm -f "$SRC/internal/web/controller/singbox.go"
rm -rf "$SRC/frontend/src/pages/singbox"

gofmt -w "$SRC/internal/database/db.go" "$SRC/internal/web/controller/api.go"

echo "Dual-core source overlay reverted from: $BACKUP"
echo "The singbox_inbounds database table is intentionally NOT dropped."
