#!/usr/bin/env bash
set -Eeuo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
PATCH_REPO="${PATCH_REPO:-oopb/3Xpatcher}"
BASE=/usr/local/x-ui-singbox
BIN="$BASE/bin"
CONF="$BASE/config"
BACKUP="$BASE/backup"
UNIT=/etc/systemd/system/x-ui-singbox.service
STATE_DIR=/etc/3xpatcher
STATS_ADDR_FILE="$STATE_DIR/singbox-stats.addr"
DEFAULT_STATS_ADDR=127.0.0.1:62789

err_report() {
  local code=$?
  echo "[3Xpatcher] sing-box installer failed (exit ${code}) near line ${BASH_LINENO[0]}: ${BASH_COMMAND}" >&2
  return "$code"
}
trap err_report ERR

[[ ${EUID:-$(id -u)} -eq 0 ]] || { echo "Run as root" >&2; exit 1; }
for cmd in curl tar systemctl sha256sum python3; do
  command -v "$cmd" >/dev/null || { echo "Missing dependency: $cmd" >&2; exit 1; }
done
[[ -r "$ROOT/SINGBOX_VERSION" && -r "$ROOT/UPSTREAM_COMPAT" ]] || { echo "Missing pinned sing-box/upstream version metadata" >&2; exit 1; }
[[ "$PATCH_REPO" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]] || { echo "Invalid PATCH_REPO" >&2; exit 1; }

mkdir -p "$BASE" "$CONF" "$BACKUP" "$STATE_DIR"
chmod 700 "$BASE" "$CONF" "$BACKUP" "$STATE_DIR"

case "$(uname -m)" in
  x86_64|amd64) arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  *) echo "sing-box runtime installer supports Linux amd64/arm64 only." >&2; exit 1 ;;
esac

version=$(tr -d '\r\n' < "$ROOT/SINGBOX_VERSION")
upstream=$(tr -d '\r\n' < "$ROOT/UPSTREAM_COMPAT")
[[ "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "Invalid SINGBOX_VERSION: $version" >&2; exit 1; }
[[ "$upstream" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "Invalid UPSTREAM_COMPAT: $upstream" >&2; exit 1; }
asset="sing-box-stats-${version}-linux-${arch}.tar.gz"
release_tag="prebuilt-${upstream}"

tmp=$(mktemp -d)
cleanup_tmp() { rm -rf "$tmp"; }
trap cleanup_tmp EXIT

curl -fsSL --retry 4 --retry-all-errors "https://api.github.com/repos/${PATCH_REPO}/releases/tags/${release_tag}" -o "$tmp/release.json"
mapfile -t meta < <(python3 - "$tmp/release.json" "$asset" <<'PY'
import json,re,sys
p,name=sys.argv[1:]
r=json.load(open(p,encoding='utf-8'))
a=next((x for x in r.get('assets',[]) if x.get('name')==name),None)
if not a: raise SystemExit(f'missing release asset: {name}')
digest=a.get('digest') or ''
if not re.fullmatch(r'sha256:[0-9a-fA-F]{64}',digest): raise SystemExit('asset has no SHA256 digest')
url=a.get('browser_download_url') or ''
if not url.startswith('https://github.com/'): raise SystemExit('unexpected asset URL')
print(url)
print(digest.split(':',1)[1])
PY
)
[[ ${#meta[@]} -eq 2 ]] || { echo "Unable to resolve $asset" >&2; exit 1; }
curl -fL --retry 4 --retry-all-errors --connect-timeout 15 "${meta[0]}" -o "$tmp/$asset"
actual=$(sha256sum "$tmp/$asset" | awk '{print $1}')
[[ "${actual,,}" == "${meta[1],,}" ]] || { echo "SHA256 mismatch for $asset" >&2; exit 1; }
mkdir -p "$tmp/runtime"
tar -xzf "$tmp/$asset" -C "$tmp/runtime"
[[ -x "$tmp/runtime/sing-box" ]] || { echo "Invalid sing-box runtime archive" >&2; exit 1; }

if [[ ! -f "$CONF/config.json" ]]; then
  cat > "$CONF/config.json" <<'JSON'
{
  "log": {"level": "info", "timestamp": true},
  "inbounds": []
}
JSON
  chmod 600 "$CONF/config.json"
fi

stage="$BASE/.bin-new-$$"
rm -rf "$stage"
mkdir -p "$stage"
install -m 0755 "$tmp/runtime/sing-box" "$stage/sing-box"
"$stage/sing-box" check -c "$CONF/config.json"

# Confirm that this is the stats-enabled build, not an upstream default binary
# that accepts ordinary configs but would later reject experimental.v2ray_api.
cat > "$tmp/stats-check.json" <<'JSON'
{
  "log":{"level":"error"},
  "experimental":{"v2ray_api":{"listen":"127.0.0.1:62789","stats":{"enabled":true,"inbounds":[],"users":[]}}},
  "inbounds":[]
}
JSON
"$stage/sing-box" check -c "$tmp/stats-check.json"

stamp=$(date +%Y%m%d-%H%M%S)
old=""
binary_changed=0
current_was_active=0
systemctl is-active --quiet x-ui-singbox.service 2>/dev/null && current_was_active=1 || true

had_unit=0
if [[ -f "$UNIT" ]]; then
  cp -a "$UNIT" "$tmp/x-ui-singbox.service.old"
  had_unit=1
fi
cp -a "$CONF/config.json" "$tmp/config.json.old"

had_stats_addr=0
if [[ -f "$STATS_ADDR_FILE" ]]; then
  cp -a "$STATS_ADDR_FILE" "$tmp/singbox-stats.addr.old"
  had_stats_addr=1
fi

rollback_runtime() {
  local reason=$1
  echo "[3Xpatcher] New sing-box did not remain healthy: $reason" >&2
  echo "[3Xpatcher] --- x-ui-singbox.service status ---" >&2
  systemctl status x-ui-singbox.service --no-pager -l >&2 || true
  echo "[3Xpatcher] --- recent x-ui-singbox.service journal ---" >&2
  journalctl -u x-ui-singbox.service -n 100 --no-pager >&2 || true

  systemctl stop x-ui-singbox.service >/dev/null 2>&1 || true

  if (( binary_changed == 1 )); then
    rm -rf "$BIN"
    if [[ -n "$old" && -d "$old" ]]; then
      mv "$old" "$BIN"
    fi
  fi

  install -m 0600 "$tmp/config.json.old" "$CONF/config.json" || true
  if (( had_stats_addr == 1 )); then
    install -m 0600 "$tmp/singbox-stats.addr.old" "$STATS_ADDR_FILE" || true
  else
    rm -f "$STATS_ADDR_FILE"
  fi

  if (( had_unit == 1 )); then
    install -m 0644 "$tmp/x-ui-singbox.service.old" "$UNIT"
  else
    rm -f "$UNIT"
  fi
  systemctl daemon-reload || true

  if (( current_was_active == 1 )) && [[ -d "$BIN" && $had_unit -eq 1 ]]; then
    if ! systemctl restart x-ui-singbox.service; then
      echo "[3Xpatcher] WARNING: previous sing-box runtime was restored but could not be restarted." >&2
    fi
  else
    echo "[3Xpatcher] NOTE: the previous sing-box service was not healthy/active before this install attempt; leaving it stopped." >&2
  fi

  echo "[3Xpatcher] Available runtime backups:" >&2
  find "$BACKUP" -mindepth 1 -maxdepth 1 -type d -name 'bin-*' -printf '  %f\n' 2>/dev/null | sort -r | head -n 8 >&2 || true
  exit 1
}

# Stop our own service before probing the stats address. This distinguishes a
# port legitimately held by the previous x-ui-singbox instance from a collision
# with an unrelated process. Never kill the unrelated process; choose another
# loopback-only port instead.
systemctl stop x-ui-singbox.service >/dev/null 2>&1 || true

if ! stats_addr=$(python3 - "$STATS_ADDR_FILE" "$DEFAULT_STATS_ADDR" <<'PY'
from pathlib import Path
import re
import socket
import sys

state_path = Path(sys.argv[1])
default = sys.argv[2]

def valid(addr):
    m = re.fullmatch(r"127\.0\.0\.1:(\d{1,5})", addr.strip())
    if not m:
        return None
    port = int(m.group(1))
    if not 1 <= port <= 65535:
        return None
    return port

def available(port):
    s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    try:
        s.bind(("127.0.0.1", port))
        return True
    except OSError:
        return False
    finally:
        s.close()

candidate = default
try:
    saved = state_path.read_text(encoding="utf-8").strip()
    if valid(saved) is not None:
        candidate = saved
except OSError:
    pass

port = valid(candidate)
if port is not None and available(port):
    print(f"127.0.0.1:{port}")
    raise SystemExit(0)

# 62000-62999 is deliberately above Linux's common default ephemeral range.
# Start after the historical port, then wrap around for deterministic reuse.
for port in list(range(62790, 63000)) + list(range(62000, 62789)):
    if available(port):
        print(f"127.0.0.1:{port}")
        raise SystemExit(0)
raise SystemExit("no free loopback stats port in 62000-62999")
PY
); then
  rollback_runtime "unable to allocate a loopback stats port"
fi

if [[ "$stats_addr" != "$DEFAULT_STATS_ADDR" ]]; then
  echo "[3Xpatcher] Stats address $DEFAULT_STATS_ADDR is unavailable or overridden; using $stats_addr."
else
  echo "[3Xpatcher] Stats address: $stats_addr"
fi

addr_tmp="$STATE_DIR/.singbox-stats.addr.$$"
printf '%s\n' "$stats_addr" > "$addr_tmp"
chmod 600 "$addr_tmp"
if ! mv -f "$addr_tmp" "$STATS_ADDR_FILE"; then
  rollback_runtime "failed to persist stats address"
fi

# Existing patched installs already have experimental.v2ray_api in config.json.
# Rewrite only its listen field; a fresh vanilla install has no v2ray_api yet,
# so the new panel will add it later using the same persisted address.
if ! python3 - "$CONF/config.json" "$stats_addr" <<'PY'
import json
import os
import stat
import sys
from pathlib import Path

path = Path(sys.argv[1])
addr = sys.argv[2]
data = json.loads(path.read_text(encoding="utf-8"))
exp = data.get("experimental")
changed = False
if isinstance(exp, dict):
    api = exp.get("v2ray_api")
    if isinstance(api, dict):
        api["listen"] = addr
        changed = True
if changed:
    mode = stat.S_IMODE(path.stat().st_mode)
    tmp = path.with_name(f".{path.name}.3xpatcher.tmp")
    tmp.write_text(json.dumps(data, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    os.chmod(tmp, mode)
    os.replace(tmp, path)
PY
then
  rollback_runtime "failed to update existing sing-box stats address"
fi

if ! "$stage/sing-box" check -c "$CONF/config.json"; then
  rollback_runtime "config check failed after stats-address migration"
fi

if [[ -d "$BIN" ]]; then
  old="$BACKUP/bin-$stamp"
  if ! mv "$BIN" "$old"; then
    rollback_runtime "failed to stage previous sing-box runtime backup"
  fi
  binary_changed=1
fi
if ! mv "$stage" "$BIN"; then
  rollback_runtime "failed to activate new sing-box binary directory"
fi
binary_changed=1

cat > "$UNIT" <<EOF2
[Unit]
Description=3x-ui Supplemental sing-box Core (stats enabled)
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=$BIN/sing-box run -c $CONF/config.json
Restart=on-failure
RestartSec=3
LimitNOFILE=1048576
NoNewPrivileges=true
# Do not hide or remount /root: native 3x-ui TLS certificates are commonly
# stored there. Some restricted VPS/LXC environments also reject ProtectHome
# mount namespacing entirely. The supplemental core still runs in its own unit.
ProtectHome=false
PrivateTmp=true

[Install]
WantedBy=multi-user.target
EOF2

if ! systemctl daemon-reload; then
  rollback_runtime "systemd daemon-reload failed"
fi
if ! systemctl enable x-ui-singbox.service >/dev/null; then
  rollback_runtime "systemd enable failed"
fi
if ! systemctl restart x-ui-singbox.service; then
  rollback_runtime "systemd restart returned failure"
fi

# Type=simple can report a successful restart before the process immediately
# exits. Require it to stay active through a short stabilization window.
for _ in 1 2 3 4; do
  sleep 1
  systemctl is-active --quiet x-ui-singbox.service || rollback_runtime "service exited during startup stabilization"
done

pid=$(systemctl show -p MainPID --value x-ui-singbox.service 2>/dev/null || true)
[[ "$pid" =~ ^[1-9][0-9]*$ ]] || rollback_runtime "service has no live MainPID"

# The unit may be healthy while a fresh vanilla config has no active inbounds;
# that is valid. At this point only process health is required.
echo "Installed $($BIN/sing-box version | head -n1) (3Xpatcher stats build)"
echo "Service: x-ui-singbox.service"
echo "Config:  $CONF/config.json"
echo "Stats:   $stats_addr (persisted in $STATS_ADDR_FILE)"
if [[ -n "$old" ]]; then
  echo "Previous runtime backup: $old"
fi
