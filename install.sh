#!/usr/bin/env bash
set -Eeuo pipefail

# 3Xpatcher fast installer.
# The patched 3x-ui binary is built by GitHub Actions; target VPSes only download
# and activate it. Xray binaries under /usr/local/x-ui/bin/xray-* are never replaced.

PATCH_REPO="${PATCH_REPO:-oopb/3Xpatcher}"
PATCH_REF="${PATCH_REF:-main}"
XUI_DIR="${XUI_DIR:-/usr/local/x-ui}"
XUI_SERVICE="${XUI_SERVICE:-x-ui.service}"
STATE_DIR="${STATE_DIR:-/var/lib/3xpatcher}"
WORK_PARENT="${WORK_PARENT:-/var/tmp}"
SKIP_SINGBOX="${SKIP_SINGBOX:-0}"
SKIP_MIERU="${SKIP_MIERU:-0}"
KEEP_WORK="${KEEP_WORK:-0}"

red='\033[0;31m'; green='\033[0;32m'; yellow='\033[0;33m'; blue='\033[0;34m'; plain='\033[0m'
info() { echo -e "${blue}[3Xpatcher]${plain} $*"; }
ok()   { echo -e "${green}[3Xpatcher]${plain} $*"; }
warn() { echo -e "${yellow}[3Xpatcher] WARNING:${plain} $*" >&2; }
die()  { echo -e "${red}[3Xpatcher] ERROR:${plain} $*" >&2; exit 1; }

[[ ${EUID:-$(id -u)} -eq 0 ]] || die "Run as root."
[[ "$(uname -s)" == Linux ]] || die "Linux only."
[[ "$PATCH_REPO" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]] || die "Invalid PATCH_REPO."
[[ "$PATCH_REF" =~ ^[A-Za-z0-9._/-]+$ ]] || die "Invalid PATCH_REF."
command -v systemctl >/dev/null 2>&1 || die "systemd is required."

case "$(uname -m)" in
  x86_64|amd64) ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) die "Fast installer currently supports amd64 and arm64 only." ;;
esac

[[ -x "$XUI_DIR/x-ui" ]] || die "Existing 3x-ui binary not found at $XUI_DIR/x-ui."
systemctl cat "$XUI_SERVICE" >/dev/null 2>&1 || die "Existing $XUI_SERVICE was not found."

if [[ -r /etc/os-release ]]; then
  # shellcheck disable=SC1091
  . /etc/os-release
else
  die "/etc/os-release not found."
fi
case "${ID:-}" in
  debian|ubuntu|armbian) ;;
  *) die "Fast installer currently supports Debian/Ubuntu/Armbian. Detected: ${ID:-unknown}." ;;
esac

ensure_runtime_deps() {
  local missing=0 cmd
  for cmd in curl tar gzip python3 sha256sum dpkg-deb; do
    command -v "$cmd" >/dev/null 2>&1 || missing=1
  done
  (( missing == 0 )) && return 0
  info "Installing small runtime prerequisites..."
  export DEBIAN_FRONTEND=noninteractive
  apt-get update -qq
  apt-get install -y -q --no-install-recommends ca-certificates curl tar gzip python3 coreutils dpkg >/dev/null
}

installed_version() {
  local raw
  raw=$($XUI_DIR/x-ui -v 2>/dev/null | tail -n1 | tr -d '\r' | xargs || true)
  if [[ "$raw" =~ ^v?([0-9]+\.[0-9]+\.[0-9]+)$ ]]; then
    PANEL_VERSION="${BASH_REMATCH[1]}"
    UPSTREAM_REF="v${PANEL_VERSION}"
    return 0
  fi
  die "Could not identify a stable installed 3x-ui version from: ${raw:-empty}."
}

STAMP=$(date -u +%Y%m%d-%H%M%S)
WORK=$(mktemp -d "$WORK_PARENT/3xpatcher.XXXXXXXX")
BACKUP_DIR="$STATE_DIR/backups/$STAMP"
PATCH_ROOT="$WORK/patch"
SUCCESS=0
BINARY_SWAPPED=0
SINGBOX_WAS_PRESENT=0
MIERU_WAS_PRESENT=0
SINGBOX_INSTALL_STARTED=0
MIERU_INSTALL_STARTED=0
PANEL_VERSION=""
UPSTREAM_REF=""
TARGET_VERSION=""

cleanup() {
  local code=$?
  set +e
  if [[ "$SUCCESS" != 1 && "$BINARY_SWAPPED" == 1 && -f "$BACKUP_DIR/x-ui" ]]; then
    warn "Activation failed; restoring original panel binary."
    systemctl stop "$XUI_SERVICE" >/dev/null 2>&1 || true
    install -m 0755 "$BACKUP_DIR/x-ui" "$XUI_DIR/x-ui"
    systemctl start "$XUI_SERVICE" >/dev/null 2>&1 || true
  fi
  # Only purge a runtime when this invocation actually entered that runtime's
  # installer and the runtime did not exist beforehand. Early failures such as
  # stale/missing prebuilt assets must never touch an existing supplemental core.
  if [[ "$SUCCESS" != 1 && "$SINGBOX_INSTALL_STARTED" == 1 && "$SINGBOX_WAS_PRESENT" == 0 && -f "$PATCH_ROOT/scripts/uninstall-singbox.sh" ]]; then
    if systemctl cat x-ui-singbox.service >/dev/null 2>&1 || [[ -d /usr/local/x-ui-singbox ]]; then
      warn "Removing sing-box runtime introduced by the failed installation."
      PURGE=1 bash "$PATCH_ROOT/scripts/uninstall-singbox.sh" >/dev/null 2>&1 || true
    fi
  fi
  if [[ "$SUCCESS" != 1 && "$MIERU_INSTALL_STARTED" == 1 && "$MIERU_WAS_PRESENT" == 0 && -f "$PATCH_ROOT/scripts/uninstall-mieru.sh" ]]; then
    if [[ -d /usr/local/x-ui-mieru || -f /etc/systemd/system/x-ui-mieru@.service ]]; then
      warn "Removing Mieru runtime introduced by the failed installation."
      PURGE=1 bash "$PATCH_ROOT/scripts/uninstall-mieru.sh" >/dev/null 2>&1 || true
    fi
  fi
  if [[ "$KEEP_WORK" != 1 ]]; then rm -rf "$WORK"; else warn "Keeping workspace: $WORK"; fi
  if [[ "$SUCCESS" != 1 ]]; then
    echo -e "${red}[3Xpatcher] Installation failed.${plain}" >&2
    [[ -d "$BACKUP_DIR" ]] && echo "Backup: $BACKUP_DIR" >&2
  fi
  return "$code"
}
trap cleanup EXIT

fetch_patch_tree() {
  mkdir -p "$PATCH_ROOT"
  info "Downloading 3Xpatcher ${PATCH_REF}..."
  curl -fL --retry 4 --retry-all-errors --connect-timeout 15 \
    "https://codeload.github.com/${PATCH_REPO}/tar.gz/${PATCH_REF}" \
    -o "$WORK/patch.tar.gz"
  tar -xzf "$WORK/patch.tar.gz" -C "$PATCH_ROOT" --strip-components=1
  rm -f "$WORK/patch.tar.gz"
  [[ -f "$PATCH_ROOT/VERSION" && -f "$PATCH_ROOT/scripts/install-singbox.sh" && -f "$PATCH_ROOT/scripts/install-mieru.sh" && -f "$PATCH_ROOT/MIERU_VERSION" ]] \
    || die "Downloaded patch tree is incomplete."
}

download_prebuilt_panel() {
  [[ "$PATCH_REF" == main ]] || die "Prebuilt binaries are published for PATCH_REF=main only."
  local release_tag="prebuilt-${UPSTREAM_REF}"
  local asset="x-ui-patched-${UPSTREAM_REF}-linux-${ARCH}.tar.gz"
  local json="$WORK/release.json"
  info "Fetching prebuilt patched panel for ${UPSTREAM_REF}/${ARCH}..."
  if ! curl -fsSL --retry 3 --retry-all-errors \
      "https://api.github.com/repos/${PATCH_REPO}/releases/tags/${release_tag}" -o "$json"; then
    die "No compatible prebuilt release exists for ${UPSTREAM_REF}/${ARCH}. Do not source-build on this VPS; update 3Xpatcher compatibility first."
  fi

  mapfile -t meta < <(python3 - "$json" "$asset" <<'PY'
import json,sys
p,name=sys.argv[1:]
with open(p,encoding='utf-8') as f: r=json.load(f)
if r.get('draft'): raise SystemExit(1)
a=next((x for x in r.get('assets',[]) if x.get('name')==name),None)
if not a: raise SystemExit(1)
print(a.get('browser_download_url',''))
print(a.get('digest','') or '')
PY
  )
  [[ ${#meta[@]} -eq 2 && "${meta[0]}" == https://github.com/* ]] || die "Prebuilt asset $asset was not found."

  curl -fL --retry 4 --retry-all-errors --connect-timeout 15 "${meta[0]}" -o "$WORK/$asset"
  if [[ "${meta[1]}" =~ ^sha256:([0-9a-fA-F]{64})$ ]]; then
    local actual expected="${BASH_REMATCH[1]}"
    actual=$(sha256sum "$WORK/$asset" | awk '{print $1}')
    [[ "${actual,,}" == "${expected,,}" ]] || die "Prebuilt panel SHA256 mismatch."
  else
    die "GitHub release asset digest is missing; refusing an unverifiable panel binary."
  fi

  mkdir -p "$WORK/prebuilt"
  tar -xzf "$WORK/$asset" -C "$WORK/prebuilt"
  rm -f "$WORK/$asset" "$json"
  [[ -x "$WORK/prebuilt/x-ui" && -f "$WORK/prebuilt/build.env" ]] || die "Invalid prebuilt archive layout."

  mapfile -t buildmeta < <(python3 - "$WORK/prebuilt/build.env" <<'PY'
from pathlib import Path
import sys
vals={}
for line in Path(sys.argv[1]).read_text().splitlines():
    if '=' in line:
        k,v=line.split('=',1); vals[k]=v
for k in ('PATCH_VERSION','UPSTREAM_REF','ARCH'):
    print(vals.get(k,''))
PY
  )
  local patch_version
  patch_version=$(tr -d '\r\n' < "$PATCH_ROOT/VERSION")
  [[ "${buildmeta[0]:-}" == "$patch_version" ]] || die "Prebuilt patch version is stale; retry after the current Actions build finishes."
  [[ "${buildmeta[1]:-}" == "$UPSTREAM_REF" ]] || die "Prebuilt upstream version mismatch."
  [[ "${buildmeta[2]:-}" == "$ARCH" ]] || die "Prebuilt architecture mismatch."

  install -m 0755 "$WORK/prebuilt/x-ui" "$WORK/x-ui-patched"
  TARGET_VERSION=$($WORK/x-ui-patched -v 2>/dev/null | tail -n1 | tr -d '\r' | xargs || true)
  [[ "$TARGET_VERSION" == "$PANEL_VERSION" ]] || die "Patched binary version mismatch: expected $PANEL_VERSION, got ${TARGET_VERSION:-empty}."
  ok "Verified prebuilt patched panel: $TARGET_VERSION ($ARCH)."
}

backup_current_install() {
  info "Backing up current panel binary/configuration..."
  mkdir -p "$BACKUP_DIR"
  chmod 700 "$STATE_DIR" "$STATE_DIR/backups" "$BACKUP_DIR" 2>/dev/null || true
  install -m 0755 "$XUI_DIR/x-ui" "$BACKUP_DIR/x-ui"
  printf '%s\n' "$PANEL_VERSION" > "$BACKUP_DIR/panel-version.txt"
  systemctl cat "$XUI_SERVICE" > "$BACKUP_DIR/x-ui.service.txt" 2>/dev/null || true
  [[ -d /etc/x-ui ]] && tar -C /etc -czf "$BACKUP_DIR/etc-x-ui.tar.gz" x-ui
  if compgen -G "$XUI_DIR/bin/xray-*" >/dev/null; then
    sha256sum "$XUI_DIR"/bin/xray-* > "$BACKUP_DIR/xray.sha256"
  fi
}

install_singbox_core() {
  [[ "$SKIP_SINGBOX" == 1 ]] && { warn "SKIP_SINGBOX=1: leaving sing-box runtime unchanged."; return 0; }
  if systemctl cat x-ui-singbox.service >/dev/null 2>&1 || [[ -e /usr/local/x-ui-singbox ]]; then SINGBOX_WAS_PRESENT=1; fi
  SINGBOX_INSTALL_STARTED=1
  info "Installing/updating stable sing-box runtime..."
  bash "$PATCH_ROOT/scripts/install-singbox.sh"
}

install_mieru_core() {
  [[ "$SKIP_MIERU" == 1 ]] && { warn "SKIP_MIERU=1: leaving Mieru runtime unchanged."; return 0; }
  if [[ -e /usr/local/x-ui-mieru || -f /etc/systemd/system/x-ui-mieru@.service ]]; then MIERU_WAS_PRESENT=1; fi
  MIERU_INSTALL_STARTED=1
  info "Installing/updating official Mieru mita runtime..."
  bash "$PATCH_ROOT/scripts/install-mieru.sh"
}

verify_xray_untouched() {
  [[ -f "$BACKUP_DIR/xray.sha256" ]] || return 0
  sha256sum -c "$BACKUP_DIR/xray.sha256" >/dev/null 2>&1 || die "Xray binary hash changed unexpectedly; panel will be rolled back."
  ok "Xray binaries are byte-for-byte unchanged."
}

activate_panel() {
  info "Activating patched panel; x-ui/Xray will restart once."
  systemctl stop "$XUI_SERVICE"
  BINARY_SWAPPED=1
  install -m 0755 "$WORK/x-ui-patched" "$XUI_DIR/x-ui"
  systemctl start "$XUI_SERVICE"
  local i
  for i in {1..20}; do
    if systemctl is-active --quiet "$XUI_SERVICE"; then sleep 1; systemctl is-active --quiet "$XUI_SERVICE" && break; fi
    sleep 1
  done
  if ! systemctl is-active --quiet "$XUI_SERVICE"; then
    journalctl -u "$XUI_SERVICE" -n 60 --no-pager >&2 || true
    die "Patched x-ui service did not become active."
  fi
  local live
  live=$($XUI_DIR/x-ui -v 2>/dev/null | tail -n1 | tr -d '\r' | xargs || true)
  [[ "$live" == "$TARGET_VERSION" ]] || die "Live panel version mismatch after activation."
  verify_xray_untouched
}

persist_state() {
  mkdir -p /etc/3xpatcher /usr/local/share/3xpatcher/current
  chmod 700 /etc/3xpatcher
  rm -rf /usr/local/share/3xpatcher/current
  mkdir -p /usr/local/share/3xpatcher/current
  tar -C "$PATCH_ROOT" --exclude=.git -cf - . | tar -C /usr/local/share/3xpatcher/current -xf -
  local patch_version mieru_version
  patch_version=$(tr -d '\r\n' < "$PATCH_ROOT/VERSION")
  mieru_version=$(tr -d '\r\n' < "$PATCH_ROOT/MIERU_VERSION")
  {
    printf 'PATCH_REF=%q\n' "$PATCH_REF"
    printf 'PATCH_VERSION=%q\n' "$patch_version"
    printf 'UPSTREAM_REF=%q\n' "$UPSTREAM_REF"
    printf 'MIERU_VERSION=%q\n' "$mieru_version"
    printf 'PANEL_VERSION_BEFORE=%q\n' "$PANEL_VERSION"
    printf 'PANEL_VERSION_AFTER=%q\n' "$TARGET_VERSION"
    printf 'BACKUP_DIR=%q\n' "$BACKUP_DIR"
    printf 'INSTALLED_AT=%q\n' "$STAMP"
    printf 'INSTALL_MODE=%q\n' prebuilt
  } > /etc/3xpatcher/install.env
  chmod 600 /etc/3xpatcher/install.env
}

main() {
  echo "============================================================"
  echo "  3Xpatcher — fast prebuilt installer"
  echo "============================================================"
  info "Architecture: $ARCH"
  ensure_runtime_deps
  installed_version
  info "Installed 3x-ui: $PANEL_VERSION"
  fetch_patch_tree
  download_prebuilt_panel
  backup_current_install
  install_singbox_core
  install_mieru_core
  activate_panel
  persist_state
  SUCCESS=1
  BINARY_SWAPPED=0

  echo
  ok "Installation complete."
  echo "Panel:        $PANEL_VERSION -> $TARGET_VERSION"
  echo "Upstream ref: $UPSTREAM_REF"
  echo "Install mode: prebuilt"
  echo "Backup:       $BACKUP_DIR"
  echo "State:        /etc/3xpatcher/install.env"
  echo "Protocols:    TUIC / AnyTLS / ShadowTLS v3 / Naive / Snell v5 / Mieru"
  echo "Rollback:     bash <(curl -fsSL https://raw.githubusercontent.com/${PATCH_REPO}/${PATCH_REF}/rollback.sh)"
}

main "$@"
