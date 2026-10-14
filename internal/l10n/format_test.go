package l10n

import "testing"

type stringer string

func (s stringer) String() string { return "<" + string(s) + ">" }

func TestFormat(t *testing.T) {
	cases := []struct {
		format string
		args   []any
		want   string
	}{
		{"plain", nil, "plain"},
		{"plain", []any{1}, "plain"},
		{"%@", []any{"x"}, "x"},
		{"Imported %d account(s)", []any{3}, "Imported 3 account(s)"},
		{"Importing %d / %d", []any{1, 2}, "Importing 1 / 2"},
		{"%lld items", []any{int64(1) << 40}, "1099511627776 items"},
		{"%ld %li %lu %llu", []any{1, 2, 3, 4}, "1 2 3 4"},
		{"≥%@%%", []any{"75"}, "≥75%"},
		{"%%", nil, "%"},
		{"100%%", nil, "100%"},
		{"%2$@ then %1$@", []any{"a", "b"}, "b then a"},
		{"%1$@ %1$@", []any{"a"}, "a a"},
		{"%@ and %2$@", []any{"a", "b"}, "a and b"},
		{"%5d|%-5d|%05d|%+d", []any{42, 42, 42, 42}, "   42|42   |00042|+42"},
		{"%x %X %o", []any{255, 255, 8}, "ff FF 10"},
		{"%u", []any{-1}, "18446744073709551615"},
		{"%.2f|%8.3f|%f", []any{3.14159, 2.5, 1.0}, "3.14|   2.500|1.000000"},
		{"%g %e", []any{0.5, 1500.0}, "0.5 1.500000e+03"},
		{"%.1f", []any{3}, "3.0"},
		{"%5@|%-5@|%.2@", []any{"ab", "ab", "abcdef"}, "   ab|ab   |ab"},
		{"%@", []any{stringer("s")}, "<s>"},
		{"%@", []any{7}, "7"},
		{"%@", []any{nil}, "(null)"},
		{"%d", []any{"text"}, "text"},
		{"%c", []any{'A'}, "A"},
		{"日本語 %@ です", []any{"値"}, "日本語 値 です"},
		// Missing arguments and malformed specs are left visible.
		{"%@ %@", []any{"a"}, "a %@"},
		{"%3$d", []any{1}, "%3$d"},
		{"80%, 95%, or 100%.", nil, "80%, 95%, or 100%."},
		{"trailing %", nil, "trailing %"},
		{"%0$@", []any{"a"}, "%0$@"},
		{"%y", []any{"a"}, "%y"},
		{"%ll", []any{"a"}, "%ll"},
	}
	for _, c := range cases {
		if got := Format(c.format, c.args...); got != c.want {
			t.Errorf("Format(%q, %v) = %q, want %q", c.format, c.args, got, c.want)
		}
	}
}

func TestParseFormatPlaceholders(t *testing.T) {
	segs := parseFormat("a %1$@ b %lld c %%")
	var convs []byte
	for _, s := range segs {
		if s.spec != nil {
			convs = append(convs, s.spec.conv)
		}
	}
	if string(convs) != "@d%" {
		t.Errorf("convs = %q", convs)
	}
}
