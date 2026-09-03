#!/usr/bin/env bash
set -Eeuo pipefail

ROOT="${MIERU_ROOT:-/usr/local/x-ui-mieru}"
SERVICE_FILE="${MIERU_SERVICE_FILE:-/etc/systemd/system/x-ui-mieru@.service}"
PURGE="${PURGE:-0}"
[[ ${EUID:-$(id -u)} -eq 0 ]] || { echo "uninstall-mieru.sh must run as root" >&2; exit 1; }

# Disable every currently configured instance. Config filenames are canonical
# because 3Xpatcher owns this runtime directory.
if [[ -d "$ROOT/config" ]]; then
  shopt -s nullglob
  for path in "$ROOT"/config/*.json; do
    id=$(basename "$path" .json)
    [[ "$id" =~ ^[0-9]+$ ]] || continue
    systemctl disable --now "x-ui-mieru@${id}.service" >/dev/null 2>&1 || true
  done
  shopt -u nullglob
fi
# Also stop any transient active instances even if their config was removed.
while read -r unit; do
  [[ -n "$unit" ]] && systemctl disable --now "$unit" >/dev/null 2>&1 || true
done < <(systemctl list-units --all --plain --no-legend 'x-ui-mieru@*.service' 2>/dev/null | awk '{print $1}')

rm -f "$SERVICE_FILE"
systemctl daemon-reload
systemctl reset-failed 'x-ui-mieru@*.service' >/dev/null 2>&1 || true
rm -rf /run/x-ui-mieru

if [[ "$PURGE" == "1" ]]; then
  rm -rf "$ROOT"
  echo "Mieru runtime and configuration purged."
else
  rm -rf "$ROOT/bin"
  echo "Mieru runtime removed; configuration preserved in $ROOT/config."
fi
