#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
source "$ROOT/Scripts/codesign-helpers.sh"
ARCH="${ARCH:-$(uname -m)}"
case "$ARCH" in
  arm64) GOARCH=arm64 ;;
  x86_64) GOARCH=amd64 ;;
  *) printf 'Unsupported architecture: %s\n' "$ARCH" >&2; exit 2 ;;
esac
PROFILE="${RUNWAY_BUILD_PROFILE:-migration-dev}"
case "$PROFILE" in
  migration-dev) DEVELOPMENT=1; APP_NAME='CodexRunway Migration'; SNAPSHOT_DIR=.codex-runway-migration; WIDGET_FLAGS=RUNWAY_MIGRATION_DEV ;;
  replacement-candidate) DEVELOPMENT=0; APP_NAME=CodexRunway; SNAPSHOT_DIR=.codex-runway; WIDGET_FLAGS= ;;
  *) printf 'Invalid RUNWAY_BUILD_PROFILE\n' >&2; exit 2 ;;
esac
[[ "${V1_DEVELOPMENT_PACKAGE:-$DEVELOPMENT}" == "$DEVELOPMENT" ]] || { printf 'Conflicting build profile\n' >&2; exit 2; }
export RUNWAY_BUILD_PROFILE="$PROFILE"
case "$DEVELOPMENT" in 0) SUFFIX= ;; 1) SUFFIX=-dev ;; *) printf 'V1_DEVELOPMENT_PACKAGE must be 0 or 1\n' >&2; exit 2 ;; esac
WORK="${V1_BUILD_DIR:-$ROOT/build/$PROFILE-$ARCH}"
APP="$WORK/darwin-$GOARCH/$APP_NAME.app"
CONTENTS="$APP/Contents"
WIDGET="$CONTENTS/PlugIns/QuosparrWidget.appex"
BRIDGE="$CONTENTS/Frameworks/libQuosparrWidgetBridge.dylib"
export GOCACHE="${GOCACHE:-$ROOT/.build/v1-go-cache}"
mkdir -p "$WORK"

# Pin the CLI to the same version as go.mod; it supplies the Go executable,
# MyGo native runtime and the SQLite plugin's checksum-verified resource.
cd "$ROOT"
mkdir -p "$ROOT/.build"
CONFIG_STAGE="$(mktemp -d "$ROOT/.build/v1-package-config.XXXXXX")"
trap 'rm -rf "$CONFIG_STAGE"' EXIT
[[ "$(go list -m -f '{{.Version}}' github.com/egoist/mygo)" == v0.5.5 ]] || { printf 'MyGo v0.5.5 is required\n' >&2; exit 1; }
if [[ "$DEVELOPMENT" == 0 ]]; then
  export MYGO_UPDATER_PRIVATE_KEY="${MYGO_UPDATE_PRIVATE_KEY:-${MYGO_UPDATER_PRIVATE_KEY:-}}"
  [[ -n "$MYGO_UPDATER_PRIVATE_KEY" ]] || { printf 'Missing MYGO_UPDATE_PRIVATE_KEY\n' >&2; exit 1; }
  go run ./Scripts/release-assets check-keys -config "${V1_CONFIG_PATH:-$ROOT/mygo.json}"
fi
V1_DEVELOPMENT_PACKAGE="$DEVELOPMENT" python3 "$ROOT/Scripts/stage-v1-config.py" "$ROOT" "$CONFIG_STAGE"
mkdir -p "$WORK/config"
cp "$CONFIG_STAGE/mygo.json" "$WORK/config/mygo.json"
if [[ "$DEVELOPMENT" == 1 ]]; then
  export GOFLAGS="${GOFLAGS:+$GOFLAGS }-tags=quosparr_dev_package"
  printf 'Development review package: updater disabled, no update manifest is generated\n'
else
  export GOFLAGS="${GOFLAGS:+$GOFLAGS }-tags=personal_replacement"
fi
go run github.com/egoist/mygo/cmd/mygo build \
  -platform "darwin/$GOARCH" -o "$WORK" -skip-dmg -skip-notarize -sign - "$CONFIG_STAGE"
# The CLI archive predates the Widget and final signatures; never ship it
rm -f "$WORK/darwin-$GOARCH"/update-*.json "$WORK/darwin-$GOARCH"/*.tar.gz
[[ -d "$APP" ]] || { printf 'Missing MyGo bundle: %s\n' "$APP" >&2; exit 1; }
go version -m "$CONTENTS/MacOS/$APP_NAME" | python3 -c 'import sys; tags=[line.strip().split("=",1)[1] for line in sys.stdin if line.strip().startswith("build\t-tags=")]; assert any("quosparr_dev_package" in value.split(",") for value in tags) == (sys.argv[1] == "1"), "packaged update mode mismatch"' "$DEVELOPMENT"
mkdir -p "$CONTENTS/Frameworks" "$CONTENTS/PlugIns"
xcrun swiftc -swift-version 5 -O -emit-library \
  -module-cache-path "$WORK/bridge-module-cache" \
  -target "$ARCH-apple-macosx12.0" -module-name QuosparrWidgetBridge \
  -Xlinker -install_name -Xlinker @rpath/libQuosparrWidgetBridge.dylib \
  "$ROOT/Sources/QuosparrWidgetBridge/WidgetBridge.swift" -o "$BRIDGE"

VERSION="$(plutil -extract CFBundleShortVersionString raw "$CONTENTS/Info.plist")"
BUILD="$(plutil -extract CFBundleVersion raw "$CONTENTS/Info.plist")"
HOST_ID="$(plutil -extract CFBundleIdentifier raw "$CONTENTS/Info.plist")"
CLANG_MODULE_CACHE_PATH="$WORK/widget-module-cache" \
SWIFTPM_MODULECACHE_OVERRIDE="$WORK/widget-module-cache" \
xcodebuild -project "$ROOT/WidgetExtension/QuosparrWidget.xcodeproj" \
  -scheme QuosparrWidget -configuration Release \
  -derivedDataPath "$WORK/widget-derived" \
  -clonedSourcePackagesDirPath "$ROOT/.build/v1-widget-packages" \
  -packageCachePath "$WORK/package-cache" -skipPackageUpdates \
  RUNWAY_WIDGET_BUNDLE_ID="$HOST_ID.widget" \
  RUNWAY_APP_GROUP_ID="group.$HOST_ID" \
  RUNWAY_WIDGET_DISPLAY_NAME="$APP_NAME" RUNWAY_WIDGET_STORAGE_MODE=local \
  MARKETING_VERSION="$VERSION" CURRENT_PROJECT_VERSION="$BUILD" \
  SWIFT_ACTIVE_COMPILATION_CONDITIONS="$WIDGET_FLAGS" CODE_SIGNING_ALLOWED=NO ARCHS="$ARCH" ONLY_ACTIVE_ARCH=YES \
  ENABLE_APP_SANDBOX=NO CODE_SIGN_ENTITLEMENTS= \
  CLANG_MODULE_CACHE_PATH="$WORK/widget-module-cache" \
  SWIFT_MODULE_CACHE_PATH="$WORK/widget-module-cache" build
ditto "$WORK/widget-derived/Build/Products/Release/QuosparrWidget.appex" "$WIDGET"
for lproj in "$ROOT/Resources"/*.lproj; do
  ditto "$lproj" "$CONTENTS/Resources/$(basename "$lproj")"
  if [[ "$DEVELOPMENT" == 1 ]]; then
    python3 - "$CONTENTS/Resources/$(basename "$lproj")/InfoPlist.strings" <<'PYDEV'
from pathlib import Path
import sys
p=Path(sys.argv[1]); p.write_text(p.read_text().replace("CodexRunway", "CodexRunway Migration"))
PYDEV
  fi
done
plutil -replace RunwayWidgetStorageMode -string local "$CONTENTS/Info.plist"
plutil -replace NSAppDataUsageDescription -string \
  'CodexRunway reads local Codex configuration to show quota, reset credits, and API-equivalent cost.' \
  "$CONTENTS/Info.plist"

# Sign only after every resource is embedded, inside out, then archive that
# final bundle. The app updates through mygo and never embeds Sparkle.
[[ ! -e "$CONTENTS/Frameworks/Sparkle.framework" ]] || { printf 'Unexpected Sparkle.framework\n' >&2; exit 1; }
APP_ENTITLEMENTS="$WORK/app-entitlements.plist"
WIDGET_ENTITLEMENTS="$WORK/widget-entitlements.plist"
rm -f "$APP_ENTITLEMENTS" "$WIDGET_ENTITLEMENTS"
plutil -create xml1 "$APP_ENTITLEMENTS"
/usr/libexec/PlistBuddy -c 'Add :com.apple.security.cs.disable-library-validation bool true' "$APP_ENTITLEMENTS"
plutil -create xml1 "$WIDGET_ENTITLEMENTS"
/usr/libexec/PlistBuddy \
  -c 'Add :com.apple.security.app-sandbox bool true' \
  -c 'Add :com.apple.security.temporary-exception.files.home-relative-path.read-only array' \
  -c "Add :com.apple.security.temporary-exception.files.home-relative-path.read-only:0 string /$SNAPSHOT_DIR/widget-snapshot.json" \
  "$WIDGET_ENTITLEMENTS"
runway_codesign "$BRIDGE" "$HOST_ID.widget-bridge"
runway_codesign "$CONTENTS/Resources/libmygo-sqlite3.dylib" "$HOST_ID.sqlite"
runway_codesign "$WIDGET" "$HOST_ID.widget" "$WIDGET_ENTITLEMENTS"
runway_codesign "$APP" "$HOST_ID" "$APP_ENTITLEMENTS"
bash "$ROOT/Scripts/verify-v1-app.sh" "$APP" "$ARCH" "$WORK/config/mygo.json"
ZIP="$WORK/CodexRunway-macos-$ARCH$SUFFIX.zip"
rm -f "$ZIP"
ditto -c -k --keepParent "$APP" "$ZIP"
printf 'Packaged v1: %s\nArchive: %s\n' "$APP" "$ZIP"
if [[ "$DEVELOPMENT" == 0 ]]; then
  TAR="$WORK/codex-runway-$VERSION-darwin-$GOARCH.tar.gz"
  COPYFILE_DISABLE=1 tar -czf "$TAR" -C "$(dirname "$APP")" "$APP_NAME.app"
  go run ./Scripts/release-assets sign -config "$WORK/config/mygo.json" -dir "$WORK" -targets "darwin-$GOARCH"
  DMG_ROOT="$WORK/dmg-root"
  rm -rf "$DMG_ROOT"
  mkdir -p "$DMG_ROOT"
  ditto "$APP" "$DMG_ROOT/$APP_NAME.app"
  ln -s /Applications "$DMG_ROOT/Applications"
  hdiutil create -volname "$APP_NAME" -srcfolder "$DMG_ROOT" -ov -format UDZO "$WORK/CodexRunway-macos-$ARCH.dmg" >/dev/null
  rm -rf "$DMG_ROOT"
  printf 'Final mygo archive and DMG generated from the verified bundle\n'
fi
