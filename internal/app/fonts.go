package app

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

// installedFontFamilies lists the font families the display font can be,
// by the names the text system takes, sorted and each once as mygo lists
// them.
func installedFontFamilies() []string {
	families := ui.FontFamilies()
	names := make([]string, 0, len(families))
	for _, f := range families {
		names = append(names, f.Name)
	}
	return offeredFontFamilies(names)
}

// offeredFontFamilies leaves out the names a font setting does not offer:
// hidden families (".SF NS"), vertical ones ("@MS Gothic") and names that
// read as a list of families.
func offeredFontFamilies(names []string) []string {
	out := make([]string, 0, len(names))
	for _, name := range names {
		if name == "" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "@") || strings.Contains(name, ",") {
			continue
		}
		out = append(out, name)
	}
	return out
}
