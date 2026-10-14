#!/usr/bin/env bash
set -euo pipefail
STAGE="${1:?Usage: verify-v1-linux-native.sh stage arch}"
ARCH="${2:?Missing architecture}"
[[ "$(uname -s)" == Linux ]] || { printf 'Native Linux verification requires Linux\n' >&2; exit 1; }
case "$(uname -m)/$ARCH" in x86_64/amd64|aarch64/arm64) ;; *) printf 'Runner architecture does not match package\n' >&2; exit 1 ;; esac
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TEMP="$(mktemp -d)"
trap 'rm -rf "$TEMP"' EXIT
shopt -s nullglob
DEBS=("$STAGE"/*.deb)
[[ ${#DEBS[@]} == 1 ]] || { printf 'Expected exactly one Debian package\n' >&2; exit 1; }
dpkg-deb --info "${DEBS[0]}"
dpkg-deb --extract "${DEBS[0]}" "$TEMP/root"
APP="$TEMP/root/opt/quosparr"
cmp "$STAGE/quosparr" "$APP/quosparr"
cmp "$STAGE/libmygo-sqlite3.so" "$APP/libmygo-sqlite3.so"
MYGO_SQLITE_LIBRARY="$APP/libmygo-sqlite3.so" go run "$ROOT/Scripts/verify-sqlite"
mkdir -p "$TEMP/home" "$TEMP/home/codex" "$TEMP/home/grok"
HOME="$TEMP/home" CODEX_HOME="$TEMP/home/codex" GROK_HOME="$TEMP/home/grok" \
  QUOSPARR_PREFS_DOMAIN="com.quosparr.ci.package" "$APP/quosparr" --self-check > "$TEMP/self-check.log"
cat "$TEMP/self-check.log"
grep -q "(linux/$ARCH)" "$TEMP/self-check.log"
[[ ! -e "$TEMP/home/codex/auth.json" && ! -e "$TEMP/home/grok/auth.json" ]]
printf 'Native Linux package and bundled SQLite verified: %s\n' "$ARCH"
