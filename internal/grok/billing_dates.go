package grok

import (
	"time"
	"unicode"
)

// BillingParseDate reads a timestamp the way the Swift app's RunwayDates.parse
// does (two ISO8601DateFormatter passes, with and without fractional
// seconds), which is considerably more lenient than RFC 3339:
//
//   - leading whitespace and anything after the zone are ignored;
//   - the year, month, day, hour, minute and second are digit runs of any
//     length ("2026-8-6T1:2:3Z"), optionally with an integral exponent
//     ("1e1" is 10); month 1-12, day 1-31 (an impossible day such as Feb 30
//     rolls over), hour 0-24, minute and second 0-59;
//   - the separators are lenient: "-" may be replaced or surrounded by
//     ",", "." or "/" and whitespace, ":" by "-", "." or whitespace; the "T"
//     between date and time is required and case sensitive;
//   - a fraction is truncated to milliseconds;
//   - the zone is "Z", or a sign followed by hours, minutes and seconds
//     written like "+5", "+0530", "+05:30", "+053015" (see zone); a
//     missing zone is a failure.
//
// Known residual differences (inputs no server sends): fractions longer than
// ten digits, non-ASCII digits, and years 12000-12099, which the system
// parser folds into 2000-2099 (mirrored here).
func BillingParseDate(text string) (time.Time, bool) {
	sc := billingDateScanner{s: []rune(text)}
	sc.skipSpace()
	neg := false
	if sc.peek() == '-' {
		neg = true
		sc.i++
	}
	year, ok := sc.number()
	if !ok {
		return time.Time{}, false
	}
	if neg {
		year = -year
	}
	if year >= 12000 && year <= 12099 {
		year -= 10000
	}

	var fields [5]int // month, day, hour, minute, second
	for k := range fields {
		minus := false // a dash before the number reads as a minus sign
		switch k {
		case 0, 1:
			if !sc.separator('-', ',', '.', '/') {
				return time.Time{}, false
			}
			sc.skipSpace()
		case 2:
			if sc.peek() != 'T' {
				return time.Time{}, false
			}
			sc.i++
			sc.skipSpace()
		default:
			if !sc.separator(':', '-', '.') {
				return time.Time{}, false
			}
			minus = sc.skipLeading('-', '\u2013')
		}
		v, ok := sc.number()
		if !ok || (minus && v != 0) {
			return time.Time{}, false
		}
		fields[k] = v
	}
	month, day, hour, minute, second := fields[0], fields[1], fields[2], fields[3], fields[4]
	if month < 1 || month > 12 || day < 1 || day > 31 || hour > 24 || minute > 59 || second > 59 {
		return time.Time{}, false
	}

	nanos := 0
	if sc.peek() == '.' {
		save := sc.i
		sc.i++
		sc.skipSpace()
		start := sc.i
		scale := 100_000_000
		for isBillingDigit(sc.peek()) {
			if sc.i-start < 3 {
				nanos += int(sc.peek()-'0') * scale
				scale /= 10
			}
			sc.i++
		}
		if sc.i == start {
			// Not a fraction: the plain pass sees "." where the zone belongs.
			sc.i = save
			return time.Time{}, false
		}
	}
	sc.skipSpace()
	offset, ok := sc.zone()
	if !ok {
		return time.Time{}, false
	}
	days := billingEpochDays(year, month, day)
	secs := days*86400 + int64(hour)*3600 + int64(minute)*60 + int64(second)
	return time.Unix(secs, int64(nanos)).UTC().Add(-offset), true
}

type billingDateScanner struct {
	s []rune
	i int
}

func (sc *billingDateScanner) peek() rune {
	if sc.i < len(sc.s) {
		return sc.s[sc.i]
	}
	return 0
}

func (sc *billingDateScanner) skipSpace() {
	for sc.i < len(sc.s) && isBillingSpace(sc.s[sc.i]) {
		sc.i++
	}
}

// skipLeading skips whitespace and the given runes before a number and
// reports whether any of those runes was skipped.
func (sc *billingDateScanner) skipLeading(extra ...rune) bool {
	skipped := false
	for sc.i < len(sc.s) {
		r := sc.s[sc.i]
		if containsRune(extra, r) {
			skipped = true
		} else if !isBillingSpace(r) {
			break
		}
		sc.i++
	}
	return skipped
}

func containsRune(set []rune, r rune) bool {
	for _, c := range set {
		if c == r {
			return true
		}
	}
	return false
}

// separator consumes the gap between two fields: whitespace and the
// ignorable punctuation (every rune of ignorable except the first, which is
// the literal the pattern asks for) in any order, then the literal itself.
// Without the literal at least one ignorable must have been skipped.
func (sc *billingDateScanner) separator(literal rune, ignorable ...rune) bool {
	skipped := 0
	for sc.i < len(sc.s) {
		r := sc.s[sc.i]
		if r == literal || !(isBillingSpace(r) || containsRune(ignorable, r)) {
			break
		}
		sc.i++
		skipped++
	}
	if sc.peek() == literal {
		sc.i++
		return true
	}
	return skipped > 0
}

// number reads digits with an optional integral exponent ("1e1", "10e-1").
func (sc *billingDateScanner) number() (int, bool) {
	start := sc.i
	v := 0
	for isBillingDigit(sc.peek()) {
		if v < 1<<40 {
			v = v*10 + int(sc.peek()-'0')
		}
		sc.i++
	}
	if sc.i == start {
		return 0, false
	}
	if r := sc.peek(); r == 'e' || r == 'E' {
		j := sc.i + 1
		expNeg := false
		if j < len(sc.s) && (sc.s[j] == '+' || sc.s[j] == '-') {
			expNeg = sc.s[j] == '-'
			j++
		}
		es := j
		exp := 0
		for j < len(sc.s) && isBillingDigit(sc.s[j]) {
			if exp < 100 {
				exp = exp*10 + int(sc.s[j]-'0')
			}
			j++
		}
		if j > es {
			sc.i = j
			if expNeg {
				for ; exp > 0; exp-- {
					if v%10 != 0 {
						return 0, false
					}
					v /= 10
				}
			} else {
				for ; exp > 0; exp-- {
					if v >= 1<<40 {
						return 0, false
					}
					v *= 10
				}
			}
		}
	}
	return v, true
}

// zone reads "Z" or a signed offset. After the sign the ASCII digit run is
// interpreted like ICU's abutting offset fields: up to six digits split as
// H, HH, Hmm, HHmm, Hmmss or HHmmss, shortening the run until every field is
// in range (hour <= 23, minute and second <= 59). A two digit hour may be
// followed by ":mm" and ":mm:ss"; a one digit hour followed by a colon takes
// the next digit as its second digit ("+1:30" is 13 hours). Anything after
// what was read is ignored.
func (sc *billingDateScanner) zone() (time.Duration, bool) {
	switch sc.peek() {
	case 'Z', 'z':
		return 0, true
	case '+', '-':
	default:
		return 0, false
	}
	sign := time.Duration(1)
	if sc.peek() == '-' {
		sign = -1
	}
	sc.i++
	start := sc.i
	for isBillingDigit(sc.peek()) {
		sc.i++
	}
	digits := sc.s[start:sc.i]
	if len(digits) == 0 {
		return 0, false
	}
	n := len(digits)
	if n > 6 {
		n = 6
	}
	d := func(k int) int { return int(digits[k] - '0') }
	pair := func(k int) int { return d(k)*10 + d(k+1) }
	var h, m, s int
	for ; n >= 1; n-- {
		switch n {
		case 6:
			h, m, s = pair(0), pair(2), pair(4)
		case 5:
			h, m, s = d(0), pair(1), pair(3)
		case 4:
			h, m, s = pair(0), pair(2), 0
		case 3:
			h, m, s = d(0), pair(1), 0
		case 2:
			h, m, s = pair(0), 0, 0
		default:
			h, m, s = d(0), 0, 0
		}
		if h <= 23 && m <= 59 && s <= 59 {
			break
		}
	}
	switch {
	case len(digits) == 1 && sc.peek() == ':' && sc.i+1 < len(sc.s) && isBillingDigit(sc.s[sc.i+1]):
		if two := h*10 + int(sc.s[sc.i+1]-'0'); two <= 23 {
			h = two
		}
	case len(digits) == 2 && n == 2:
		if sc.peek() == ':' {
			if v, ok := sc.colonField(); ok {
				m = v
				if sc.peek() == ':' {
					if v, ok := sc.colonField(); ok {
						s = v
					}
				}
			}
		}
	}
	return sign * (time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(s)*time.Second), true
}

// colonField reads ":dd" with dd <= 59, leaving the scanner untouched
// otherwise.
func (sc *billingDateScanner) colonField() (int, bool) {
	if sc.i+2 >= len(sc.s) || !isBillingDigit(sc.s[sc.i+1]) || !isBillingDigit(sc.s[sc.i+2]) {
		return 0, false
	}
	v := int(sc.s[sc.i+1]-'0')*10 + int(sc.s[sc.i+2]-'0')
	if v > 59 {
		return 0, false
	}
	sc.i += 3
	return v, true
}

// billingEpochDays is the number of days from 1970-01-01 to the given civil
// date. Like Foundation's Gregorian calendar it follows the Julian calendar
// before the 1582 cutover (15 October 1582); a day past the end of the month
// rolls into the next one.
func billingEpochDays(year, month, day int) int64 {
	a := int64(14-month) / 12
	y := int64(year) + 4800 - a
	m := int64(month) + 12*a - 3
	jdn := int64(day) + (153*m+2)/5 + 365*y + billingFloorDiv(y, 4)
	if year > 1582 || (year == 1582 && (month > 10 || (month == 10 && day >= 15))) {
		jdn += -billingFloorDiv(y, 100) + billingFloorDiv(y, 400) - 32045
	} else {
		jdn -= 32083
	}
	return jdn - 2440588
}

func billingFloorDiv(a, b int64) int64 {
	q := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		q--
	}
	return q
}

func isBillingDigit(r rune) bool { return r >= '0' && r <= '9' }

// isBillingSpace matches CharacterSet.whitespacesAndNewlines.
func isBillingSpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', 0x85:
		return true
	}
	return unicode.Is(unicode.Z, r)
}
