#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
ARCH="${2:-$(uname -m)}"
case "$ARCH" in arm64) GOARCH=arm64 ;; x86_64) GOARCH=amd64 ;; *) exit 2 ;; esac
APP="${1:-$ROOT/build/v1-$ARCH/darwin-$GOARCH/Quosparr.app}"
CONFIG="${3:-$ROOT/mygo.json}"
CONTENTS="$APP/Contents"
WIDGET="$CONTENTS/PlugIns/QuosparrWidget.appex"
BRIDGE="$CONTENTS/Frameworks/libQuosparrWidgetBridge.dylib"
SQLITE="$CONTENTS/Resources/libmygo-sqlite3.dylib"
bash "$ROOT/Scripts/verify-packaged-app.sh" "$APP" "$ARCH" local
[[ "$(plutil -extract CFBundleIdentifier raw "$CONTENTS/Info.plist")" == com.quosparr.app ]]
[[ "$(plutil -extract LSUIElement raw "$CONTENTS/Info.plist")" == true ]]
[[ "$(plutil -extract LSMinimumSystemVersion raw "$CONTENTS/Info.plist")" == 12.0 ]]
[[ "$(plutil -extract LSMinimumSystemVersion raw "$WIDGET/Contents/Info.plist")" == 14.0 ]]
[[ "$(plutil -extract RunwayWidgetStorageMode raw "$CONTENTS/Info.plist")" == local ]]
[[ ! -e "$CONTENTS/Frameworks/Sparkle.framework" ]]
for file in "$CONTENTS/MacOS/Quosparr" "$BRIDGE" "$SQLITE"; do
  [[ -f "$file" ]]
  [[ " $(lipo -archs "$file") " == *" $ARCH "* ]]
  codesign --verify --strict "$file"
done
go version -m "$CONTENTS/MacOS/Quosparr" | grep -Fq 'github.com/licoy/quosparr'
nm -gU "$BRIDGE" | grep -Fq _quosparr_widget_query
nm -gU "$BRIDGE" | grep -Fq _quosparr_widget_reload
HOST_ENTITLEMENTS="$(codesign -d --entitlements :- "$APP" 2>/dev/null)"
WIDGET_ENTITLEMENTS="$(codesign -d --entitlements :- "$WIDGET" 2>/dev/null)"
printf '%s' "$HOST_ENTITLEMENTS" | python3 -c 'import plistlib,sys; assert plistlib.loads(sys.stdin.buffer.read())["com.apple.security.cs.disable-library-validation"] is True'
printf '%s' "$WIDGET_ENTITLEMENTS" | python3 -c 'import plistlib,sys; assert plistlib.loads(sys.stdin.buffer.read())["com.apple.security.app-sandbox"] is True'
printf '%s' "$WIDGET_ENTITLEMENTS" | python3 -c 'import plistlib,sys; assert plistlib.loads(sys.stdin.buffer.read())["com.apple.security.temporary-exception.files.home-relative-path.read-only"] == ["/.quosparr/widget-snapshot.json"]'
printf 'Verified Go v1 host, WidgetKit bridge, SQLite, local snapshot permission and final signatures (%s)\n' "$ARCH"
python3 - "$CONTENTS/Info.plist" "$CONFIG" "$CONTENTS/Resources/AppIcon.icns" <<'PYVERIFY'
import json,pathlib,plistlib,struct,sys
info=plistlib.loads(pathlib.Path(sys.argv[1]).read_bytes())
config=json.loads(pathlib.Path(sys.argv[2]).read_text())
assert info["CFBundleShortVersionString"] == config["version"], "host version mismatch"
assert info["CFBundleVersion"] == str(config["macos"]["infoPlist"]["CFBundleVersion"]), "build mismatch"
assert info["SUPublicEDKey"] == config["macos"]["infoPlist"]["SUPublicEDKey"], "Sparkle key changed"
assert int(info["CFBundleVersion"]) >= 100
icon=pathlib.Path(sys.argv[3]).read_bytes()
extra_licenses={"LOBEHUB-LICENSE","REMIXICON-LICENSE","TOML-LICENSE"}
assert not any(p.name in extra_licenses for p in pathlib.Path(sys.argv[3]).parent.rglob("*")), "source-only licenses copied into app"
assert icon[:4] == b"icns"
offset=8
found=False
while offset<len(icon):
 kind,size=struct.unpack(">4sI",icon[offset:offset+8])
 assert size>=8 and offset+size<=len(icon)
 if kind==b"ic10":
  png=icon[offset+8:offset+size]
  assert png[:8]==b"\x89PNG\r\n\x1a\n" and struct.unpack(">II",png[16:24])==(1024,1024)
  found=True
 offset+=size
assert found, "icns has no 1024px image"
PYVERIFY
