package platform

import (
	"slices"
	"strings"
)

// FontFamilies returns the font families installed on the system, sorted
// and without duplicates, by the names the text system finds them by. It
// leaves out hidden families (".SF NS") and vertical ones ("@MS Gothic").
// It is nil where the system cannot be asked.
func FontFamilies() []string { return tidyFamilies(systemFontFamilies()) }

func tidyFamilies(names []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		key := strings.ToLower(name)
		if name == "" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "@") || strings.Contains(name, ",") || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, name)
	}
	slices.SortFunc(out, func(a, b string) int {
		if c := strings.Compare(strings.ToLower(a), strings.ToLower(b)); c != 0 {
			return c
		}
		return strings.Compare(a, b)
	})
	return out
}
