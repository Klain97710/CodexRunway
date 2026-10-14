#!/usr/bin/env bash
# Fixed release tools only; Sparkle is never linked into the Go application.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TOOLS="$ROOT/.build/personal-sparkle-2.9.3"
ARCHIVE="$TOOLS/Sparkle.zip"
mkdir -p "$TOOLS"
if [[ ! -f "$ARCHIVE" ]]; then
  curl --fail --location --proto '=https' --proto-redir '=https' \
    https://github.com/sparkle-project/Sparkle/releases/download/2.9.3/Sparkle-for-Swift-Package-Manager.zip -o "$ARCHIVE.tmp" >&2
  mv "$ARCHIVE.tmp" "$ARCHIVE"
fi
EXPECTED=3a5d7fd698acc39c122e75764ed3614b472b284cc483f32ae7006d86c513370c
[[ "$(shasum -a 256 "$ARCHIVE" | awk '{print $1}')" == "$EXPECTED" ]] || { echo 'Sparkle tools checksum mismatch' >&2; exit 1; }
ditto -x -k "$ARCHIVE" "$TOOLS/unpacked"
printf '%s\n' "$TOOLS/unpacked/bin/sign_update"
