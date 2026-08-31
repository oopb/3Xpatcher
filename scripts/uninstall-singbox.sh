#!/usr/bin/env bash
set -euo pipefail
[[ ${EUID:-$(id -u)} -eq 0 ]] || { echo "Run as root" >&2; exit 1; }
systemctl disable --now x-ui-singbox.service 2>/dev/null || true
rm -f /etc/systemd/system/x-ui-singbox.service
systemctl daemon-reload
if [[ ${PURGE:-0} == 1 ]]; then
  rm -rf /usr/local/x-ui-singbox
  echo "Removed service and all sing-box patch data. Xray was not touched."
else
  rm -rf /usr/local/x-ui-singbox/bin
  echo "Removed sing-box binary/service; config/backups preserved under /usr/local/x-ui-singbox."
fi
