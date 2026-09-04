#!/usr/bin/env bash
set -Eeuo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
PATCH_REPO="${PATCH_REPO:-oopb/3Xpatcher}"
BASE=/usr/local/x-ui-singbox
BIN="$BASE/bin"
CONF="$BASE/config"
BACKUP="$BASE/backup"
UNIT=/etc/systemd/system/x-ui-singbox.service

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

mkdir -p "$BASE" "$CONF" "$BACKUP"
chmod 700 "$BASE" "$CONF" "$BACKUP"

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
current_was_active=0
systemctl is-active --quiet x-ui-singbox.service 2>/dev/null && current_was_active=1 || true

had_unit=0
if [[ -f "$UNIT" ]]; then
  cp -a "$UNIT" "$tmp/x-ui-singbox.service.old"
  had_unit=1
fi

if [[ -d "$BIN" ]]; then
  old="$BACKUP/bin-$stamp"
  mv "$BIN" "$old"
fi
mv "$stage" "$BIN"

rollback_runtime() {
  local reason=$1
  echo "[3Xpatcher] New sing-box did not remain healthy: $reason" >&2
  echo "[3Xpatcher] --- x-ui-singbox.service status ---" >&2
  systemctl status x-ui-singbox.service --no-pager -l >&2 || true
  echo "[3Xpatcher] --- recent x-ui-singbox.service journal ---" >&2
  journalctl -u x-ui-singbox.service -n 100 --no-pager >&2 || true

  systemctl stop x-ui-singbox.service >/dev/null 2>&1 || true
  rm -rf "$BIN"
  if [[ -n "$old" && -d "$old" ]]; then
    mv "$old" "$BIN"
  fi

  if (( had_unit == 1 )); then
    install -m 0644 "$tmp/x-ui-singbox.service.old" "$UNIT"
  else
    rm -f "$UNIT"
  fi
  systemctl daemon-reload || true

  if [[ -d "$BIN" && $had_unit -eq 1 ]]; then
    if ! systemctl restart x-ui-singbox.service; then
      echo "[3Xpatcher] WARNING: previous sing-box runtime was restored but could not be restarted." >&2
    elif (( current_was_active == 0 )); then
      echo "[3Xpatcher] NOTE: the previous sing-box service was already inactive before this install attempt." >&2
    fi
  fi

  echo "[3Xpatcher] Available runtime backups:" >&2
  find "$BACKUP" -mindepth 1 -maxdepth 1 -type d -name 'bin-*' -printf '  %f\n' 2>/dev/null | sort -r | head -n 8 >&2 || true
  exit 1
}

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

# The unit may be healthy while the old config has no active inbounds; that is
# valid. At this point only process health is required.
echo "Installed $($BIN/sing-box version | head -n1) (3Xpatcher stats build)"
echo "Service: x-ui-singbox.service"
echo "Config:  $CONF/config.json"
echo "Stats:   127.0.0.1:62789 (configured by panel)"
if [[ -n "$old" ]]; then
  echo "Previous runtime backup: $old"
fi
