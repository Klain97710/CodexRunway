//go:build darwin

package platform

import (
	"sync"

	"github.com/ebitengine/purego"
)

const kCFStringEncodingUTF8 = 0x08000100

type coreTextFonts struct {
	copyFamilyNames      func() uintptr
	arrayGetCount        func(array uintptr) int
	arrayGetValueAtIndex func(array uintptr, index int) uintptr
	stringGetCString     func(str uintptr, buf *byte, size int, encoding uint32) uint8
	release              func(ref uintptr)
}

var (
	ctFontsOnce sync.Once
	ctFonts     *coreTextFonts
)

func loadCoreTextFonts() *coreTextFonts {
	ctFontsOnce.Do(func() {
		cf, err := purego.Dlopen("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation", purego.RTLD_LAZY|purego.RTLD_GLOBAL)
		if err != nil {
			return
		}
		ct, err := purego.Dlopen("/System/Library/Frameworks/CoreText.framework/CoreText", purego.RTLD_LAZY|purego.RTLD_GLOBAL)
		if err != nil {
			return
		}
		f := &coreTextFonts{}
		purego.RegisterLibFunc(&f.copyFamilyNames, ct, "CTFontManagerCopyAvailableFontFamilyNames")
		purego.RegisterLibFunc(&f.arrayGetCount, cf, "CFArrayGetCount")
		purego.RegisterLibFunc(&f.arrayGetValueAtIndex, cf, "CFArrayGetValueAtIndex")
		purego.RegisterLibFunc(&f.stringGetCString, cf, "CFStringGetCString")
		purego.RegisterLibFunc(&f.release, cf, "CFRelease")
		ctFonts = f
	})
	return ctFonts
}

// systemFontFamilies asks CoreText for the visible families, the names
// kCTFontFamilyNameAttribute matches.
func systemFontFamilies() []string {
	f := loadCoreTextFonts()
	if f == nil {
		return nil
	}
	arr := f.copyFamilyNames()
	if arr == 0 {
		return nil
	}
	defer f.release(arr)
	count := f.arrayGetCount(arr)
	out := make([]string, 0, count)
	buf := make([]byte, 1024)
	for i := range count {
		str := f.arrayGetValueAtIndex(arr, i)
		if str == 0 || f.stringGetCString(str, &buf[0], len(buf), kCFStringEncodingUTF8) == 0 {
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
