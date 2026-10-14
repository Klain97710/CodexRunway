//go:build darwin

package l10n

import (
	"os"
	"sync"

	"github.com/ebitengine/purego"
)

const (
	coreFoundationPath    = "/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation"
	kCFStringEncodingUTF8 = 0x08000100
)

type coreFoundation struct {
	copyPreferredLanguages func() uintptr
	arrayGetCount          func(array uintptr) int
	arrayGetValueAtIndex   func(array uintptr, index int) uintptr
	stringGetCString       func(str uintptr, buf *byte, size int, encoding uint32) uint8
	release                func(ref uintptr)
}

var (
	cfOnce sync.Once
	cf     *coreFoundation
)

func loadCoreFoundation() *coreFoundation {
	cfOnce.Do(func() {
		lib, err := purego.Dlopen(coreFoundationPath, purego.RTLD_LAZY|purego.RTLD_GLOBAL)
		if err != nil {
			return
		}
		c := &coreFoundation{}
		purego.RegisterLibFunc(&c.copyPreferredLanguages, lib, "CFLocaleCopyPreferredLanguages")
		purego.RegisterLibFunc(&c.arrayGetCount, lib, "CFArrayGetCount")
		purego.RegisterLibFunc(&c.arrayGetValueAtIndex, lib, "CFArrayGetValueAtIndex")
		purego.RegisterLibFunc(&c.stringGetCString, lib, "CFStringGetCString")
		purego.RegisterLibFunc(&c.release, lib, "CFRelease")
		cf = c
	})
	return cf
}

// SystemPreferredLanguages returns the user's preferred languages in
// priority order (CFLocaleCopyPreferredLanguages), for example
// ["zh-Hans-CN", "en-US"]. It falls back to the LANG family of environment
// variables if CoreFoundation is unavailable.
func SystemPreferredLanguages() []string {
	if c := loadCoreFoundation(); c != nil {
		if langs := c.preferredLanguages(); len(langs) > 0 {
			return langs
		}
	}
	return languagesFromEnv(os.Getenv)
}

func (c *coreFoundation) preferredLanguages() []string {
	arr := c.copyPreferredLanguages()
	if arr == 0 {
		return nil
	}
	defer c.release(arr)
	count := c.arrayGetCount(arr)
	out := make([]string, 0, count)
	buf := make([]byte, 128)
	for i := 0; i < count; i++ {
		str := c.arrayGetValueAtIndex(arr, i)
		if str == 0 || c.stringGetCString(str, &buf[0], len(buf), kCFStringEncodingUTF8) == 0 {
			continue
		}
		n := 0
		for n < len(buf) && buf[n] != 0 {
			n++
		}
		out = append(out, string(buf[:n]))
	}
	return out
}
