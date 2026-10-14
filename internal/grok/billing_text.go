package grok

import (
	"errors"
	"math"
	"strconv"
	"strings"
	"unicode"
)

// billingTrim trims what CharacterSet.whitespacesAndNewlines contains.
func billingTrim(s string) string { return strings.TrimFunc(s, isBillingSpace) }

// billingNonEmpty is the trimmed value, false when nothing is left.
func billingNonEmpty(s string) (string, bool) {
	t := billingTrim(s)
	return t, t != ""
}

// billingLocalizedCompare orders two names like localizedCaseInsensitiveCompare
// (negative, zero or positive): punctuation before digits before letters,
// digits one by one (no numeric ordering), accents as the second level and no
// case level. It shares the element table of the account list collation; text
// outside Latin is ordered by code point.
func billingLocalizedCompare(a, b string) int {
	ea, eb := billingElements(a), billingElements(b)
	if c := acctCompareLevel(ea, eb, acctComparePrimary); c != 0 {
		return c
	}
	return billingCompareSecondary(billingSecondary(ea), billingSecondary(eb))
}

// billingCommonWeight is the secondary weight of a plain letter; accent marks
// weigh less.
const billingCommonWeight = 1 << 20

// billingSecondary is the accent level: a plain element weighs billingCommonWeight
// and an accent adds a lighter mark element after its letter. Compared
// element by element, an accent that is followed by more text sorts before the
// plain spelling ("éa" < "ea") and a trailing one after it ("aé" > "ae").
func billingSecondary(es []acctElement) []int {
	out := make([]int, 0, len(es))
	for _, e := range es {
		out = append(out, billingCommonWeight)
		if e.accent != 0 {
			out = append(out, e.accent)
		}
	}
	return out
}

func billingCompareSecondary(a, b []int) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return acctCmpInt(a[i], b[i])
		}
	}
	return acctCmpInt(len(a), len(b))
}

// billingSymbolPrimary ranks non-ASCII symbols between punctuation and digits.
const billingSymbolPrimary = acctNumberBase - 100

// billingElements is acctElements with every digit a collation element of its
// own instead of one number per digit run.
func billingElements(s string) []acctElement {
	var out []acctElement
	chunk := make([]rune, 0, len(s))
	flush := func() {
		if len(chunk) > 0 {
			out = append(out, acctElements(string(chunk))...)
			chunk = chunk[:0]
		}
	}
	for _, r := range s {
		if r >= '0' && r <= '9' {
			flush()
			out = append(out, acctElement{primary: acctNumberBase + int(r-'0')})
			continue
		}
		if r >= 0x80 && (unicode.IsSymbol(r) || unicode.IsPunct(r)) {
			// Symbols and emoji rank above digits and letters; among
			// themselves they fall back to code point order.
			flush()
			out = append(out, acctElement{primary: billingSymbolPrimary, accent: int(r) + 1})
			continue
		}
		chunk = append(chunk, r)
	}
	flush()
	return out
}

// billingSwiftDouble reads text like Swift's Double(String): no surrounding
// whitespace, an optional sign, decimal or hexadecimal digits ("0x10" is 16),
// an exponent, "inf", "infinity" or "nan" in any case. A value out of range
// is infinite, not an error.
func billingSwiftDouble(text string) (float64, bool) {
	if text == "" || strings.ContainsRune(text, '_') {
		return 0, false
	}
	for _, r := range text {
		if r > unicode.MaxASCII {
			return 0, false
		}
	}
	s := text
	if strings.HasPrefix(strings.TrimLeft(s, "+-"), "0x") || strings.HasPrefix(strings.TrimLeft(s, "+-"), "0X") {
		if !strings.ContainsAny(s, "pP") {
			s += "p0"
		}
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		var numErr *strconv.NumError
		if errors.As(err, &numErr) && errors.Is(numErr.Err, strconv.ErrRange) {
			return f, true
		}
		return 0, false
	}
	return f, true
}

// billingRoundCents is Int64(value.rounded()): halves round away from zero.
// A value beyond the Int64 range traps in Swift; here it reports false.
func billingRoundCents(f float64) (int64, bool) {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, false
	}
	r := math.Round(f)
	if r >= 1<<63 || r < -(1<<63) {
		return 0, false
	}
	return int64(r), true
}

// billingParseCentText is the string branch of the Cent decoder: Int64(text),
// else a finite Double(text) rounded, after trimming whitespace.
func billingParseCentText(text string) (int64, bool) {
	t := billingTrim(text)
	if v, err := strconv.ParseInt(t, 10, 64); err == nil {
		return v, true
	}
	if f, ok := billingSwiftDouble(t); ok && !math.IsInf(f, 0) && !math.IsNaN(f) {
		return billingRoundCents(f)
	}
	return 0, false
}

// billingNumberInt64 is JSONDecoder's Int64 decode of a number token: an
// integral value that fits, written with or without a fraction or exponent.
func billingNumberInt64(tok string) (int64, bool) {
	if v, err := strconv.ParseInt(tok, 10, 64); err == nil {
		return v, true
	}
	if !strings.ContainsAny(tok, ".eE") {
		return 0, false
	}
	f, err := strconv.ParseFloat(tok, 64)
	if err != nil || f != math.Trunc(f) || f >= 1<<63 || f < -(1<<63) {
		return 0, false
	}
	return int64(f), true
}

// billingNumberDouble is JSONDecoder's Double decode of a number token.
func billingNumberDouble(tok string) float64 {
	f, _ := strconv.ParseFloat(tok, 64)
	return f
}
