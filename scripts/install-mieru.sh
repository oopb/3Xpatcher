#!/usr/bin/env bash
set -Eeuo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
VERSION_FILE="$ROOT/MIERU_VERSION"
INSTALL_ROOT="${MIERU_ROOT:-/usr/local/x-ui-mieru}"
SERVICE_FILE="${MIERU_SERVICE_FILE:-/etc/systemd/system/x-ui-mieru@.service}"
REPO="${MIERU_REPO:-enfein/mieru}"

[[ ${EUID:-$(id -u)} -eq 0 ]] || { echo "install-mieru.sh must run as root" >&2; exit 1; }
[[ -r "$VERSION_FILE" ]] || { echo "Missing $VERSION_FILE" >&2; exit 1; }
for cmd in curl python3 sha256sum dpkg-deb systemctl; do command -v "$cmd" >/dev/null || { echo "Missing required command: $cmd" >&2; exit 1; }; done

version=$(tr -d '\r\n' < "$VERSION_FILE")
[[ "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "Invalid MIERU_VERSION: $version" >&2; exit 1; }
plain=${version#v}
case "$(uname -m)" in
  x86_64|amd64) arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  *) echo "Mieru runtime supports amd64/arm64 in this installer" >&2; exit 1 ;;
esac
asset="mita_${plain}_${arch}.deb"
work=$(mktemp -d /var/tmp/3xpatcher-mieru.XXXXXXXX)
trap 'rm -rf "$work"' EXIT

curl -fsSL --retry 4 --retry-all-errors "https://api.github.com/repos/${REPO}/releases/tags/${version}" -o "$work/release.json"
mapfile -t meta < <(python3 - "$work/release.json" "$asset" <<'PY'
import json,sys
p,name=sys.argv[1:]
r=json.load(open(p,encoding='utf-8'))
a=next((x for x in r.get('assets',[]) if x.get('name')==name),None)
if not a: raise SystemExit(1)
print(a.get('browser_download_url',''))
print(a.get('digest','') or '')
PY
)
[[ ${#meta[@]} -eq 2 && "${meta[0]}" == https://github.com/* ]] || { echo "Mieru asset not found: $asset" >&2; exit 1; }
[[ "${meta[1]}" =~ ^sha256:([0-9a-fA-F]{64})$ ]] || { echo "Mieru release asset has no SHA256 digest" >&2; exit 1; }
expected="${BASH_REMATCH[1]}"
curl -fL --retry 4 --retry-all-errors --connect-timeout 15 "${meta[0]}" -o "$work/$asset"
actual=$(sha256sum "$work/$asset" | awk '{print $1}')
[[ "${actual,,}" == "${expected,,}" ]] || { echo "Mieru SHA256 mismatch" >&2; exit 1; }

dpkg-deb -x "$work/$asset" "$work/root"
bin=$(find "$work/root" -type f -path '*/bin/mita' -perm -u+x | head -n1)
[[ -n "$bin" ]] || { echo "mita binary not found in $asset" >&2; exit 1; }
install -d -m 0755 "$INSTALL_ROOT/bin"
install -d -m 0700 "$INSTALL_ROOT/config"
install -m 0755 "$bin" "$INSTALL_ROOT/bin/mita"

cat > "$SERVICE_FILE" <<'EOF'
[Unit]
Description=3Xpatcher Mieru inbound %i
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
Environment="MITA_CONFIG_JSON_FILE=/usr/local/x-ui-mieru/config/%i.json"
Environment="MITA_UDS_PATH=/run/x-ui-mieru/%i.sock"
Environment="MITA_LOG_NO_TIMESTAMP=true"
ExecStartPre=+/usr/bin/mkdir -p /run/x-ui-mieru
ExecStart=/usr/local/x-ui-mieru/bin/mita run
Restart=on-failure
RestartSec=3
LimitNOFILE=1048576
NoNewPrivileges=true
PrivateTmp=true
ProtectHome=true
ProtectSystem=full

[Install]
WantedBy=multi-user.target
EOF
chmod 0644 "$SERVICE_FILE"
systemctl daemon-reload

installed=$($INSTALL_ROOT/bin/mita version 2>&1 | head -n1 | tr -d '\r' || true)
[[ -n "$installed" ]] || { echo "Installed mita failed version check" >&2; exit 1; }
echo "Mieru runtime installed: $installed"
echo "Binary: $INSTALL_ROOT/bin/mita"
echo "Config: $INSTALL_ROOT/config/<inbound-id>.json"
echo "Service: x-ui-mieru@<inbound-id>.service"
