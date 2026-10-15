//go:build windows

package platform

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var procDWriteCreateFactory = windows.NewLazySystemDLL("dwrite.dll").NewProc("DWriteCreateFactory")

// IID_IDWriteFactory.
var iidIDWriteFactory = windows.GUID{Data1: 0xb859ee5a, Data2: 0xd838, Data3: 0x4b5b, Data4: [8]byte{0xa2, 0xe8, 0x1a, 0xdc, 0x7d, 0x93, 0xdb, 0x48}}

// Method indexes in the COM vtables.
const (
	methodRelease                          = 2
	factoryGetSystemFontCollection         = 3 // IDWriteFactory
	collectionGetFontFamilyCount           = 3 // IDWriteFontCollection
	collectionGetFontFamily                = 4
	familyGetFamilyNames                   = 6 // IDWriteFontFamily
	stringsFindLocaleName                  = 4 // IDWriteLocalizedStrings
	stringsGetStringLength                 = 7
	stringsGetString                       = 8
	dwriteFactoryTypeShared        uintptr = 0
)

//go:uintptrescapes
func comCall(obj uintptr, i int, args ...uintptr) uintptr {
	vtbl := *(*uintptr)(foreignPointer(obj))
	fn := *(*uintptr)(unsafe.Add(foreignPointer(vtbl), i*int(unsafe.Sizeof(uintptr(0)))))
	r, _, _ := syscall.SyscallN(fn, append([]uintptr{obj}, args...)...)
	return r
}

// foreignPointer is the address of memory DirectWrite owns, which the Go
// garbage collector never moves.
func foreignPointer(p uintptr) unsafe.Pointer { return *(*unsafe.Pointer)(unsafe.Pointer(&p)) }

func comFailed(hr uintptr) bool { return int32(uint32(hr)) < 0 }

func comRelease(obj uintptr) {
	if obj != 0 {
		comCall(obj, methodRelease)
	}
}

// systemFontFamilies asks DirectWrite, which the text system uses, for the
// families of the system font collection, by their English names, which it
// finds them by in every UI language.
func systemFontFamilies() []string {
	if procDWriteCreateFactory.Find() != nil {
		return nil
	}
	var factory, coll uintptr
	hr, _, _ := procDWriteCreateFactory.Call(dwriteFactoryTypeShared, uintptr(unsafe.Pointer(&iidIDWriteFactory)), uintptr(unsafe.Pointer(&factory)))
	if comFailed(hr) || factory == 0 {
		return nil
	}
	defer comRelease(factory)
	if comFailed(comCall(factory, factoryGetSystemFontCollection, uintptr(unsafe.Pointer(&coll)), 0)) || coll == 0 {
		return nil
	}
	defer comRelease(coll)
	n := uint32(comCall(coll, collectionGetFontFamilyCount))
	out := make([]string, 0, n)
	for i := range n {
		var fam, strs uintptr
		if comFailed(comCall(coll, collectionGetFontFamily, uintptr(i), uintptr(unsafe.Pointer(&fam)))) {
			continue
		}
		if !comFailed(comCall(fam, familyGetFamilyNames, uintptr(unsafe.Pointer(&strs)))) {
			if name := englishFamilyName(strs); name != "" {
				out = append(out, name)
			}
			comRelease(strs)
		}
		comRelease(fam)
	}
	return out
}

// englishFamilyName returns the English name of localized strings, or
// their first.
func englishFamilyName(strs uintptr) string {
	var index uint32
	var exists int32
	en, _ := windows.UTF16PtrFromString("en-us")
	comCall(strs, stringsFindLocaleName, uintptr(unsafe.Pointer(en)), uintptr(unsafe.Pointer(&index)), uintptr(unsafe.Pointer(&exists)))
	if exists == 0 {
		index = 0
	}
	var length uint32
	if comFailed(comCall(strs, stringsGetStringLength, uintptr(index), uintptr(unsafe.Pointer(&length)))) {
		return ""
	}
	buf := make([]uint16, length+1)
	if comFailed(comCall(strs, stringsGetString, uintptr(index), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))) {
		return ""
	}
	return windows.UTF16ToString(buf)
}
