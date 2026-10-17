package tomlstmt

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
)

// ApplyEdits applies non-overlapping edits without mutating the input slice.
func ApplyEdits(data []byte, edits []Edit) []byte {
	ordered := append([]Edit(nil), edits...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Start < ordered[j].Start })
	out := make([]byte, 0, len(data)+256)
	pos := 0
	for _, edit := range ordered {
		out = append(out, data[pos:edit.Start]...)
		out = append(out, edit.Text...)
		pos = edit.End
	}
	return append(out, data[pos:]...)
}

// DetectEOL returns CRLF when the first newline uses it, otherwise LF.
func DetectEOL(data []byte) string {
	if i := bytes.IndexByte(data, '\n'); i > 0 && data[i-1] == '\r' {
		return "\r\n"
	}
	return "\n"
}

// Quote returns s as a TOML basic string.
func Quote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"', '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case '\b':
			b.WriteString(`\b`)
		case '\t':
			b.WriteString(`\t`)
		case '\n':
			b.WriteString(`\n`)
		case '\f':
			b.WriteString(`\f`)
		case '\r':
			b.WriteString(`\r`)
		default:
			if r < 0x20 || r == 0x7f {
				fmt.Fprintf(&b, `\u%04X`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// EndsWithBlankLine reports whether the final line before EOF is blank.
func EndsWithBlankLine(data []byte) bool {
	body := bytes.TrimSuffix(data, []byte("\n"))
	line := body[bytes.LastIndexByte(body, '\n')+1:]
	return len(bytes.TrimSpace(line)) == 0
}

// TableEnd returns the exclusive item index of a table's semantic body.
func TableEnd(items []Item, start int) int {
	if start < 0 {
		return -1
	}
	end := start + 1
	for end < len(items) && items[end].Kind != Header {
		end++
	}
	for end > start+1 && (items[end-1].Kind == Blank || items[end-1].Kind == Comment) {
		end--
	}
	return end
}
