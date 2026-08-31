#!/usr/bin/env bash
set -Eeuo pipefail

# 3Xpatcher one-click installer
# Patches an existing 3x-ui installation with the supplemental sing-box core.
# It never replaces /usr/local/x-ui/bin/xray-*.

PATCH_REPO="${PATCH_REPO:-oopb/3Xpatcher}"
PATCH_REF="${PATCH_REF:-main}"
UPSTREAM_REPO="${UPSTREAM_REPO:-MHSanaei/3x-ui}"
UPSTREAM_REF="${UPSTREAM_REF:-}"
XUI_DIR="${XUI_DIR:-/usr/local/x-ui}"
XUI_SERVICE="${XUI_SERVICE:-x-ui.service}"
STATE_DIR="${STATE_DIR:-/var/lib/3xpatcher}"
WORK_PARENT="${WORK_PARENT:-/var/tmp}"
AUTO_SWAP="${AUTO_SWAP:-1}"
KEEP_WORK="${KEEP_WORK:-0}"
SKIP_SINGBOX="${SKIP_SINGBOX:-0}"

red='\033[0;31m'; green='\033[0;32m'; yellow='\033[0;33m'; blue='\033[0;34m'; plain='\033[0m'
info() { echo -e "${blue}[3Xpatcher]${plain} $*"; }
ok()   { echo -e "${green}[3Xpatcher]${plain} $*"; }
warn() { echo -e "${yellow}[3Xpatcher] WARNING:${plain} $*" >&2; }
die()  { echo -e "${red}[3Xpatcher] ERROR:${plain} $*" >&2; exit 1; }

[[ ${EUID:-$(id -u)} -eq 0 ]] || die "Run as root."
[[ "$(uname -s)" == "Linux" ]] || die "Linux only."
[[ "$PATCH_REPO" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]] || die "Invalid PATCH_REPO."
[[ "$UPSTREAM_REPO" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]] || die "Invalid UPSTREAM_REPO."
[[ "$PATCH_REF" =~ ^[A-Za-z0-9._/-]+$ ]] || die "Invalid PATCH_REF."
if [[ -n "$UPSTREAM_REF" && ! "$UPSTREAM_REF" =~ ^[A-Za-z0-9._/-]+$ ]]; then
  die "Invalid UPSTREAM_REF."
fi
command -v systemctl >/dev/null 2>&1 || die "systemd is required by V1."

case "$(uname -m)" in
  x86_64|amd64) ARCH=amd64; NODE_ARCH=x64 ;;
  aarch64|arm64) ARCH=arm64; NODE_ARCH=arm64 ;;
  *) die "V1 one-click installer supports amd64 and arm64 only." ;;
esac

[[ -x "$XUI_DIR/x-ui" ]] || die "Existing 3x-ui binary not found at $XUI_DIR/x-ui. Install official 3x-ui first."
systemctl cat "$XUI_SERVICE" >/dev/null 2>&1 || die "Existing $XUI_SERVICE was not found."

# V1 builds natively. Keep distro support conservative until release artifacts
# remove the build-time dependency from target VPSes.
if [[ -r /etc/os-release ]]; then
  # shellcheck disable=SC1091
  . /etc/os-release
else
  die "/etc/os-release not found."
fi
case "${ID:-}" in
  debian|ubuntu|armbian) ;;
  *) die "V1 one-click source builder currently supports Debian/Ubuntu/Armbian. Detected: ${ID:-unknown}." ;;
esac

STAMP=$(date -u +%Y%m%d-%H%M%S)
WORK=$(mktemp -d "$WORK_PARENT/3xpatcher.XXXXXXXX")
BACKUP_DIR="$STATE_DIR/backups/$STAMP"
mkdir -p "$BACKUP_DIR"
chmod 700 "$STATE_DIR" "$STATE_DIR/backups" "$BACKUP_DIR"

SUCCESS=0
BINARY_SWAPPED=0
SINGBOX_WAS_PRESENT=0
TEMP_SWAP=""
PATCH_ROOT=""
OLD_VERSION=""
TARGET_VERSION=""
TARGET_REF=""
SRC_DIR=""

cleanup() {
  local code=$?
  set +e

  if [[ "$SUCCESS" != "1" && "$BINARY_SWAPPED" == "1" && -f "$BACKUP_DIR/x-ui" ]]; then
    warn "Installation failed after replacing the panel binary; restoring the original binary."
    systemctl stop "$XUI_SERVICE" >/dev/null 2>&1 || true
    install -m 0755 "$BACKUP_DIR/x-ui" "$XUI_DIR/x-ui"
    systemctl start "$XUI_SERVICE" >/dev/null 2>&1 || true
  fi

  # If this run introduced sing-box and the panel patch did not complete, remove
  # only that freshly-created empty supplemental runtime. Never purge a preexisting
  # sing-box installation or its config.
  if [[ "$SUCCESS" != "1" && "$SINGBOX_WAS_PRESENT" == "0" && -n "$PATCH_ROOT" && -x "$PATCH_ROOT/scripts/uninstall-singbox.sh" ]]; then
    if systemctl cat x-ui-singbox.service >/dev/null 2>&1 || [[ -d /usr/local/x-ui-singbox ]]; then
      warn "Removing sing-box runtime created by the failed installation."
      PURGE=1 bash "$PATCH_ROOT/scripts/uninstall-singbox.sh" >/dev/null 2>&1 || true
    fi
  fi

  if [[ -n "$TEMP_SWAP" ]]; then
    swapoff "$TEMP_SWAP" >/dev/null 2>&1 || true
    rm -f "$TEMP_SWAP"
  fi

  if [[ "$KEEP_WORK" != "1" ]]; then
    rm -rf "$WORK"
  else
    warn "Keeping build workspace: $WORK"
  fi

  if [[ "$SUCCESS" != "1" ]]; then
    echo -e "${red}[3Xpatcher] Installation failed.${plain}" >&2
    echo "Backup: $BACKUP_DIR" >&2
  fi
  return "$code"
}
trap cleanup EXIT

install_deps() {
  info "Installing build prerequisites..."
  export DEBIAN_FRONTEND=noninteractive
  apt-get update -qq
  apt-get install -y -q --no-install-recommends \
    ca-certificates curl tar gzip xz-utils python3 git build-essential pkg-config util-linux >/dev/null
}

version_ge() {
  # version_ge ACTUAL REQUIRED
  [[ "$(printf '%s\n%s\n' "$2" "$1" | sort -V | head -n1)" == "$2" ]]
}

ensure_space() {
  local avail_kb
  avail_kb=$(df -Pk "$WORK_PARENT" | awk 'NR==2 {print $4}')
  [[ "$avail_kb" =~ ^[0-9]+$ ]] || return 0
  if (( avail_kb < 4194304 )); then
    warn "Less than 4 GiB free under $WORK_PARENT. The frontend/Go build may fail."
  fi
}

ensure_swap() {
  [[ "$AUTO_SWAP" == "1" ]] || return 0
  local mem_kb swap_kb total_kb need_kb
  mem_kb=$(awk '/MemTotal:/ {print $2}' /proc/meminfo)
  swap_kb=$(awk '/SwapTotal:/ {print $2}' /proc/meminfo)
  total_kb=$((mem_kb + swap_kb))
  # npm/Vite + Go/CGO can spike on small VPSes. Reach ~3 GiB combined.
  need_kb=$((3 * 1024 * 1024 - total_kb))
  (( need_kb > 0 )) || return 0
  (( need_kb > 2 * 1024 * 1024 )) && need_kb=$((2 * 1024 * 1024))

  local avail_kb
  avail_kb=$(df -Pk "$WORK_PARENT" | awk 'NR==2 {print $4}')
  if [[ "$avail_kb" =~ ^[0-9]+$ ]] && (( avail_kb < need_kb + 524288 )); then
    warn "Not enough free disk for temporary swap; continuing without it."
    return 0
  fi

  TEMP_SWAP="$WORK_PARENT/3xpatcher-swap-$STAMP"
  info "Low-memory host detected; creating temporary $((need_kb / 1024)) MiB swap."
  if command -v fallocate >/dev/null 2>&1; then
    fallocate -l "${need_kb}K" "$TEMP_SWAP" || true
  fi
  if [[ ! -s "$TEMP_SWAP" ]]; then
    dd if=/dev/zero of="$TEMP_SWAP" bs=1M count=$((need_kb / 1024)) status=none
  fi
  chmod 600 "$TEMP_SWAP"
  mkswap "$TEMP_SWAP" >/dev/null
  swapon "$TEMP_SWAP"
}

fetch_patch_tree() {
  local self_dir
  self_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" 2>/dev/null && pwd || true)
  if [[ -n "$self_dir" && -x "$self_dir/scripts/apply-overlay.sh" && -f "$self_dir/VERSION" ]]; then
    PATCH_ROOT="$self_dir"
    info "Using local 3Xpatcher tree: $PATCH_ROOT"
    return 0
  fi

  PATCH_ROOT="$WORK/patch"
  mkdir -p "$PATCH_ROOT"
  local url="https://codeload.github.com/${PATCH_REPO}/tar.gz/${PATCH_REF}"
  info "Downloading 3Xpatcher ${PATCH_REF}..."
  curl -fL --retry 4 --retry-all-errors --connect-timeout 15 "$url" -o "$WORK/patch.tar.gz"
  tar -xzf "$WORK/patch.tar.gz" -C "$PATCH_ROOT" --strip-components=1
  [[ -x "$PATCH_ROOT/scripts/apply-overlay.sh" ]] || die "Downloaded patch tree is incomplete."
}

normalize_installed_version() {
  local raw
  raw=$($XUI_DIR/x-ui -v 2>/dev/null | tail -n1 | tr -d '\r' | xargs || true)
  OLD_VERSION="$raw"
  if [[ "$raw" =~ ^v?([0-9]+\.[0-9]+\.[0-9]+)$ ]]; then
    echo "v${BASH_REMATCH[1]}"
    return 0
  fi
  return 1
}

latest_stable_upstream() {
  curl -fsSL --retry 4 --retry-all-errors \
    "https://api.github.com/repos/${UPSTREAM_REPO}/releases/latest" \
    | python3 -c 'import json,re,sys; r=json.load(sys.stdin); t=r.get("tag_name",""); ok=(not r.get("draft") and not r.get("prerelease") and re.fullmatch(r"v[0-9]+\.[0-9]+\.[0-9]+",t)); print(t if ok else "")'
}

fetch_upstream_source() {
  local detected=""
  if [[ -n "$UPSTREAM_REF" ]]; then
    TARGET_REF="$UPSTREAM_REF"
  elif detected=$(normalize_installed_version); then
    TARGET_REF="$detected"
    info "Detected installed 3x-ui version: ${OLD_VERSION}; preserving that upstream version."
  else
    warn "Could not map installed panel version '${OLD_VERSION:-unknown}' to a stable source tag."
    TARGET_REF=$(latest_stable_upstream)
    [[ -n "$TARGET_REF" ]] || die "Could not resolve latest stable 3x-ui release."
    info "Falling back to latest stable upstream: $TARGET_REF"
  fi

  SRC_DIR="$WORK/source"
  mkdir -p "$SRC_DIR"
  info "Downloading 3x-ui source $TARGET_REF..."
  if ! curl -fL --retry 4 --retry-all-errors --connect-timeout 15 \
      "https://codeload.github.com/${UPSTREAM_REPO}/tar.gz/${TARGET_REF}" \
      -o "$WORK/upstream.tar.gz"; then
    die "Unable to download 3x-ui source ref $TARGET_REF. Set UPSTREAM_REF explicitly if needed."
  fi
  tar -xzf "$WORK/upstream.tar.gz" -C "$SRC_DIR" --strip-components=1
  [[ -f "$SRC_DIR/go.mod" && -f "$SRC_DIR/frontend/package-lock.json" ]] || die "Unexpected upstream source archive layout."
}

ensure_go() {
  local src="$1" required current=""
  required=$(awk '$1=="go" {print $2; exit}' "$src/go.mod")
  [[ "$required" =~ ^[0-9]+\.[0-9]+(\.[0-9]+)?$ ]] || die "Could not parse required Go version from go.mod."
  if command -v go >/dev/null 2>&1; then
    current=$(go version | sed -n 's/.* go\([0-9][0-9.]*\).*/\1/p')
  fi
  if [[ -n "$current" ]] && version_ge "$current" "$required"; then
    info "Using system Go $current (required >= $required)."
    return 0
  fi

  info "Installing temporary Go $required toolchain..."
  local meta="$WORK/go-downloads.json"
  curl -fsSL --retry 4 --retry-all-errors 'https://go.dev/dl/?mode=json&include=all' -o "$meta"
  mapfile -t gometa < <(python3 - "$meta" "$required" "$ARCH" <<'PY'
import json,sys
p,ver,arch=sys.argv[1:]
with open(p,encoding='utf-8') as f: data=json.load(f)
want='go'+ver
for rel in data:
    if rel.get('version') != want: continue
    name=f'{want}.linux-{arch}.tar.gz'
    for x in rel.get('files',[]):
        if x.get('filename') == name:
            print(name); print(x.get('sha256','')); raise SystemExit
raise SystemExit(1)
PY
  )
  [[ ${#gometa[@]} -eq 2 && ${gometa[1]} =~ ^[0-9a-fA-F]{64}$ ]] || die "Official Go $required Linux/$ARCH archive metadata not found."
  curl -fL --retry 4 --retry-all-errors "https://go.dev/dl/${gometa[0]}" -o "$WORK/${gometa[0]}"
  echo "${gometa[1]}  $WORK/${gometa[0]}" | sha256sum -c - >/dev/null
  mkdir -p "$WORK/toolchains"
  tar -xzf "$WORK/${gometa[0]}" -C "$WORK/toolchains"
  export PATH="$WORK/toolchains/go/bin:$PATH"
  info "Using $(go version)."
}

ensure_node() {
  local src="$1" major current_major=""
  major=$(tr -dc '0-9' < "$src/.nvmrc" 2>/dev/null || true)
  [[ "$major" =~ ^[0-9]+$ ]] || die "Could not parse Node major from upstream .nvmrc."
  if command -v node >/dev/null 2>&1; then
    current_major=$(node -p 'process.versions.node.split(".")[0]' 2>/dev/null || true)
  fi
  if [[ "$current_major" =~ ^[0-9]+$ ]] && (( current_major == major )) && command -v npm >/dev/null 2>&1; then
    info "Using system Node $(node -v) (required major >= $major)."
    return 0
  fi

  info "Installing temporary Node.js v${major}.x toolchain..."
  local base="https://nodejs.org/dist/latest-v${major}.x"
  curl -fsSL --retry 4 --retry-all-errors "$base/SHASUMS256.txt" -o "$WORK/node-shasums.txt"
  local file expected
  file=$(awk -v a="$NODE_ARCH" '$2 ~ ("^node-v[0-9.]+-linux-" a "\\.tar\\.xz$") {print $2; exit}' "$WORK/node-shasums.txt")
  expected=$(awk -v f="$file" '$2==f {print $1; exit}' "$WORK/node-shasums.txt")
  [[ -n "$file" && "$expected" =~ ^[0-9a-fA-F]{64}$ ]] || die "Official Node v${major}.x Linux/$NODE_ARCH archive metadata not found."
  curl -fL --retry 4 --retry-all-errors "$base/$file" -o "$WORK/$file"
  echo "$expected  $WORK/$file" | sha256sum -c - >/dev/null
  mkdir -p "$WORK/toolchains/node"
  tar -xJf "$WORK/$file" -C "$WORK/toolchains/node" --strip-components=1
  export PATH="$WORK/toolchains/node/bin:$PATH"
  info "Using Node $(node -v), npm $(npm -v)."
}

backup_current_install() {
  info "Backing up current panel binary and configuration..."
  install -m 0755 "$XUI_DIR/x-ui" "$BACKUP_DIR/x-ui"
  printf '%s\n' "$OLD_VERSION" > "$BACKUP_DIR/panel-version.txt"
  systemctl cat "$XUI_SERVICE" > "$BACKUP_DIR/x-ui.service.txt" 2>/dev/null || true
  if [[ -d /etc/x-ui ]]; then
    tar -C /etc -czf "$BACKUP_DIR/etc-x-ui.tar.gz" x-ui
  fi
  if compgen -G "$XUI_DIR/bin/xray-*" >/dev/null; then
    sha256sum "$XUI_DIR"/bin/xray-* > "$BACKUP_DIR/xray.sha256"
  fi
}

verify_xray_untouched() {
  [[ -f "$BACKUP_DIR/xray.sha256" ]] || return 0
  if ! sha256sum -c "$BACKUP_DIR/xray.sha256" >/dev/null 2>&1; then
    die "Xray binary hash changed unexpectedly. Original panel binary will be restored."
  fi
  ok "Xray binaries are byte-for-byte unchanged."
}

build_patched_panel() {
  local src="$1"
  info "Applying dual-core source overlay..."
  bash "$PATCH_ROOT/scripts/apply-overlay.sh" "$src"

  # Run renderer unit tests against the exact upstream module before compiling.
  if [[ -f "$PATCH_ROOT/internal/singbox/config_test.go" ]]; then
    cp "$PATCH_ROOT/internal/singbox/config_test.go" "$src/internal/singbox/config_test.go"
  fi

  export GOCACHE="$WORK/cache/go-build"
  export GOMODCACHE="$WORK/cache/go-mod"
  export npm_config_cache="$WORK/cache/npm"
  mkdir -p "$GOCACHE" "$GOMODCACHE" "$npm_config_cache"

  info "Running supplemental core unit tests..."
  (cd "$src" && go test ./internal/singbox)
  rm -f "$src/internal/singbox/config_test.go"

  info "Building 3x-ui React frontend..."
  (cd "$src/frontend" && npm ci --no-audit --no-fund && npm run build)

  info "Compiling patched 3x-ui binary..."
  (cd "$src" && CGO_ENABLED=1 go build -buildvcs=false -trimpath -ldflags='-w -s' -o "$WORK/x-ui-patched" .)
  chmod 0755 "$WORK/x-ui-patched"
  TARGET_VERSION=$($WORK/x-ui-patched -v 2>/dev/null | tail -n1 | tr -d '\r' | xargs || true)
  [[ -n "$TARGET_VERSION" ]] || die "Patched binary did not report a panel version."
  ok "Patched panel compiled successfully: $TARGET_VERSION"
}

install_singbox_core() {
  if [[ "$SKIP_SINGBOX" == "1" ]]; then
    warn "SKIP_SINGBOX=1: not installing/updating supplemental sing-box runtime."
    return 0
  fi
  if systemctl cat x-ui-singbox.service >/dev/null 2>&1 || [[ -e /usr/local/x-ui-singbox ]]; then
    SINGBOX_WAS_PRESENT=1
  fi
  info "Installing/updating latest stable sing-box runtime..."
  bash "$PATCH_ROOT/scripts/install-singbox.sh"
}

activate_panel() {
  info "Activating patched panel. This restarts x-ui once; its Xray child may reconnect once during installation."
  systemctl stop "$XUI_SERVICE"
  BINARY_SWAPPED=1
  install -m 0755 "$WORK/x-ui-patched" "$XUI_DIR/x-ui"
  systemctl start "$XUI_SERVICE"

  local i
  for i in {1..20}; do
    if systemctl is-active --quiet "$XUI_SERVICE"; then
      sleep 1
      if systemctl is-active --quiet "$XUI_SERVICE"; then
        break
      fi
    fi
    sleep 1
  done
  systemctl is-active --quiet "$XUI_SERVICE" || {
    journalctl -u "$XUI_SERVICE" -n 60 --no-pager >&2 || true
    die "Patched x-ui service did not become active."
  }

  local live_version
  live_version=$($XUI_DIR/x-ui -v 2>/dev/null | tail -n1 | tr -d '\r' | xargs || true)
  [[ "$live_version" == "$TARGET_VERSION" ]] || die "Live panel version mismatch after replacement."
  verify_xray_untouched
}

persist_state() {
  mkdir -p /etc/3xpatcher /usr/local/share/3xpatcher
  chmod 700 /etc/3xpatcher

  # Keep a self-contained copy of the patch scripts used for this installation.
  rm -rf /usr/local/share/3xpatcher/current
  mkdir -p /usr/local/share/3xpatcher/current
  tar -C "$PATCH_ROOT" --exclude=.git -cf - . | tar -C /usr/local/share/3xpatcher/current -xf -

  local patch_version
  patch_version=$(cat "$PATCH_ROOT/VERSION" 2>/dev/null || echo unknown)
  {
    printf 'PATCH_REF=%q\n' "$PATCH_REF"
    printf 'PATCH_VERSION=%q\n' "$patch_version"
    printf 'UPSTREAM_REF=%q\n' "$TARGET_REF"
    printf 'PANEL_VERSION_BEFORE=%q\n' "$OLD_VERSION"
    printf 'PANEL_VERSION_AFTER=%q\n' "$TARGET_VERSION"
    printf 'BACKUP_DIR=%q\n' "$BACKUP_DIR"
    printf 'INSTALLED_AT=%q\n' "$STAMP"
  } > /etc/3xpatcher/install.env
  chmod 600 /etc/3xpatcher/install.env
}

main() {
  echo "============================================================"
  echo "  3Xpatcher — 3x-ui + stable sing-box supplemental core"
  echo "============================================================"
  info "Architecture: $ARCH"
  info "Existing panel binary: $XUI_DIR/x-ui"

  install_deps
  ensure_space
  ensure_swap
  fetch_patch_tree

  normalize_installed_version >/dev/null 2>&1 || true
  fetch_upstream_source

  backup_current_install
  ensure_go "$SRC_DIR"
  ensure_node "$SRC_DIR"
  build_patched_panel "$SRC_DIR"
  install_singbox_core
  activate_panel
  persist_state

  SUCCESS=1
  BINARY_SWAPPED=0

  echo
  ok "Installation complete."
  echo "Panel:        $OLD_VERSION -> $TARGET_VERSION"
  echo "Upstream ref: $TARGET_REF"
  if [[ "$SKIP_SINGBOX" != "1" ]]; then
    echo "Sing-box:     $(/usr/local/x-ui-singbox/bin/sing-box version 2>/dev/null | head -n1 || echo installed)"
    echo "Service:      x-ui-singbox.service"
  fi
  echo "Backup:       $BACKUP_DIR"
  echo "State:        /etc/3xpatcher/install.env"
  echo
  echo "Open the 3x-ui panel and select: Sing-box"
  echo "V1 protocols: TUIC / AnyTLS / ShadowTLS v3 / Naive"
  echo
  echo "Rollback: bash <(curl -fsSL https://raw.githubusercontent.com/${PATCH_REPO}/${PATCH_REF}/rollback.sh)"
}

main "$@"
