#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SOURCE="${1:-$ROOT/Resources/Brand/quosparr-logo-1024.png}"
PNG="$ROOT/Resources/AppIcon.png"
ICNS="$ROOT/Resources/AppIcon.icns"
ICO="$ROOT/Resources/AppIcon.ico"
EMBED="$ROOT/internal/app/assets/app-icon.png"
TRAY_EMBED="$ROOT/internal/app/assets/tray-app-icon.png"

[[ -f "$SOURCE" ]] || { printf 'Missing icon source: %s\n' "$SOURCE" >&2; exit 1; }
[[ "$(sips -g pixelWidth "$SOURCE" | awk '/pixelWidth:/ { print $2 }')" == 1024 ]]
[[ "$(sips -g pixelHeight "$SOURCE" | awk '/pixelHeight:/ { print $2 }')" == 1024 ]]
[[ "$(sips -g hasAlpha "$SOURCE" | awk '/hasAlpha:/ { print $2 }')" == yes ]]

WORK="$(mktemp -d "${TMPDIR:-/tmp}/quosparr-icons.XXXXXX")"
trap 'rm -rf "$WORK"' EXIT
ICONSET="$WORK/AppIcon.iconset"
WINDOWS="$WORK/windows"
mkdir -p "$ICONSET"

while read -r name size; do
  sips -z "$size" "$size" "$SOURCE" --out "$ICONSET/$name" >/dev/null
done <<'SIZES'
icon_16x16.png 16
icon_16x16@2x.png 32
icon_32x32.png 32
icon_32x32@2x.png 64
icon_128x128.png 128
icon_128x128@2x.png 256
icon_256x256.png 256
icon_256x256@2x.png 512
icon_512x512.png 512
icon_512x512@2x.png 1024
SIZES

mkdir -p "$WINDOWS"
for size in 16 24 32 48 64 128 256; do
  sips -z "$size" "$size" "$SOURCE" --out "$WINDOWS/$size.png" >/dev/null
done
python3 - "$ICONSET" "$WINDOWS" "$WORK/AppIcon.icns" "$WORK/AppIcon.ico" <<'PY'
import pathlib
import struct
import sys

iconset, windows, icns_output, ico_output = map(pathlib.Path, sys.argv[1:])
icns_images = (
    (b"icp4", "icon_16x16.png"),
    (b"icp5", "icon_32x32.png"),
    (b"ic11", "icon_16x16@2x.png"),
    (b"ic12", "icon_32x32@2x.png"),
    (b"ic07", "icon_128x128.png"),
    (b"ic13", "icon_128x128@2x.png"),
    (b"ic08", "icon_256x256.png"),
    (b"ic14", "icon_256x256@2x.png"),
    (b"ic09", "icon_512x512.png"),
    (b"ic10", "icon_512x512@2x.png"),
)
icns_entries = []
for kind, name in icns_images:
    image = iconset.joinpath(name).read_bytes()
    icns_entries.append(kind + struct.pack(">I", len(image) + 8) + image)
icns_body = b"".join(icns_entries)
icns_output.write_bytes(b"icns" + struct.pack(">I", len(icns_body) + 8) + icns_body)

sizes = (16, 24, 32, 48, 64, 128, 256)
images = [windows.joinpath(f"{size}.png").read_bytes() for size in sizes]
offset = 6 + 16 * len(images)
entries = []
for size, image in zip(sizes, images):
    assert image[:8] == b"\x89PNG\r\n\x1a\n"
    assert struct.unpack(">II", image[16:24]) == (size, size)
    dimension = 0 if size == 256 else size
    entries.append(struct.pack("<BBBBHHII", dimension, dimension, 0, 0, 1, 32, len(image), offset))
    offset += len(image)
ico_output.write_bytes(struct.pack("<HHH", 0, 1, len(images)) + b"".join(entries) + b"".join(images))
PY

cp "$SOURCE" "$PNG"
cp "$SOURCE" "$EMBED"
cp "$WINDOWS/64.png" "$TRAY_EMBED"
cp "$WORK/AppIcon.icns" "$ICNS"
cp "$WORK/AppIcon.ico" "$ICO"
cmp "$PNG" "$EMBED"
printf 'Exported AppIcon.png, AppIcon.icns, AppIcon.ico and embedded app/tray icons\n'
