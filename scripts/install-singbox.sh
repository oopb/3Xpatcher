#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
PATCH_REPO="${PATCH_REPO:-oopb/3Xpatcher}"
BASE=/usr/local/x-ui-singbox
BIN="$BASE/bin"
CONF="$BASE/config"
BACKUP="$BASE/backup"

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
trap 'rm -rf "$tmp"' EXIT
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
if [[ -d "$BIN" ]]; then
  old="$BACKUP/bin-$stamp"
  mv "$BIN" "$old"
fi
mv "$stage" "$BIN"

cat > /etc/systemd/system/x-ui-singbox.service <<EOF2
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
# Native 3x-ui certificates are frequently stored below /root. Hide-home breaks
# TUIC/AnyTLS/Naive only at service runtime even though panel-side `sing-box check`
# succeeds. Keep home trees readable but immutable to the supplemental runtime.
ProtectHome=read-only
PrivateTmp=true

[Install]
WantedBy=multi-user.target
EOF2

systemctl daemon-reload
systemctl enable x-ui-singbox.service >/dev/null
if ! systemctl restart x-ui-singbox.service; then
  echo "New sing-box failed to start; rolling back binary directory." >&2
  systemctl stop x-ui-singbox.service 2>/dev/null || true
  failed="$BACKUP/failed-bin-$stamp"
  mv "$BIN" "$failed" || true
  if [[ -n "$old" && -d "$old" ]]; then
    mv "$old" "$BIN"
    systemctl restart x-ui-singbox.service || true
  fi
  exit 1
fi
sleep 1
systemctl is-active --quiet x-ui-singbox.service

echo "Installed $($BIN/sing-box version | head -n1) (3Xpatcher stats build)"
echo "Service: x-ui-singbox.service"
echo "Config:  $CONF/config.json"
echo "Stats:   127.0.0.1:62789 (configured by panel)"
if [[ -n "$old" ]]; then
  echo "Previous runtime backup: $old"
fi
