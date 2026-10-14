#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
export GOCACHE="${GOCACHE:-$ROOT/.build/v1-go-cache}"
DIST="${1:?asset directory required}"
CONFIG="${2:-$ROOT/mygo.json}"
TAG="${3:?release tag required}"
cd "$ROOT"
go run ./Scripts/release-assets check-keys -config "$CONFIG"
python3 - "$CONFIG" "$TAG" <<'PY'
import json,sys
cfg=json.load(open(sys.argv[1]))
if sys.argv[2] != 'personal-v'+cfg['version']: raise SystemExit('Personal release tag mismatch')
PY
IFS=',' read -r -a ARCHES <<< "${4:-arm64,x86_64}"
for arch in "${ARCHES[@]}"; do
  case "$arch" in arm64|x86_64) ;; *) exit 2 ;; esac
  ZIP="CodexRunway-macos-$arch.zip"
  go run ./Scripts/release-assets sparkle-appcast -config "$CONFIG" -file "$DIST/$ZIP" \
    -url "https://github.com/Klain97710/CodexRunway/releases/download/$TAG/$ZIP" \
    -out "$DIST/appcast-$arch.xml"
  bash "$ROOT/Scripts/sign-personal-appcast.sh" sign "$DIST/appcast-$arch.xml"
done
python3 "$ROOT/Scripts/verify-legacy-appcast.py" "$DIST" "$CONFIG" "$TAG" "${4:-arm64,x86_64}"
