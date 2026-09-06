#!/usr/bin/env bash
set -Eeuo pipefail

OFFICIAL_REPO="${OFFICIAL_REPO:-MHSanaei/3x-ui}"
XUI_DIR="${XUI_DIR:-/usr/local/x-ui}"
XUI_SERVICE="${XUI_SERVICE:-x-ui.service}"
XUI_ENV_FILE="${XUI_ENV_FILE:-/etc/default/x-ui}"
DB_FILE="${DB_FILE:-/etc/x-ui/x-ui.db}"
CHECK_ONLY="${CHECK_ONLY:-0}"

SUPPLEMENTAL_PROTOCOLS=(tuic anytls shadowtls naive snell mieru)

red='\033[0;31m'; green='\033[0;32m'; yellow='\033[0;33m'; blue='\033[0;34m'; plain='\033[0m'
info() { echo -e "${blue}[3Xpatcher]${plain} $*"; }
ok()   { echo -e "${green}[3Xpatcher]${plain} $*"; }
warn() { echo -e "${yellow}[3Xpatcher] WARNING:${plain} $*" >&2; }
die()  { echo -e "${red}[3Xpatcher] ERROR:${plain} $*" >&2; exit 1; }

for cmd in python3; do
  command -v "$cmd" >/dev/null 2>&1 || die "Missing required command: $cmd"
done

read_env_value() {
  local key="$1" file="$2"
  [[ -r "$file" ]] || return 0
  python3 - "$file" "$key" <<'PY'
import shlex, sys
path, key = sys.argv[1:]
value = ""
with open(path, encoding="utf-8") as f:
    for raw in f:
        line = raw.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        k, v = line.split("=", 1)
        if k.strip() != key:
            continue
        lex = shlex.shlex(v, posix=True)
        lex.whitespace_split = True
        lex.commenters = ""
        parts = list(lex)
        value = " ".join(parts) if parts else ""
print(value)
PY
}

DB_TYPE="${XUI_DB_TYPE:-}"
DB_DSN="${XUI_DB_DSN:-}"
if [[ -z "$DB_TYPE" ]]; then DB_TYPE=$(read_env_value XUI_DB_TYPE "$XUI_ENV_FILE"); fi
if [[ -z "$DB_DSN" ]]; then DB_DSN=$(read_env_value XUI_DB_DSN "$XUI_ENV_FILE"); fi
DB_TYPE="${DB_TYPE:-sqlite}"
DB_TYPE=$(printf '%s' "$DB_TYPE" | tr '[:upper:]' '[:lower:]' | xargs)

check_sqlite_database() {
  [[ -f "$DB_FILE" ]] || die "SQLite database not found: $DB_FILE"
  local output rc
  set +e
  output=$(python3 - "$DB_FILE" "${SUPPLEMENTAL_PROTOCOLS[@]}" 2>&1 <<'PY'
import sqlite3, sys, urllib.parse
path = sys.argv[1]
protocols = tuple(x.lower() for x in sys.argv[2:])
uri = "file:" + urllib.parse.quote(path, safe="/") + "?mode=ro"
try:
    con = sqlite3.connect(uri, uri=True, timeout=5)
    con.execute("PRAGMA query_only = ON")
    marks = ",".join("?" for _ in protocols)
    rows = con.execute(
        f"SELECT id, protocol, port, COALESCE(remark, '') FROM inbounds "
        f"WHERE lower(protocol) IN ({marks}) ORDER BY id",
        protocols,
    ).fetchall()
except Exception as exc:
    print(f"database read failed: {exc}", file=sys.stderr)
    raise SystemExit(20)
finally:
    try:
        con.close()
    except Exception:
        pass
if rows:
    for ident, proto, port, remark in rows:
        clean = str(remark).replace("\t", " ").replace("\r", " ").replace("\n", " ")
        print(f"id={ident} protocol={proto} port={port} remark={clean}")
    raise SystemExit(10)
PY
)
  rc=$?
  set -e
  case "$rc" in
    0) return 0 ;;
    10)
      echo -e "${red}[3Xpatcher] Refusing to uninstall: supplemental inbounds still exist.${plain}" >&2
      printf '%s\n' "$output" >&2
      echo "Delete those inbounds from the panel first, then rerun uninstall.sh." >&2
      exit 3
      ;;
    *) die "Could not verify SQLite database safely: $output" ;;
  esac
}

check_postgres_database() {
  [[ -n "$DB_DSN" ]] || die "XUI_DB_TYPE=postgres but XUI_DB_DSN is empty."
  command -v psql >/dev/null 2>&1 || die "PostgreSQL backend detected but psql is not installed; refusing to uninstall without a database check."
  local list sql output rc
  list="'tuic','anytls','shadowtls','naive','snell','mieru'"
  sql="BEGIN READ ONLY; SELECT id, protocol, port, replace(replace(COALESCE(remark,''), E'\\n',' '), E'\\t',' ') FROM inbounds WHERE lower(protocol) IN (${list}) ORDER BY id; COMMIT;"
  set +e
  output=$(PGCONNECT_TIMEOUT=5 psql "$DB_DSN" -X -A -t -F $'\t' -v ON_ERROR_STOP=1 -c "$sql" 2>&1)
  rc=$?
  set -e
  (( rc == 0 )) || die "Could not verify PostgreSQL database safely: $output"
  output=$(printf '%s\n' "$output" | sed '/^BEGIN$/d;/^COMMIT$/d;/^[[:space:]]*$/d')
  if [[ -n "$output" ]]; then
    echo -e "${red}[3Xpatcher] Refusing to uninstall: supplemental inbounds still exist.${plain}" >&2
    printf '%s\n' "$output" >&2
    echo "Delete those inbounds from the panel first, then rerun uninstall.sh." >&2
    exit 3
  fi
}

info "Read-only database preflight (${DB_TYPE})..."
case "$DB_TYPE" in
  sqlite) check_sqlite_database ;;
  postgres|postgresql) check_postgres_database ;;
  *) die "Unsupported/unknown database backend: $DB_TYPE. Nothing was changed." ;;
esac
ok "Database guard passed: no TUIC / AnyTLS / ShadowTLS / Naive / Snell / Mieru inbounds remain."

if [[ "$CHECK_ONLY" == "1" ]]; then
  ok "CHECK_ONLY=1: no filesystem or service changes were made."
  exit 0
fi

[[ ${EUID:-$(id -u)} -eq 0 ]] || die "Run as root. Database was checked, but nothing was changed."
for cmd in curl tar sha256sum systemctl install find mktemp; do
  command -v "$cmd" >/dev/null 2>&1 || die "Missing required command: $cmd. Nothing was changed."
done
[[ -x "$XUI_DIR/x-ui" ]] || die "Existing x-ui binary not found: $XUI_DIR/x-ui"
systemctl cat "$XUI_SERVICE" >/dev/null 2>&1 || die "Existing $XUI_SERVICE was not found."

current_version=$($XUI_DIR/x-ui -v 2>/dev/null | tail -n1 | tr -d '\r' | xargs || true)
if [[ "$current_version" =~ ^v?([0-9]+\.[0-9]+\.[0-9]+)$ ]]; then
  version="${BASH_REMATCH[1]}"
else
  die "Could not determine installed 3x-ui version from: ${current_version:-empty}"
fi
ref="v${version}"

case "$(uname -m)" in
  x86_64|amd64) arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  i386|i686) arch=386 ;;
  armv7l) arch=armv7 ;;
  armv6l) arch=armv6 ;;
  armv5*) arch=armv5 ;;
  s390x) arch=s390x ;;
  *) die "Unsupported architecture for official restore: $(uname -m)" ;;
esac
asset="x-ui-linux-${arch}.tar.gz"
work=$(mktemp -d /var/tmp/3xpatcher-uninstall.XXXXXXXX)
trap 'rm -rf "$work"' EXIT

info "Downloading verified official 3x-ui ${ref} (${arch}) before making changes..."
curl -fsSL --retry 4 --retry-all-errors --connect-timeout 15 \
  "https://api.github.com/repos/${OFFICIAL_REPO}/releases/tags/${ref}" -o "$work/release.json"
mapfile -t meta < <(python3 - "$work/release.json" "$asset" <<'PY'
import json, sys
path, name = sys.argv[1:]
release = json.load(open(path, encoding="utf-8"))
asset = next((x for x in release.get("assets", []) if x.get("name") == name), None)
if not asset:
    raise SystemExit(1)
print(asset.get("browser_download_url", ""))
print(asset.get("digest", "") or "")
PY
)
[[ ${#meta[@]} -eq 2 && "${meta[0]}" == https://github.com/* ]] || die "Official release asset not found: ${asset}"
[[ "${meta[1]}" =~ ^sha256:([0-9a-fA-F]{64})$ ]] || die "Official release asset has no SHA256 digest; refusing an unverifiable restore."
expected="${BASH_REMATCH[1]}"
curl -fL --retry 4 --retry-all-errors --connect-timeout 15 "${meta[0]}" -o "$work/$asset"
actual=$(sha256sum "$work/$asset" | awk '{print $1}')
[[ "${actual,,}" == "${expected,,}" ]] || die "Official release SHA256 mismatch. Nothing was changed."
mkdir -p "$work/official"
tar -xzf "$work/$asset" -C "$work/official"
official=$(find "$work/official" -type f -name x-ui -perm -u+x | head -n1)
[[ -n "$official" ]] || die "Official x-ui binary not found in ${asset}."
official_version=$($official -v 2>/dev/null | tail -n1 | tr -d '\r' | xargs || true)
[[ "$official_version" == "$version" || "$official_version" == "$ref" ]] || die "Official binary version mismatch: got ${official_version:-empty}, expected ${version}."
cp -a "$XUI_DIR/x-ui" "$work/x-ui.before-uninstall"
ok "Verified official ${ref}; beginning uninstall. Database will not be modified."

singbox_was_active=0
systemctl is-active --quiet x-ui-singbox.service 2>/dev/null && singbox_was_active=1 || true
mapfile -t active_mieru_units < <(systemctl list-units --type=service --state=active --plain --no-legend 'x-ui-mieru@*.service' 2>/dev/null | awk '{print $1}')

systemctl stop "$XUI_SERVICE"
systemctl stop x-ui-singbox.service >/dev/null 2>&1 || true
for unit in "${active_mieru_units[@]:-}"; do
  [[ -n "$unit" ]] && systemctl stop "$unit" >/dev/null 2>&1 || true
done
install -m 0755 "$official" "$XUI_DIR/x-ui"

if ! systemctl start "$XUI_SERVICE"; then
  warn "Official panel failed to start; restoring the pre-uninstall x-ui binary."
  install -m 0755 "$work/x-ui.before-uninstall" "$XUI_DIR/x-ui"
  systemctl start "$XUI_SERVICE" >/dev/null 2>&1 || true
  (( singbox_was_active == 1 )) && systemctl start x-ui-singbox.service >/dev/null 2>&1 || true
  for unit in "${active_mieru_units[@]:-}"; do
    [[ -n "$unit" ]] && systemctl start "$unit" >/dev/null 2>&1 || true
  done
  journalctl -u "$XUI_SERVICE" -n 60 --no-pager >&2 || true
  die "Official panel restore failed; 3Xpatcher files were preserved."
fi
sleep 1
if ! systemctl is-active --quiet "$XUI_SERVICE"; then
  warn "Official panel did not stay active; restoring the pre-uninstall x-ui binary."
  systemctl stop "$XUI_SERVICE" >/dev/null 2>&1 || true
  install -m 0755 "$work/x-ui.before-uninstall" "$XUI_DIR/x-ui"
  systemctl start "$XUI_SERVICE" >/dev/null 2>&1 || true
  (( singbox_was_active == 1 )) && systemctl start x-ui-singbox.service >/dev/null 2>&1 || true
  for unit in "${active_mieru_units[@]:-}"; do
    [[ -n "$unit" ]] && systemctl start "$unit" >/dev/null 2>&1 || true
  done
  die "Official panel restore was unstable; 3Xpatcher files were preserved."
fi

systemctl disable --now x-ui-singbox.service >/dev/null 2>&1 || true
rm -f /etc/systemd/system/x-ui-singbox.service
while read -r unit; do
  [[ -n "$unit" ]] && systemctl disable --now "$unit" >/dev/null 2>&1 || true
done < <(systemctl list-units --all --plain --no-legend 'x-ui-mieru@*.service' 2>/dev/null | awk '{print $1}')
rm -f /etc/systemd/system/x-ui-mieru@.service

rm -rf \
  /usr/local/x-ui-singbox \
  /usr/local/x-ui-mieru \
  /var/lib/x-ui-mieru \
  /run/x-ui-mieru \
  /usr/local/share/3xpatcher \
  /etc/3xpatcher \
  /var/lib/3xpatcher
rmdir /var/lib/mita >/dev/null 2>&1 || true

systemctl daemon-reload
systemctl reset-failed x-ui-singbox.service >/dev/null 2>&1 || true
systemctl reset-failed 'x-ui-mieru@*.service' >/dev/null 2>&1 || true

residue=0
for path in /usr/local/x-ui-singbox /usr/local/x-ui-mieru /var/lib/x-ui-mieru /run/x-ui-mieru /usr/local/share/3xpatcher /etc/3xpatcher /var/lib/3xpatcher; do
  if [[ -e "$path" ]]; then
    warn "Residual path remains: $path"
    residue=1
  fi
done
if systemctl cat x-ui-singbox.service >/dev/null 2>&1; then warn "Residual unit remains: x-ui-singbox.service"; residue=1; fi
if systemctl cat x-ui-mieru@.service >/dev/null 2>&1; then warn "Residual unit remains: x-ui-mieru@.service"; residue=1; fi
(( residue == 0 )) || die "Official panel is restored, but some 3Xpatcher residue could not be removed."

ok "3Xpatcher completely uninstalled."
echo "Official panel: ${version}"
echo "Database: unchanged (${DB_TYPE})"
echo "Preserved: /etc/x-ui and all native 3x-ui data/configuration"
