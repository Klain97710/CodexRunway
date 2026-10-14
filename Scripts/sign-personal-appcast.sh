#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
MODE="${1:?sign or verify required}"
FEED="${2:?appcast required}"
[[ "$MODE" == sign || "$MODE" == verify ]] || exit 2
TOOL="$(bash "$ROOT/Scripts/prepare-personal-sparkle-tools.sh")"
KEY="$(mktemp "${TMPDIR:-/tmp}/personal-sparkle-key.XXXXXX")"
trap 'rm -f "$KEY"' EXIT
chmod 600 "$KEY"
python3 - "$KEY" <<'PY'
import os,sys,pathlib
key=os.environ.get('SPARKLE_PRIVATE_KEY','').strip()
if not key: raise SystemExit('Missing SPARKLE_PRIVATE_KEY')
pathlib.Path(sys.argv[1]).write_text(key)
PY
if [[ "$MODE" == sign ]]; then "$TOOL" --ed-key-file "$KEY" "$FEED"; fi
"$TOOL" --verify --ed-key-file "$KEY" "$FEED"
