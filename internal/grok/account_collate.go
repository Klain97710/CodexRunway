package grok

import (
	"strings"
	"unicode"
)

// Foundation orders the keys of a JSONSerialization document, and the display
// names of the account list, with a localized comparison. This file rebuilds
// the part of it that matters for credential files: punctuation before digits
// before letters, digit runs by value, accents and case as tie breakers
// (lowercase first), and the usual Latin ligatures. Scripts other than Latin
// sort by code point, after Latin.

// acctPunctuationOrder lists the ASCII symbols in the order the root
// collation ranks them; a symbol's weight is its index.
const acctPunctuationOrder = " _-,;:!?.'\"()[]{}@*/\\&#%`^+<=>|~$"

// acctAccents ranks combining marks the way the secondary level does; the
// weight is the index plus one, no accent being zero.
var acctAccentRank = map[rune]int{
	0x301: 1, 0x300: 2, 0x306: 3, 0x302: 4, 0x30C: 5, 0x30A: 6, 0x308: 7,
	0x30B: 8, 0x303: 9, 0x307: 10, 0x327: 11, 0x328: 12, 0x304: 13, 0x338: 14,
}

// acctLatinAccents maps a base letter to the accented forms folded onto it,
// listed in the order of acctAccentRank's marks they carry.
var acctFolded = func() map[rune]acctElement {
	m := map[rune]acctElement{}
	add := func(base rune, mark rune, chars string) {
		for _, c := range chars {
			m[c] = acctElement{primary: acctLetterWeight(base), accent: acctAccentRank[mark]}
		}
	}
	for _, spec := range []struct {
		base        rune
		mark        rune
		lower, high string
	}{
		{'a', 0x301, "á", "Á"}, {'a', 0x300, "à", "À"}, {'a', 0x306, "ă", "Ă"}, {'a', 0x302, "â", "Â"},
		{'a', 0x30A, "å", "Å"}, {'a', 0x308, "ä", "Ä"}, {'a', 0x303, "ã", "Ã"}, {'a', 0x328, "ą", "Ą"},
		{'a', 0x304, "ā", "Ā"}, {'c', 0x301, "ć", "Ć"}, {'c', 0x302, "ĉ", "Ĉ"}, {'c', 0x30C, "č", "Č"},
		{'c', 0x327, "ç", "Ç"}, {'c', 0x307, "ċ", "Ċ"}, {'d', 0x30C, "ď", "Ď"}, {'e', 0x301, "é", "É"},
		{'e', 0x300, "è", "È"}, {'e', 0x302, "ê", "Ê"}, {'e', 0x30C, "ě", "Ě"}, {'e', 0x308, "ë", "Ë"},
		{'e', 0x307, "ė", "Ė"}, {'e', 0x328, "ę", "Ę"}, {'e', 0x304, "ē", "Ē"}, {'g', 0x306, "ğ", "Ğ"},
		{'i', 0x301, "í", "Í"}, {'i', 0x300, "ì", "Ì"}, {'i', 0x302, "î", "Î"}, {'i', 0x308, "ï", "Ï"},
		{'i', 0x303, "ĩ", "Ĩ"}, {'i', 0x304, "ī", "Ī"}, {'l', 0x301, "ĺ", "Ĺ"}, {'n', 0x301, "ń", "Ń"},
		{'n', 0x303, "ñ", "Ñ"}, {'n', 0x30C, "ň", "Ň"}, {'o', 0x301, "ó", "Ó"}, {'o', 0x300, "ò", "Ò"},
		{'o', 0x302, "ô", "Ô"}, {'o', 0x308, "ö", "Ö"}, {'o', 0x303, "õ", "Õ"}, {'o', 0x30B, "ő", "Ő"},
		{'o', 0x304, "ō", "Ō"}, {'o', 0x338, "ø", "Ø"}, {'r', 0x30C, "ř", "Ř"}, {'s', 0x301, "ś", "Ś"},
		{'s', 0x30C, "š", "Š"}, {'s', 0x327, "ş", "Ş"}, {'t', 0x30C, "ť", "Ť"}, {'u', 0x301, "ú", "Ú"},
		{'u', 0x300, "ù", "Ù"}, {'u', 0x302, "û", "Û"}, {'u', 0x308, "ü", "Ü"}, {'u', 0x30A, "ů", "Ů"},
		{'u', 0x30B, "ű", "Ű"}, {'u', 0x304, "ū", "Ū"}, {'y', 0x301, "ý", "Ý"}, {'y', 0x308, "ÿ", "Ÿ"},
		{'z', 0x301, "ź", "Ź"}, {'z', 0x30C, "ž", "Ž"}, {'z', 0x307, "ż", "Ż"},
	} {
		add(spec.base, spec.mark, spec.lower)
		for _, c := range spec.high {
			m[c] = acctElement{primary: acctLetterWeight(spec.base), accent: acctAccentRank[spec.mark], upper: true}
		}
	}
	return m
}()

// acctElement is one collation element: a primary weight, plus the accent
// (secondary) and case/variant (tertiary) tie breakers.
type acctElement struct {
	primary  int
	accent   int
	upper    bool
	compat   bool // ligature or sharp s: sorts after its plain spelling
	numeric  string
	isNumber bool
}

const (
	acctPunctBase  = 100
	acctNumberBase = 1000
	acctLetterBase = 2000
	acctOtherBase  = 100000
)

func acctLetterWeight(r rune) int { return acctLetterBase + int(unicode.ToLower(r)-'a')*10 }

// acctElements expands s into collation elements.
func acctElements(s string) []acctElement {
	var out []acctElement
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch {
		case r >= '0' && r <= '9':
			j := i
			for j < len(runes) && runes[j] >= '0' && runes[j] <= '9' {
				j++
			}
			digits := strings.TrimLeft(string(runes[i:j]), "0")
			out = append(out, acctElement{primary: acctNumberBase, numeric: digits, isNumber: true})
			i = j - 1
		case r < 0x80 && unicode.IsLetter(r):
			out = append(out, acctElement{primary: acctLetterWeight(r), upper: unicode.IsUpper(r)})
		case r < 0x80:
			if p := strings.IndexRune(acctPunctuationOrder, r); p >= 0 {
				out = append(out, acctElement{primary: acctPunctBase + p})
			} else {
				out = append(out, acctElement{primary: acctPunctBase + len(acctPunctuationOrder) + int(r)})
			}
		default:
			out = append(out, acctExpand(r)...)
		}
	}
	return out
}

// acctExpand collates a non-ASCII rune.
func acctExpand(r rune) []acctElement {
	switch r {
	case 'ß':
		return []acctElement{{primary: acctLetterWeight('s'), compat: true}, {primary: acctLetterWeight('s'), compat: true}}
	case 'æ', 'Æ':
		return []acctElement{{primary: acctLetterWeight('a'), compat: true, upper: r == 'Æ'}, {primary: acctLetterWeight('e'), compat: true}}
	case 'œ', 'Œ':
		return []acctElement{{primary: acctLetterWeight('o'), compat: true, upper: r == 'Œ'}, {primary: acctLetterWeight('e'), compat: true}}
	case 'ﬁ':
		return []acctElement{{primary: acctLetterWeight('f'), compat: true}, {primary: acctLetterWeight('i'), compat: true}}
	case 'ﬂ':
		return []acctElement{{primary: acctLetterWeight('f'), compat: true}, {primary: acctLetterWeight('l'), compat: true}}
	case 'ǆ', 'ǅ', 'Ǆ':
		return []acctElement{{primary: acctLetterWeight('d'), compat: true, upper: r == 'Ǆ'}, {primary: acctLetterWeight('z'), accent: acctAccentRank[0x30C], compat: true}}
	case 'đ', 'Đ':
		return []acctElement{{primary: acctLetterWeight('d'), accent: acctAccentRank[0x338], upper: r == 'Đ'}}
	case 'ł', 'Ł':
		return []acctElement{{primary: acctLetterWeight('l'), accent: acctAccentRank[0x338], upper: r == 'Ł'}}
	}
	if e, ok := acctFolded[r]; ok {
		return []acctElement{e}
	}
	lower := unicode.ToLower(r)
	return []acctElement{{primary: acctOtherBase + int(lower), upper: lower != r}}
}

// acctCollateCompare orders a and b: primary weights first, then accents,
// then case with lowercase first, and finally the raw bytes so the order is
// total.
func acctCollateCompare(a, b string) int {
	ea, eb := acctElements(a), acctElements(b)
	if c := acctCompareLevel(ea, eb, acctComparePrimary); c != 0 {
		return c
	}
	if c := acctCompareLevel(ea, eb, func(x, y acctElement) int { return acctCmpInt(x.accent, y.accent) }); c != 0 {
		return c
	}
	if c := acctCompareLevel(ea, eb, acctCompareTertiary); c != 0 {
		return c
	}
	return strings.Compare(a, b)
}

// acctCollateFolded is acctCollateCompare without the case level, the
// comparison behind localizedCaseInsensitiveCompare.
func acctCollateFolded(a, b string) int {
	ea, eb := acctElements(a), acctElements(b)
	if c := acctCompareLevel(ea, eb, acctComparePrimary); c != 0 {
		return c
	}
	return acctCompareLevel(ea, eb, func(x, y acctElement) int { return acctCmpInt(x.accent, y.accent) })
}

func acctCollateLess(a, b string) bool { return acctCollateCompare(a, b) < 0 }

func acctCompareLevel(a, b []acctElement, cmp func(x, y acctElement) int) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		if c := cmp(a[i], b[i]); c != 0 {
			return c
		}
	}
	return acctCmpInt(len(a), len(b))
}

func acctComparePrimary(x, y acctElement) int {
	if c := acctCmpInt(x.primary, y.primary); c != 0 {
		return c
	}
	if x.isNumber && y.isNumber {
		if c := acctCmpInt(len(x.numeric), len(y.numeric)); c != 0 {
			return c
		}
		return strings.Compare(x.numeric, y.numeric)
	}
	return 0
}

func acctCompareTertiary(x, y acctElement) int {
	if x.compat != y.compat {
		if x.compat {
			return 1
		}
		return -1
	}
	if x.upper != y.upper {
		if x.upper {
			return 1
		}
		return -1
	}
	return 0
}

func acctCmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}
