package app

import (
	"os"
	"strings"
	"sync"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
	"github.com/licoy/quosparr/internal/store"
)

const finderBundleID = "com.apple.finder"

// macSystemUI are the parts of the system that take the front for a moment
// without the user leaving an app.
var macSystemUI = map[string]bool{
	"com.apple.dock":                 true,
	"com.apple.Spotlight":            true,
	"com.apple.notificationcenterui": true,
	"com.apple.controlcenter":        true,
	"com.apple.systemuiserver":       true,
	"com.apple.loginwindow":          true,
	"com.apple.WindowManager":        true,
	"com.apple.screencaptureui":      true,
}

// CGWindowListOption bits.
const (
	cgWindowListOnScreenOnly           = 1 << 0
	cgWindowListExcludeDesktopElements = 1 << 4
)

var (
	cgOnce           sync.Once
	cgWindowListInfo func(option uint32, relativeTo uint32) uintptr
)

func loadCoreGraphics() bool {
	cgOnce.Do(func() {
		lib, err := purego.Dlopen("/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics", purego.RTLD_LAZY|purego.RTLD_LOCAL)
		if err == nil {
			purego.RegisterLibFunc(&cgWindowListInfo, lib, "CGWindowListCopyWindowInfo")
		}
	})
	return cgWindowListInfo != nil
}

// foregroundSupported: NSWorkspace tells the app in front without any
// permission.
func foregroundSupported() bool { return true }

func newForegroundProbe() (func() foreground, func()) {
	return func() (fg foreground) {
		waitOnMain(func() { fg = macForeground() })
		return fg
	}, func() {}
}

// macForeground is the app in front. Finder in front without a window on
// screen is the desktop.
func macForeground() foreground {
	pool := objc.ID(objc.GetClass("NSAutoreleasePool")).Send(objc.RegisterName("new"))
	defer pool.Send(objc.RegisterName("drain"))
	workspace := objc.ID(objc.GetClass("NSWorkspace")).Send(objc.RegisterName("sharedWorkspace"))
	app := workspace.Send(objc.RegisterName("frontmostApplication"))
	if app == 0 {
		return foreground{}
	}
	pid := objc.Send[int32](app, objc.RegisterName("processIdentifier"))
	id := accountNSString(app.Send(objc.RegisterName("bundleIdentifier")))
	if int(pid) == os.Getpid() || macSystemUI[id] {
		return foreground{}
	}
	if id == finderBundleID && !macHasWindowOnScreen(pid) {
		return foreground{kind: fgDesktop}
	}
	path := accountNSString(app.Send(objc.RegisterName("bundleURL")).Send(objc.RegisterName("path")))
	return foreground{kind: fgApp, keys: lowerKeys(id, path)}
}

// macHasWindowOnScreen reports whether the process has a window of the
// normal level on screen. Window owners and levels need no permission,
// unlike window titles.
func macHasWindowOnScreen(pid int32) bool {
	if !loadCoreGraphics() {
		return true
	}
	list := objc.ID(cgWindowListInfo(cgWindowListOnScreenOnly|cgWindowListExcludeDesktopElements, 0))
	if list == 0 {
		return true
	}
	defer list.Send(objc.RegisterName("release"))
	ownerKey, layerKey := nsStr("kCGWindowOwnerPID"), nsStr("kCGWindowLayer")
	for i := uint64(0); i < objc.Send[uint64](list, objc.RegisterName("count")); i++ {
		w := list.Send(objc.RegisterName("objectAtIndex:"), i)
		owner := w.Send(objc.RegisterName("objectForKey:"), ownerKey)
		layer := w.Send(objc.RegisterName("objectForKey:"), layerKey)
		if owner == 0 || layer == 0 {
			continue
		}
		if objc.Send[int32](owner, objc.RegisterName("intValue")) == pid && objc.Send[int32](layer, objc.RegisterName("intValue")) == 0 {
			return true
		}
	}
	return false
}

// floatingAppKeys: the bundle identifier, or the bundle's path.
func floatingAppKeys(a store.FloatingApp) []string { return lowerKeys(a.ID, a.Path) }

// floatingAppFromFile reads an .app bundle the user picked.
func floatingAppFromFile(path string) (store.FloatingApp, bool) {
	if !strings.HasSuffix(strings.TrimSuffix(path, "/"), ".app") {
		return store.FloatingApp{}, false
	}
	var a store.FloatingApp
	waitOnMain(func() {
		pool := objc.ID(objc.GetClass("NSAutoreleasePool")).Send(objc.RegisterName("new"))
		defer pool.Send(objc.RegisterName("drain"))
		bundle := objc.ID(objc.GetClass("NSBundle")).Send(objc.RegisterName("bundleWithPath:"), nsStr(path))
		if bundle == 0 {
			return
		}
		fm := objc.ID(objc.GetClass("NSFileManager")).Send(objc.RegisterName("defaultManager"))
		name := strings.TrimSuffix(accountNSString(fm.Send(objc.RegisterName("displayNameAtPath:"), nsStr(path))), ".app")
		a = store.FloatingApp{ID: accountNSString(bundle.Send(objc.RegisterName("bundleIdentifier"))), Name: name, Path: path}
		if a.ID == "" {
			a.ID = path
		}
	})
	return a, a.ID != ""
}
