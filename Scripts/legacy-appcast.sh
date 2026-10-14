#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
export GOCACHE="${GOCACHE:-$ROOT/.build/v1-go-cache}"
DIST="${1:?asset directory required}"
CONFIG="${2:-$ROOT/mygo.json}"
TAG="${3:?release tag required}"
[[ -n "${SPARKLE_PUBLIC_KEY:-}" && -n "${SPARKLE_PRIVATE_KEY:-}" ]] || { printf 'Missing Sparkle signing secrets\n' >&2; exit 1; }
python3 - "$CONFIG" <<'PY'
import json,os,sys
expected=json.load(open(sys.argv[1]))["macos"]["infoPlist"]["SUPublicEDKey"]
if os.environ["SPARKLE_PUBLIC_KEY"].strip()!=expected:
 raise SystemExit("SPARKLE_PUBLIC_KEY must match the existing SUPublicEDKey")
PY
cd "$ROOT"
IFS=',' read -r -a ARCHES <<< "${4:-arm64,x86_64}"
for arch in "${ARCHES[@]}"; do
  case "$arch" in arm64|x86_64) ;; *) exit 2 ;; esac
  # The Swift releases share the bundle ID, so their Sparkle feed installs
  # the new archive; release-assets signs it as generate_appcast would.
  ZIP="Quosparr-macos-$arch.zip"
  go run ./Scripts/release-assets sparkle-appcast -config "$CONFIG" -file "$DIST/$ZIP" \
    -url "https://github.com/Licoy/Quosparr/releases/download/$TAG/$ZIP" \
    -out "$DIST/quosparr-appcast-$arch.xml"
  # A different bundle ID cannot be installed over the previous app by Sparkle.
  # Its original feed offers the manual download page, never the new archive.
  python3 "$ROOT/Scripts/legacy-install-notice.py" "$CONFIG" "$DIST/appcast-$arch.xml" "$TAG"
done
python3 "$ROOT/Scripts/verify-legacy-appcast.py" "$DIST" "$CONFIG" "$TAG" "${4:-arm64,x86_64}"
