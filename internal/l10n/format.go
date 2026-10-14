package l10n

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Format renders a Swift/Foundation String(format:) style template.
//
// Supported: %@ and %s (any value, rendered with its String/Error method or
// fmt.Sprint), %d %i %u %x %X %o (integers), %f %e %g (floats), %c, %%,
// positional arguments (%1$@), flags "-+ 0#", width, precision and the C
// length modifiers (h, l, ll, q, z, t, j), so %lld behaves like %d.
//
// Unlike Foundation, which has undefined behaviour, a specifier that is
// malformed or has no matching argument is copied to the output unchanged,
// and surplus arguments are ignored.
func Format(format string, args ...any) string {
	if !strings.Contains(format, "%") {
		return format
	}
	var b strings.Builder
	next := 0
	for _, seg := range parseFormat(format) {
		b.WriteString(seg.lit)
		sp := seg.spec
		if sp == nil {
			continue
		}
		if sp.conv == '%' {
			b.WriteByte('%')
			continue
		}
		idx := next
		if sp.pos > 0 {
			idx = sp.pos - 1
		} else {
			next++
		}
		if idx >= len(args) {
			b.WriteString(sp.raw)
			continue
		}
		b.WriteString(sp.render(args[idx]))
	}
	return b.String()
}

// segment is a run of literal text followed by an optional specifier.
type segment struct {
	lit  string
	spec *spec
}

type spec struct {
	raw       string // original text, used when the spec cannot be rendered
	pos       int    // 1-based explicit argument position, 0 when sequential
	flags     string
	width     int
	prec      int // -1 when absent
	conv      byte
	hasLength bool
}

// parseFormat splits format into literal text and specifiers. Text that
// looks like a specifier but is malformed stays literal.
func parseFormat(format string) []segment {
	var segs []segment
	var lit strings.Builder
	for i := 0; i < len(format); {
		if format[i] != '%' {
			lit.WriteByte(format[i])
			i++
			continue
		}
		sp, n := parseSpec(format[i:])
		if sp == nil {
			lit.WriteByte('%')
			i++
			continue
		}
		segs = append(segs, segment{lit.String(), sp})
		lit.Reset()
		i += n
	}
	if lit.Len() > 0 {
		segs = append(segs, segment{lit: lit.String()})
	}
	return segs
}

// parseSpec parses one specifier at the start of s (which begins with '%').
// It returns nil when s does not start with a valid specifier.
func parseSpec(s string) (*spec, int) {
	sp := &spec{prec: -1}
	i := 1
	// Optional "n$" argument position.
	j := i
	for j < len(s) && s[j] >= '0' && s[j] <= '9' {
		j++
	}
	if j > i && j < len(s) && s[j] == '$' {
		n, err := strconv.Atoi(s[i:j])
		if err != nil || n < 1 {
			return nil, 0
		}
		sp.pos = n
		i = j + 1
	}
	for i < len(s) && strings.IndexByte("-+ 0#'", s[i]) >= 0 {
		if s[i] != '\'' {
			sp.flags += string(s[i])
		}
		i++
	}
	i, sp.width = scanNumber(s, i)
	if i < len(s) && s[i] == '.' {
		var p int
		i, p = scanNumber(s, i+1)
		sp.prec = p
	}
	for i < len(s) && strings.IndexByte("hlqLztj", s[i]) >= 0 {
		sp.hasLength = true
		i++
	}
	if i >= len(s) || strings.IndexByte("@sSdiDuUxXoOfFeEgGcC%", s[i]) < 0 {
		return nil, 0
	}
	sp.conv = s[i]
	if sp.conv == '%' && (i != 1 || sp.hasLength) {
		return nil, 0
	}
	sp.raw = s[:i+1]
	return sp, i + 1
}

func scanNumber(s string, i int) (int, int) {
	n := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		n = n*10 + int(s[i]-'0')
		i++
	}
	return i, n
}

func (sp *spec) render(arg any) string {
	switch sp.conv {
	case '@', 's', 'S':
		return sp.pad(sp.truncate(stringOf(arg)))
	case 'c', 'C':
		if r, ok := intOf(arg); ok {
			return sp.pad(string(rune(r)))
		}
		return sp.pad(stringOf(arg))
	case 'd', 'i', 'D':
		if v, ok := intOf(arg); ok {
			return fmt.Sprintf(sp.goFormat('d'), v)
		}
	case 'u', 'U':
		if v, ok := intOf(arg); ok {
			return fmt.Sprintf(sp.goFormat('d'), uint64(v))
		}
	case 'x', 'X', 'o', 'O':
		if v, ok := intOf(arg); ok {
			verb := sp.conv
			if verb == 'O' {
				verb = 'o'
			}
			return fmt.Sprintf(sp.goFormat(verb), uint64(v))
		}
	case 'f', 'F', 'e', 'E', 'g', 'G':
		if v, ok := floatOf(arg); ok {
			verb := sp.conv
			if verb == 'F' {
				verb = 'f'
			}
			f := *sp
			if f.prec < 0 {
				f.prec = 6 // C default; Go's %g would use the shortest form
			}
			return fmt.Sprintf(f.goFormat(verb), v)
		}
	}
	// Argument of the wrong kind: show it rather than garbage.
	return sp.pad(stringOf(arg))
}

func (sp *spec) goFormat(verb byte) string {
	var b strings.Builder
	b.WriteByte('%')
	for _, f := range sp.flags {
		if f != '#' || verb == 'x' || verb == 'X' || verb == 'o' {
			b.WriteRune(f)
		}
	}
	if sp.width > 0 {
		b.WriteString(strconv.Itoa(sp.width))
	}
	if sp.prec >= 0 {
		b.WriteByte('.')
		b.WriteString(strconv.Itoa(sp.prec))
	}
	b.WriteByte(verb)
	return b.String()
}

func (sp *spec) truncate(s string) string {
	if sp.prec < 0 || utf8.RuneCountInString(s) <= sp.prec {
		return s
	}
	return string([]rune(s)[:sp.prec])
}

func (sp *spec) pad(s string) string {
	n := sp.width - utf8.RuneCountInString(s)
	if n <= 0 {
		return s
	}
	if strings.Contains(sp.flags, "-") {
		return s + strings.Repeat(" ", n)
	}
	return strings.Repeat(" ", n) + s
}

func stringOf(arg any) string {
	switch v := arg.(type) {
	case nil:
		return "(null)"
	case string:
		return v
	case error:
		return v.Error()
	case fmt.Stringer:
		return v.String()
	}
	return fmt.Sprint(arg)
}

func intOf(arg any) (int64, bool) {
	switch v := arg.(type) {
	case int:
		return int64(v), true
	case int8:
		return int64(v), true
	case int16:
		return int64(v), true
	case int32:
		return int64(v), true
	case int64:
		return v, true
	case uint:
		return int64(v), true
	case uint8:
		return int64(v), true
	case uint16:
		return int64(v), true
	case uint32:
		return int64(v), true
	case uint64:
		return int64(v), true
	case uintptr:
		return int64(v), true
	}
	return 0, false
}

func floatOf(arg any) (float64, bool) {
	switch v := arg.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	}
	if i, ok := intOf(arg); ok {
		return float64(i), true
	}
	return 0, false
}
