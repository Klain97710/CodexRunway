//go:build darwin

package app

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

type sessionLoginItems struct{ appPath string }

type sharedLoginAPI struct {
	listType uintptr
	create   func(uintptr, uintptr, uintptr) uintptr
	snapshot func(uintptr, uintptr) uintptr
	count    func(uintptr) int
	value    func(uintptr, int) uintptr
	resolve  func(uintptr, uint32, uintptr) uintptr
	remove   func(uintptr, uintptr) int32
	release  func(uintptr)
	path     func(uintptr) string
}

var sharedLogin struct {
	sync.Once
	api sharedLoginAPI
	err error
}

func legacyMigratingLoginBackend(exe string, current LoginBackend) LoginBackend {
	if !inAppBundle(exe) {
		return current
	}
	end := strings.LastIndex(exe, ".app/Contents/MacOS/") + len(".app")
	path := exe[:end]
	if nativeLoginAvailable() {
		path = ""
	} // Never remove the newly registered SMAppService item.
	return &migratingLoginBackend{current: current, legacy: macLegacyLogin{session: sessionLoginItems{appPath: path}}}
}

func loadSharedLoginAPI() (*sharedLoginAPI, error) {
	sharedLogin.Do(func() {
		h, err := purego.Dlopen("/System/Library/Frameworks/CoreServices.framework/CoreServices", purego.RTLD_GLOBAL|purego.RTLD_NOW)
		if err != nil {
			sharedLogin.err = err
			return
		}
		symbol, err := purego.Dlsym(h, "kLSSharedFileListSessionLoginItems")
		if err != nil {
			sharedLogin.err = err
			return
		}
		// Dlsym returns the address of the exported CFStringRef constant,
		// using the same native-address conversion as the proxy/keychain code.
		sharedLogin.api.listType = **(**uintptr)(unsafe.Pointer(&symbol))
		sharedLogin.api.path = func(url uintptr) string { return resolvedLoginAppPath(objc.ID(url)) }
		for _, binding := range []struct {
			fn   any
			name string
		}{
			{&sharedLogin.api.create, "LSSharedFileListCreate"},
			{&sharedLogin.api.snapshot, "LSSharedFileListCopySnapshot"},
			{&sharedLogin.api.count, "CFArrayGetCount"},
			{&sharedLogin.api.value, "CFArrayGetValueAtIndex"},
			{&sharedLogin.api.resolve, "LSSharedFileListItemCopyResolvedURL"},
			{&sharedLogin.api.remove, "LSSharedFileListItemRemove"},
			{&sharedLogin.api.release, "CFRelease"},
		} {
			address, err := purego.Dlsym(h, binding.name)
			if err != nil {
				sharedLogin.err = err
				return
			}
			purego.RegisterFunc(binding.fn, address)
		}
	})
	return &sharedLogin.api, sharedLogin.err
}

func (s sessionLoginItems) State() (legacyLoginState, error) {
	found, err := s.visit(false)
	return legacyLoginState{Exists: found, Enabled: found}, err
}

func (s sessionLoginItems) Remove() error {
	_, err := s.visit(true)
	return err
}

// Match resolved bundle paths with no interaction or volume mounting, as
// Swift's SessionLoginItems does. Remove every matching record if older
// installations left duplicates; never remove another application's item.
func (s sessionLoginItems) visit(remove bool) (bool, error) {
	api, err := loadSharedLoginAPI()
	if err != nil {
		return false, err
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pool := objc.ID(objc.GetClass("NSAutoreleasePool")).Send(objc.RegisterName("new"))
	defer pool.Send(objc.RegisterName("drain"))
	list := api.create(0, api.listType, 0)
	if list == 0 {
		return false, fmt.Errorf("could not open the session login item list")
	}
	defer api.release(list)
	snapshot := api.snapshot(list, 0)
	if snapshot == 0 {
		return false, fmt.Errorf("could not read the session login item list")
	}
	defer api.release(snapshot)
	wanted := ""
	if s.appPath != "" {
		url := objc.ID(objc.GetClass("NSURL")).Send(objc.RegisterName("fileURLWithPath:"), nsStr(s.appPath))
		wanted = resolvedLoginAppPath(url)
		if wanted == "" {
			return false, fmt.Errorf("could not resolve the app bundle for the login item")
		}
	}
	return matchingSessionLoginItems(api, list, snapshot, wanted, remove)
}

func matchingSessionLoginItems(api *sharedLoginAPI, list, snapshot uintptr, wanted string, remove bool) (bool, error) {
	found := false
	for i := 0; i < api.count(snapshot); i++ {
		item := api.value(snapshot, i)
		// Flags are kLSSharedFileListNoUserInteraction (1) and
		// kLSSharedFileListDoNotMountVolumes (2).
		url := api.resolve(item, 3, 0)
		if url == 0 {
			continue // Swift skips items whose target no longer resolves.
		}
		path := api.path(url)
		api.release(url)
		if (wanted == "" || path != wanted) && !previousLoginBundle(path) {
			continue
		}
		found = true
		if remove {
			if status := api.remove(list, item); status != 0 {
				return found, fmt.Errorf("remove legacy login item: OSStatus %d", status)
			}
		}
	}
	return found, nil
}

func resolvedLoginAppPath(url objc.ID) string {
	resolved := url.Send(objc.RegisterName("URLByResolvingSymlinksInPath"))
	path := resolved.Send(objc.RegisterName("path"))
	return objc.Send[string](path, objc.RegisterName("UTF8String"))
}

// Read the on-disk identifier, never a display name or a cached NSBundle.
func previousLoginBundle(path string) bool {
	data, err := readLoginPlist(filepath.Join(path, "Contents", "Info.plist"))
	if err != nil {
		return false
	} // An unidentifiable entry is never removed.
	id, _ := data["CFBundleIdentifier"].(string)
	return previousLoginID(id)
}

func readLoginPlist(path string) (map[string]any, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	data, err := exec.Command("/usr/bin/plutil", "-convert", "json", "-o", "-", path).Output()
	if err != nil {
		return nil, fmt.Errorf("cannot read login item property list")
	}
	var values map[string]any
	if err := json.Unmarshal(data, &values); err != nil {
		return nil, fmt.Errorf("invalid login item property list")
	}
	return values, nil
}
