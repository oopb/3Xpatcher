#!/usr/bin/env bash
set -euo pipefail

[[ ${EUID:-$(id -u)} -eq 0 ]] || { echo "Run as root" >&2; exit 1; }
for cmd in curl tar systemctl sha256sum python3; do
  command -v "$cmd" >/dev/null || { echo "Missing dependency: $cmd" >&2; exit 1; }
done

BASE=/usr/local/x-ui-singbox
BIN="$BASE/bin"
CONF="$BASE/config"
BACKUP="$BASE/backup"
mkdir -p "$BASE" "$CONF" "$BACKUP"
chmod 700 "$BASE" "$CONF" "$BACKUP"

case "$(uname -m)" in
  x86_64|amd64) arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  *) echo "V1 installer currently supports Linux amd64/arm64 only." >&2; exit 1 ;;
esac

api='https://api.github.com/repos/SagerNet/sing-box/releases/latest'

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
curl -fsSL --retry 3 -o "$tmp/release.json" "$api"

# Resolve the latest official stable release and the exact Linux archive from the
# GitHub API. We deliberately use the asset's API-provided SHA256 digest instead
# of assuming a separate checksums filename, because release packaging can change.
mapfile -t release_meta < <(python3 - "$tmp/release.json" "$arch" <<'PYMETA'
import json, re, sys
p, arch = sys.argv[1], sys.argv[2]
with open(p, 'r', encoding='utf-8') as f:
    r = json.load(f)
tag = r.get('tag_name', '')
if r.get('draft') or r.get('prerelease') or not re.fullmatch(r'v\d+\.\d+\.\d+', tag):
    raise SystemExit(f'latest release is not a stable semver tag: {tag!r}')
ver = tag[1:]
name = f'sing-box-{ver}-linux-{arch}.tar.gz'
asset = next((a for a in r.get('assets', []) if a.get('name') == name), None)
if not asset:
    raise SystemExit(f'official release asset not found: {name}')
digest = asset.get('digest') or ''
if not re.fullmatch(r'sha256:[0-9a-fA-F]{64}', digest):
    raise SystemExit(f'official SHA256 digest missing for: {name}')
url = asset.get('browser_download_url') or ''
if not url.startswith('https://github.com/SagerNet/sing-box/releases/download/'):
    raise SystemExit('unexpected release asset URL')
print(tag)
print(name)
print(url)
print(digest.split(':', 1)[1])
PYMETA
)

[[ ${#release_meta[@]} -eq 4 ]] || { echo "Could not resolve stable sing-box release metadata" >&2; exit 1; }
tag=${release_meta[0]}
asset=${release_meta[1]}
asset_url=${release_meta[2]}
expected=${release_meta[3]}
ver=${tag#v}

curl -fL --retry 3 -o "$tmp/$asset" "$asset_url"
actual=$(sha256sum "$tmp/$asset" | awk '{print $1}')
[[ ${actual,,} == ${expected,,} ]] || { echo "SHA256 mismatch for $asset" >&2; exit 1; }

tar -xzf "$tmp/$asset" -C "$tmp"
dir="$tmp/sing-box-${ver}-linux-${arch}"
[[ -x "$dir/sing-box" ]] || { echo "Release archive layout changed: sing-box not found" >&2; exit 1; }

if [[ ! -f "$CONF/config.json" ]]; then
  cat > "$CONF/config.json" <<'JSON'
{
  "log": {"level": "info", "timestamp": true},
  "inbounds": []
}
JSON
  chmod 600 "$CONF/config.json"
fi

# Stage the complete official runtime directory first. This preserves libcronet
# shipped in purego Linux builds and lets us validate before touching the live binary.
stage="$BASE/.bin-new-$$"
rm -rf "$stage"
mkdir -p "$stage"
install -m 0755 "$dir/sing-box" "$stage/sing-box"
find "$dir" -maxdepth 1 -type f -name '*.so' -exec install -m 0755 {} "$stage/" \; 2>/dev/null || true
LD_LIBRARY_PATH="$stage" "$stage/sing-box" check -c "$CONF/config.json"

stamp=$(date +%Y%m%d-%H%M%S)
old=""
if [[ -d "$BIN" ]]; then
  old="$BACKUP/bin-$stamp"
  mv "$BIN" "$old"
fi
mv "$stage" "$BIN"

cat > /etc/systemd/system/x-ui-singbox.service <<EOF2
[Unit]
Description=3x-ui Supplemental sing-box Core
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
Environment=LD_LIBRARY_PATH=$BIN
ExecStart=$BIN/sing-box run -c $CONF/config.json
Restart=on-failure
RestartSec=3
LimitNOFILE=1048576
NoNewPrivileges=true

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

echo "Installed $($BIN/sing-box version | head -n1)"
echo "Service: x-ui-singbox.service"
echo "Config:  $CONF/config.json"
if [[ -n "$old" ]]; then
  echo "Previous runtime backup: $old"
fi

exit 0
