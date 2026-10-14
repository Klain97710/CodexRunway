#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
export GOCACHE="${GOCACHE:-$ROOT/.build/v1-go-cache}"
DIST="${1:?asset directory required}"
CONFIG="${2:-$ROOT/mygo.json}"
TARGETS="${3:-darwin-arm64,darwin-amd64}"
cd "$ROOT"
python3 "$ROOT/Scripts/release-checksums.py" verify "$DIST"
go run ./Scripts/release-assets verify -config "$CONFIG" -dir "$DIST" -targets "$TARGETS"
if [[ "$TARGETS" == *darwin-* ]]; then
  VERSION="$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["version"])' "$CONFIG")"
  MAC_ARCHES=
  [[ "$TARGETS" != *darwin-arm64* ]] || MAC_ARCHES=arm64
  if [[ "$TARGETS" == *darwin-amd64* ]]; then MAC_ARCHES="${MAC_ARCHES:+$MAC_ARCHES,}x86_64"; fi
  python3 "$ROOT/Scripts/verify-legacy-appcast.py" "$DIST" "$CONFIG" "${V1_RELEASE_TAG:-personal-v$VERSION}" "$MAC_ARCHES"
fi
WORK="$(mktemp -d "${TMPDIR:-/tmp}/runway-release-verify.XXXXXX")"
MOUNT=
cleanup() {
  if [[ -n "$MOUNT" ]]; then hdiutil detach "$MOUNT" >/dev/null; fi
  rm -rf "$WORK"
}
trap cleanup EXIT
VERSION="$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["version"])' "$CONFIG")"
IFS=',' read -r -a SELECTED <<< "$TARGETS"
for target in "${SELECTED[@]}"; do
  [[ "$target" == darwin-* ]] || continue
  [[ "$(uname -s)" == Darwin ]] || { printf 'macOS signature verification requires macOS\n' >&2; exit 1; }
  ARCH="${target#darwin-}"
  [[ "$ARCH" != amd64 ]] || ARCH=x86_64
  mkdir -p "$WORK/$target/tar" "$WORK/$target/zip"
  tar -xzf "$DIST/codex-runway-$VERSION-$target.tar.gz" -C "$WORK/$target/tar"
  ditto -x -k "$DIST/CodexRunway-macos-$ARCH.zip" "$WORK/$target/zip"
  bash "$ROOT/Scripts/verify-v1-app.sh" "$WORK/$target/tar/CodexRunway.app" "$ARCH" "$CONFIG"
  codesign --verify --deep --strict "$WORK/$target/zip/CodexRunway.app"
  diff -qr "$WORK/$target/tar/CodexRunway.app" "$WORK/$target/zip/CodexRunway.app"
  hdiutil verify "$DIST/CodexRunway-macos-$ARCH.dmg" >/dev/null
  MOUNT="$WORK/$target/mount"
  mkdir -p "$MOUNT"
  hdiutil attach "$DIST/CodexRunway-macos-$ARCH.dmg" -mountpoint "$MOUNT" -readonly -nobrowse >/dev/null
  codesign --verify --deep --strict "$MOUNT/CodexRunway.app"
  diff -qr "$WORK/$target/tar/CodexRunway.app" "$MOUNT/CodexRunway.app"
  hdiutil detach "$MOUNT" >/dev/null
  MOUNT=
done
printf 'Selected release assets fully verified\n'
