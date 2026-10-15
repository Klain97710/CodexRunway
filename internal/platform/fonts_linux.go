//go:build linux

package platform

import (
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

type pangoFonts struct {
	defaultFontMap func() uintptr
	listFamilies   func(fontMap uintptr, families **uintptr, n *int32)
	familyName     func(family uintptr) *byte
	free           func(p unsafe.Pointer)
}

var (
	pangoFontsOnce sync.Once
	pangoFontsLib  *pangoFonts
)

func loadPangoFonts() *pangoFonts {
	pangoFontsOnce.Do(func() {
		open := func(name string) uintptr {
			h, _ := purego.Dlopen(name, purego.RTLD_NOW|purego.RTLD_GLOBAL)
			return h
		}
		glib, pango, pangocairo := open("libglib-2.0.so.0"), open("libpango-1.0.so.0"), open("libpangocairo-1.0.so.0")
		if glib == 0 || pango == 0 || pangocairo == 0 {
			return
		}
		p := &pangoFonts{}
		purego.RegisterLibFunc(&p.defaultFontMap, pangocairo, "pango_cairo_font_map_get_default")
		purego.RegisterLibFunc(&p.listFamilies, pango, "pango_font_map_list_families")
		purego.RegisterLibFunc(&p.familyName, pango, "pango_font_family_get_name")
		purego.RegisterLibFunc(&p.free, glib, "g_free")
		pangoFontsLib = p
	})
	return pangoFontsLib
}

// systemFontFamilies asks Pango, which the text system uses, for the
// families fontconfig knows. Pango's default font map belongs to the
// calling thread; the families stay the map's, the array is ours.
func systemFontFamilies() []string {
	p := loadPangoFonts()
	if p == nil {
		return nil
	}
	fontMap := p.defaultFontMap()
	if fontMap == 0 {
		return nil
	}
	var families *uintptr
	var n int32
	p.listFamilies(fontMap, &families, &n)
	if families == nil {
		return nil
	}
	defer p.free(unsafe.Pointer(families))
	out := make([]string, 0, n)
	for _, fam := range unsafe.Slice(families, n) {
		if name := cStringAt(p.familyName(fam)); name != "" {
			out = append(out, name)
		}
	}
	return out
}

func cStringAt(s *byte) string {
	if s == nil {
		return ""
	}
	n := 0
	for *(*byte)(unsafe.Add(unsafe.Pointer(s), n)) != 0 {
		n++
	}
	return string(unsafe.Slice(s, n))
}
