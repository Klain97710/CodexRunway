package swiftjson

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const hexDigits = "0123456789abcdef"

func (e *encoder) writeString(s string) {
	buf := &e.buf
	buf.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			buf.WriteString(`\"`)
		case '\\':
			buf.WriteString(`\\`)
		case '/':
			if e.options.WithoutEscapingSlashes {
				buf.WriteByte('/')
			} else {
				buf.WriteString(`\/`)
			}
		case '\b':
			buf.WriteString(`\b`)
		case '\f':
			buf.WriteString(`\f`)
		case '\n':
			buf.WriteString(`\n`)
		case '\r':
			buf.WriteString(`\r`)
		case '\t':
			buf.WriteString(`\t`)
		default:
			switch {
			case e.options.EscapeLineSeparators && (r == '\u2028' || r == '\u2029'):
				fmt.Fprintf(buf, `\u%04x`, r)
			case r < 0x20:
				buf.WriteString(`\u00`)
				buf.WriteByte(hexDigits[r>>4])
				buf.WriteByte(hexDigits[r&0xf])
			case r == utf8.RuneError:
				// Invalid UTF-8 cannot occur in a Swift String; keep the
				// output valid JSON.
				buf.WriteString("�")
			default:
				buf.WriteRune(r)
			}
		}
	}
	buf.WriteByte('"')
}

// FormatDouble prints f like Swift's Double.description as JSONEncoder uses it:
// the shortest digits that round-trip, plain notation for magnitudes from 1e-4
// up to 2^53, and an exponent ("1e+16", "1e-05", "5e-324") beyond that. A whole
// number has no ".0" and negative zero is "-0".
func FormatDouble(f float64) string {
	if f == 0 {
		if math.Signbit(f) {
			return "-0"
		}
		return "0"
	}
	abs := math.Abs(f)
	exp := strconv.FormatFloat(f, 'e', -1, 64) // d.ddde±XX
	mant, expPart, _ := strings.Cut(exp, "e")
	exp10, _ := strconv.Atoi(expPart)
	if abs > 1<<53 || exp10 < -4 {
		sign := "+"
		if exp10 < 0 {
			sign = "-"
			exp10 = -exp10
		}
		return fmt.Sprintf("%se%s%02d", mant, sign, exp10)
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// FormatDate is the ISO 8601 form of JSONEncoder's .iso8601 strategy: UTC, "Z",
// whole seconds (the fraction is dropped, not rounded).
func FormatDate(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05Z")
}

// FormatMillis is the rounded ISO8601DateFormatter representation in accounts
// and transfer packs, with UTC milliseconds.
func FormatMillis(t time.Time) string {
	return t.Round(time.Millisecond).UTC().Format("2006-01-02T15:04:05.000Z")
}

// ParseDate reads a date as RunwayDates.parse does: ISO 8601 with or without
// fractional seconds, with "Z" or a numeric offset.
func ParseDate(s string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, s)
}

// normalizeNumber reproduces how JSONSerialization prints a number it
// parsed: integers that fit 64 bits and bigger integer literals as digits,
// everything else as a double printed with 17 significant digits. Literals
// with more than 17 significant digits are kept as written (Foundation holds
// them as decimals).
func normalizeNumber(tok string) string {
	if isInteger(tok) {
		if n, err := strconv.ParseInt(tok, 10, 64); err == nil {
			return strconv.FormatInt(n, 10)
		}
		if n, err := strconv.ParseUint(tok, 10, 64); err == nil {
			return strconv.FormatUint(n, 10)
		}
		if b, ok := new(big.Int).SetString(tok, 10); ok {
			return b.String()
		}
		return tok
	}
	if significantDigits(tok) > 17 && !strings.ContainsAny(tok, "eE") {
		return tok
	}
	f, err := strconv.ParseFloat(tok, 64)
	if err != nil || math.IsInf(f, 0) {
		return tok
	}
	return strconv.FormatFloat(f, 'g', 17, 64)
}

func isInteger(tok string) bool {
	digits := strings.TrimPrefix(tok, "-")
	if digits == "" {
		return false
	}
	for i := 0; i < len(digits); i++ {
		if digits[i] < '0' || digits[i] > '9' {
			return false
		}
	}
	return true
}

// significantDigits counts the digits of the mantissa without leading and
// trailing zeros.
func significantDigits(tok string) int {
	mantissa := strings.TrimPrefix(tok, "-")
	if i := strings.IndexAny(mantissa, "eE"); i >= 0 {
		mantissa = mantissa[:i]
	}
	digits := strings.Replace(mantissa, ".", "", 1)
	digits = strings.TrimLeft(digits, "0")
	if strings.Contains(mantissa, ".") {
		digits = strings.TrimRight(digits, "0")
	}
	return len(digits)
}
