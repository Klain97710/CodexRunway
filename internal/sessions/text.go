package sessions

import (
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// UnknownProject is shown for a session without a working directory.
const UnknownProject = "Unknown project"

// ProjectName is SessionProjectName.displayName: the last element of the
// working directory, the whole trimmed value when there is none.
func ProjectName(cwd *string) string {
	if cwd == nil {
		return UnknownProject
	}
	trimmed := strings.TrimSpace(*cwd)
	if trimmed == "" {
		return UnknownProject
	}
	path := strings.TrimRight(trimmed, `/\`)
	if i := strings.LastIndexAny(path, `/\`); i >= 0 {
		path = path[i+1:]
	}
	if path == "" {
		return trimmed
	}
	return path
}

const maxTitleChars = 80

// cleanTitle trims a title and cuts it at 80 characters; empty gives "".
func cleanTitle(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}
	return truncateTitle(trimmed, maxTitleChars)
}

// truncateTitle keeps the first n user-perceived characters. Swift counts
// grapheme clusters; the standard library has no segmenter, so a cluster is a
// base rune plus following combining marks, variation selectors, emoji
// modifiers, tag characters and zero-width-joiner sequences, a regional
// indicator pair, or CRLF. That covers the cases that occur in titles; a rare
// script-specific cluster (for example Hangul jamo sequences) can still count
// as several characters.
func truncateTitle(s string, n int) string {
	count, i := 0, 0
	for i < len(s) {
		if count == n {
			return s[:i]
		}
		i += clusterLen(s[i:])
		count++
	}
	return s
}

func clusterLen(s string) int {
	r, size := utf8.DecodeRuneInString(s)
	if r == '\r' && strings.HasPrefix(s[size:], "\n") {
		return size + 1
	}
	if isRegionalIndicator(r) {
		if next, nsize := utf8.DecodeRuneInString(s[size:]); isRegionalIndicator(next) {
			size += nsize
		}
	}
	for size < len(s) {
		next, nsize := utf8.DecodeRuneInString(s[size:])
		switch {
		case isExtend(next):
			size += nsize
		case next == 0x200D: // ZWJ joins the following rune into the cluster
			size += nsize
			if size < len(s) {
				_, jsize := utf8.DecodeRuneInString(s[size:])
				size += jsize
			}
		default:
			return size
		}
	}
	return size
}

func isRegionalIndicator(r rune) bool { return r >= 0x1F1E6 && r <= 0x1F1FF }

func isExtend(r rune) bool {
	switch {
	case r >= 0x1F3FB && r <= 0x1F3FF: // emoji modifiers
		return true
	case r >= 0xE0020 && r <= 0xE007F: // tag characters
		return true
	case r == 0x200C: // ZWNJ
		return true
	}
	return unicode.In(r, unicode.Mn, unicode.Me, unicode.Mc)
}

// sessionID is the id a rollout file name carries: the text after "rollout-",
// reduced to its last 36 characters (the UUID) when longer.
func sessionID(base string) string {
	name := strings.TrimSuffix(base, ".jsonl")
	rest, ok := strings.CutPrefix(name, "rollout-")
	if !ok {
		return ""
	}
	if utf8.RuneCountInString(rest) > 36 {
		runes := []rune(rest)
		rest = string(runes[len(runes)-36:])
	}
	return rest
}

// dayFromPath reads a year/month/day directory triple (sessions/2026/06/29/...)
// as local midnight.
func dayFromPath(path string) (time.Time, bool) {
	parts := strings.FieldsFunc(path, func(r rune) bool { return r == '/' || r == '\\' })
	for i := 0; i+2 < len(parts); i++ {
		if utf8.RuneCountInString(parts[i]) != 4 {
			continue
		}
		year, err1 := strconv.Atoi(parts[i])
		month, err2 := strconv.Atoi(parts[i+1])
		day, err3 := strconv.Atoi(parts[i+2])
		if err1 != nil || err2 != nil || err3 != nil {
			continue
		}
		if month < 1 || month > 12 || day < 1 || day > 31 {
			return time.Time{}, false
		}
		return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.Local), true
	}
	return time.Time{}, false
}

// splitLines splits on "\n", drops a trailing "\r" and skips empty lines.
func splitLines(data []byte) []string {
	text := strings.ToValidUTF8(string(data), "�")
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSuffix(line, "\r")
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
