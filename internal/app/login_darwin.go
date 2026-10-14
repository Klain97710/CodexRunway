//go:build darwin

package app

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

// SMAppService.Status values.
const (
	smNotRegistered    = 0
	smEnabled          = 1
	smRequiresApproval = 2
	smNotFound         = 3
)

var smOnce sync.Once

// nativeLoginStatus reads SMAppService.mainApp.status (macOS 13+), the same
// call as Swift's SystemLoginItemBackend, to tell a login item that waits
// for approval from an enabled one: mygo's OpenAtLogin reports both as
// "off"/"on" only. It reports false where there is no such service (macOS 12)
// or the process is not an app bundle. On macOS 12 the production backend
// migrates Swift's session login item before checking mygo's launch agent.
func nativeLoginStatus() (LoginStatus, bool) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	cls := nativeLoginClass()
	if cls == 0 {
		return 0, false
	}
	pool := objc.ID(objc.GetClass("NSAutoreleasePool")).Send(objc.RegisterName("new"))
	defer pool.Send(objc.RegisterName("drain"))
	svc := objc.ID(cls).Send(objc.RegisterName("mainAppService"))
	if svc == 0 {
		return 0, false
	}
	switch objc.Send[int](svc, objc.RegisterName("status")) {
	case smEnabled:
		return LoginEnabled, true
	case smRequiresApproval:
		return LoginRequiresApproval, true
	case smNotRegistered, smNotFound:
		return LoginNotRegistered, true
	}
	return 0, false
}

func nativeLoginClass() objc.Class {
	exe, err := os.Executable()
	if err != nil || !inAppBundle(exe) {
		return 0 // SMAppService.mainApp needs a bundle
	}
	smOnce.Do(func() {
		_, _ = purego.Dlopen("/System/Library/Frameworks/ServiceManagement.framework/ServiceManagement", purego.RTLD_GLOBAL|purego.RTLD_NOW)
	})
	return objc.GetClass("SMAppService")
}

func nativeLoginAvailable() bool { return nativeLoginClass() != 0 }

// SMAppService has no main-thread requirement. Each operation obtains its
// own service inside a thread-bound autorelease pool; no Objective-C object
// is shared across workers. MyGo's wrapper dispatches to the UI thread.
func nativeLoginSet(on bool) (error, bool) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	cls := nativeLoginClass()
	if cls == 0 {
		return nil, false
	}
	pool := objc.ID(objc.GetClass("NSAutoreleasePool")).Send(objc.RegisterName("new"))
	defer pool.Send(objc.RegisterName("drain"))
	svc := objc.ID(cls).Send(objc.RegisterName("mainAppService"))
	if svc == 0 {
		return fmt.Errorf("SMAppService.mainAppService is unavailable"), true
	}
	selector := "unregisterAndReturnError:"
	if on {
		selector = "registerAndReturnError:"
	}
	var nativeErr objc.ID
	if !objc.Send[bool](svc, objc.RegisterName(selector), uintptr(unsafe.Pointer(&nativeErr))) {
		message := "SMAppService rejected the login change"
		if nativeErr != 0 {
			description := nativeErr.Send(objc.RegisterName("localizedDescription"))
			message = objc.Send[string](description, objc.RegisterName("UTF8String"))
		}
		return fmt.Errorf("%s", message), true
	}
	return nil, true
}
