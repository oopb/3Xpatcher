#!/usr/bin/env bash
set -uo pipefail

section() {
  printf '\n\n========== %s ==========\n' "$1"
}

cmd() {
  printf '\n$ %s\n' "$*"
  "$@" 2>&1
  local rc=$?
  if (( rc != 0 )); then
    printf '[exit=%d]\n' "$rc"
  fi
  return 0
}

shell() {
  printf '\n$ %s\n' "$1"
  bash -lc "$1" 2>&1
  local rc=$?
  if (( rc != 0 )); then
    printf '[exit=%d]\n' "$rc"
  fi
  return 0
}

redact_json_file() {
  local file=$1
  python3 - "$file" <<'PY' 2>&1 || true
import json, sys
path = sys.argv[1]
try:
    with open(path, 'r', encoding='utf-8') as f:
        obj = json.load(f)
except Exception as e:
    print(f"[unable to parse {path}: {e}]")
    raise SystemExit(0)

sensitive = {
    'password', 'passwd', 'psk', 'secret', 'token', 'uuid',
    'privatekey', 'private_key', 'key', 'keyfile', 'key_file',
    'certificate', 'certificatefile', 'certificate_file',
}

def walk(v):
    if isinstance(v, dict):
        out = {}
        for k, x in v.items():
            if k.lower() in sensitive:
                if isinstance(x, list):
                    out[k] = [f"<redacted:{len(x)}-items>"]
                else:
                    out[k] = '<redacted>'
            else:
                out[k] = walk(x)
        return out
    if isinstance(v, list):
        return [walk(x) for x in v]
    return v

print(json.dumps(walk(obj), ensure_ascii=False, indent=2))
PY
}

redact_metrics() {
  sed -E \
    -e 's/user - [^"[:space:]]+/user - <redacted>/g' \
    -e 's/[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}/<redacted-email>/g'
}

if [[ ${EUID:-$(id -u)} -ne 0 ]]; then
  echo 'Please run this diagnostic as root (sudo bash ...).'
  exit 1
fi

export LANG=C
export LC_ALL=C

section 'SYSTEM'
cmd date -Is
cmd uname -a
shell 'cat /etc/os-release 2>/dev/null || true'
cmd id

section '3XPATCHER INSTALLATION'
shell 'readlink -f /usr/local/share/3xpatcher/current 2>/dev/null || true'
shell 'for f in VERSION UPSTREAM_COMPAT MIERU_VERSION SINGBOX_VERSION; do p="/usr/local/share/3xpatcher/current/$f"; if [[ -f "$p" ]]; then printf "%s=" "$f"; cat "$p"; fi; done'
shell 'ls -la /usr/local/share/3xpatcher/current 2>/dev/null | head -80 || true'

section 'X-UI SERVICE'
cmd systemctl is-active x-ui.service
cmd systemctl status x-ui.service --no-pager -l
cmd journalctl -u x-ui.service -n 220 --no-pager -o short-iso
shell 'command -v x-ui >/dev/null 2>&1 && x-ui version 2>&1 || /usr/local/x-ui/x-ui version 2>&1 || true'
shell 'sha256sum /usr/local/x-ui/x-ui 2>/dev/null || true'

section 'SING-BOX SERVICE'
cmd systemctl is-enabled x-ui-singbox.service
cmd systemctl is-active x-ui-singbox.service
cmd systemctl status x-ui-singbox.service --no-pager -l
cmd journalctl -u x-ui-singbox.service -n 220 --no-pager -o short-iso
cmd systemctl cat x-ui-singbox.service
shell '/usr/local/x-ui-singbox/bin/sing-box version 2>&1 || true'
shell 'sha256sum /usr/local/x-ui-singbox/bin/sing-box 2>/dev/null || true'
shell 'printf "stats-address: "; cat /etc/3xpatcher/singbox-stats.addr 2>/dev/null || echo "<default 127.0.0.1:62789>"'

section 'SING-BOX CONFIG (SECRETS REDACTED)'
if [[ -f /usr/local/x-ui-singbox/config/config.json ]]; then
  redact_json_file /usr/local/x-ui-singbox/config/config.json
else
  echo '[missing /usr/local/x-ui-singbox/config/config.json]'
fi

section 'SELF-SIGNED TLS FILES (NO CONTENTS)'
shell 'find /usr/local/x-ui-singbox/certs -maxdepth 4 -type f -printf "%M %u:%g %s %TY-%Tm-%TdT%TH:%TM:%TS %p\n" 2>/dev/null | sort || true'

section 'MIERU INSTALLATION'
shell '/usr/local/x-ui-mieru/bin/mita version 2>&1 || true'
shell 'sha256sum /usr/local/x-ui-mieru/bin/mita 2>/dev/null || true'
shell 'getent passwd mita 2>/dev/null || true; getent group mita 2>/dev/null || true'
shell 'stat -c "%A %U:%G %a %n" /usr/local/x-ui-mieru /usr/local/x-ui-mieru/bin /usr/local/x-ui-mieru/config /var/lib/x-ui-mieru /var/lib/mita /run/x-ui-mieru 2>/dev/null || true'
cmd systemctl cat x-ui-mieru@.service
shell "systemctl list-units --type=service --all --no-pager 'x-ui-mieru@*.service' || true"

section 'MIERU INSTANCES'
shopt -s nullglob
mieru_configs=(/usr/local/x-ui-mieru/config/*.json)
if (( ${#mieru_configs[@]} == 0 )); then
  echo '[no Mieru config files found]'
fi
for cfg in "${mieru_configs[@]}"; do
  id=$(basename "$cfg" .json)
  unit="x-ui-mieru@${id}.service"
  uds="/run/x-ui-mieru/${id}.sock"
  section "MIERU INSTANCE ${id}"
  cmd stat -c '%A %U:%G %a %s %n' "$cfg"
  echo '-- config (secrets redacted) --'
  redact_json_file "$cfg"
  cmd systemctl is-enabled "$unit"
  cmd systemctl is-active "$unit"
  cmd systemctl status "$unit" --no-pager -l
  cmd journalctl -u "$unit" -n 180 --no-pager -o short-iso
  if [[ -x /usr/local/x-ui-mieru/bin/mita && -r "$cfg" ]]; then
    echo '-- mita status --'
    sudo -u mita -- env \
      MITA_CONFIG_JSON_FILE="$cfg" \
      MITA_UDS_PATH="$uds" \
      MITA_LOG_NO_TIMESTAMP=true \
      /usr/local/x-ui-mieru/bin/mita status 2>&1 || true
    echo '-- mita metrics (identity redacted) --'
    sudo -u mita -- env \
      MITA_CONFIG_JSON_FILE="$cfg" \
      MITA_UDS_PATH="$uds" \
      MITA_LOG_NO_TIMESTAMP=true \
      /usr/local/x-ui-mieru/bin/mita get metrics 2>&1 | redact_metrics || true
  fi
done

section 'LISTENING SOCKETS'
shell 'ss -lntup 2>&1 | sed -n "1,260p"'

section 'DATABASE TRAFFIC SUMMARY'
if command -v sqlite3 >/dev/null 2>&1 && [[ -r /etc/x-ui/x-ui.db ]]; then
  shell "sqlite3 -readonly -header -column /etc/x-ui/x-ui.db \"SELECT id,remark,protocol,port,enable,up,down,tag FROM inbounds ORDER BY id;\" 2>&1 || true"
  shell "sqlite3 -readonly -header -column /etc/x-ui/x-ui.db \"SELECT COUNT(*) AS client_rows, COALESCE(SUM(up),0) AS total_up, COALESCE(SUM(down),0) AS total_down, SUM(CASE WHEN enable=1 THEN 1 ELSE 0 END) AS enabled_rows FROM client_traffics;\" 2>&1 || true"
else
  echo '[sqlite3 unavailable, PostgreSQL configured, or /etc/x-ui/x-ui.db not readable]'
  shell 'systemctl show x-ui.service -p Environment --no-pager 2>/dev/null | sed -E "s#(XUI_DB_DSN=[^:]+://[^:]+:)[^@]+@#\\1<redacted>@#g" || true'
fi

section 'RECENT KERNEL / SERVICE ERRORS'
shell 'journalctl -p warning..alert -n 120 --no-pager -o short-iso 2>/dev/null || true'

section 'END'
echo '3Xpatcher diagnostic collection complete.'
echo 'Paste the complete output back into the chat. Secrets in JSON configs are redacted by this script.'
