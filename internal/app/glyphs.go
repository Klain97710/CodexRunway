package app

import "github.com/egoist/mygo/ui"

// The interface icons are Remix Icon's (https://remixicon.com), under the
// package's Remix Icon License v1.0 (January 2026),
// written to glyphs_gen.go by Scripts/gen-glyphs and drawn in the text color.

func remix(body string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor">` + body + `</svg>`))
}
