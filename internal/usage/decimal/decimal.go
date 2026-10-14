// Package decimal is a small exact decimal type that follows the arithmetic
// and text rules of Foundation's Decimal for the operations the usage cost
// code needs: construction from integers and text, addition, multiplication,
// division, comparison, and JSON numbers.
//
// A Decimal is a sign, a mantissa below 2^128 and a power-of-ten exponent in
// [-128, 127], like NSDecimal. Results that need a wider mantissa lose their
// trailing digits by truncation, exactly as Foundation does, and a result
// whose exponent leaves the range is reported through Overflow. The zero
// value is 0 and Decimal values are immutable: no method modifies its
// receiver or its arguments.
package decimal

import (
	"errors"
	"math/big"
	"strings"
)

const (
	minExponent = -128
	maxExponent = 127
)

var (
	maxMantissa = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1))
	ten         = big.NewInt(10)

	// ErrOverflow reports a result whose exponent is outside [-128, 127].
	ErrOverflow = errors.New("decimal: value out of range")
	// ErrSyntax reports text that does not start with a number.
	ErrSyntax = errors.New("decimal: invalid number")
)

// Decimal is an exact decimal number. It is always normalized: the mantissa
// has no trailing zeros, and zero has exponent 0.
type Decimal struct {
	mant *big.Int // signed; nil means 0
	exp  int
	bad  bool // set when an operation overflowed the exponent range
}

// Zero is the decimal 0.
var Zero Decimal

// FromInt64 returns n as a Decimal.
func FromInt64(n int64) Decimal {
	return normalize(big.NewInt(n), 0)
}

// FromInt returns n as a Decimal.
func FromInt(n int) Decimal { return FromInt64(int64(n)) }

// New returns mantissa * 10^exp.
func New(mantissa int64, exp int) Decimal {
	return normalize(big.NewInt(mantissa), exp)
}

// MustParse parses s like Parse and panics on error. It is meant for tables
// of constants.
func MustParse(s string) Decimal {
	d, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return d
}

// normalize strips trailing zeros, truncates a mantissa that needs more than
// 128 bits and applies the exponent range.
func normalize(m *big.Int, exp int) Decimal {
	if m.Sign() == 0 {
		return Decimal{}
	}
	m = new(big.Int).Set(m)
	neg := m.Sign() < 0
	if neg {
		m.Neg(m)
	}
	r := new(big.Int)
	for m.Cmp(maxMantissa) > 0 {
		m.QuoRem(m, ten, r)
		exp++
	}
	for {
		q, rem := new(big.Int).QuoRem(m, ten, new(big.Int))
		if rem.Sign() != 0 {
			break
		}
		m = q
		exp++
	}
	if neg {
		m.Neg(m)
	}
	d := Decimal{mant: m, exp: exp}
	if exp > maxExponent || exp < minExponent {
		d.bad = true
	}
	return d
}

func (d Decimal) m() *big.Int {
	if d.mant == nil {
		return new(big.Int)
	}
	return d.mant
}

// Overflow reports whether the value is the result of an operation that left
// the representable exponent range. Such a value must not be used.
func (d Decimal) Overflow() bool { return d.bad }

// IsZero reports whether d is 0.
func (d Decimal) IsZero() bool { return d.mant == nil || d.mant.Sign() == 0 }

// Sign returns -1, 0 or +1.
func (d Decimal) Sign() int {
	if d.mant == nil {
		return 0
	}
	return d.mant.Sign()
}

// Add returns d + o.
func (d Decimal) Add(o Decimal) Decimal {
	if d.IsZero() {
		return o
	}
	if o.IsZero() {
		return d
	}
	a, b := d.m(), o.m()
	exp := d.exp
	switch {
	case d.exp > o.exp:
		a = new(big.Int).Mul(a, pow10(d.exp-o.exp))
		exp = o.exp
	case o.exp > d.exp:
		b = new(big.Int).Mul(b, pow10(o.exp-d.exp))
	}
	return normalize(new(big.Int).Add(a, b), exp)
}

// Sub returns d - o.
func (d Decimal) Sub(o Decimal) Decimal { return d.Add(o.Neg()) }

// Neg returns -d.
func (d Decimal) Neg() Decimal {
	if d.IsZero() {
		return d
	}
	return Decimal{mant: new(big.Int).Neg(d.mant), exp: d.exp, bad: d.bad}
}

// Mul returns d * o.
func (d Decimal) Mul(o Decimal) Decimal {
	if d.IsZero() || o.IsZero() {
		return Decimal{}
	}
	return normalize(new(big.Int).Mul(d.m(), o.m()), d.exp+o.exp)
}

// Div returns d / o with as many quotient digits as fit a 128-bit mantissa,
// truncated. Dividing by zero returns a value with Overflow set.
func (d Decimal) Div(o Decimal) Decimal {
	if o.IsZero() {
		return Decimal{bad: true}
	}
	if d.IsZero() {
		return Decimal{}
	}
	num, den := new(big.Int).Abs(d.m()), new(big.Int).Abs(o.m())
	exp := d.exp - o.exp
	// Scale the numerator until the quotient has at least 39 digits, which is
	// the most a 128-bit mantissa can hold, then truncate.
	shift := 40 - (len(num.String()) - len(den.String()))
	if shift > 0 {
		num.Mul(num, pow10(shift))
		exp -= shift
	}
	q := num.Quo(num, den)
	if d.Sign()*o.Sign() < 0 {
		q.Neg(q)
	}
	return normalize(q, exp)
}

// Cmp compares d and o numerically and returns -1, 0 or +1.
func (d Decimal) Cmp(o Decimal) int {
	if ds, os := d.Sign(), o.Sign(); ds != os {
		if ds < os {
			return -1
		}
		return 1
	}
	if d.IsZero() {
		return 0
	}
	a, b := d.m(), o.m()
	switch {
	case d.exp > o.exp:
		a = new(big.Int).Mul(a, pow10(d.exp-o.exp))
	case o.exp > d.exp:
		b = new(big.Int).Mul(b, pow10(o.exp-d.exp))
	}
	return a.Cmp(b)
}

// Equal reports whether d and o are numerically equal.
func (d Decimal) Equal(o Decimal) bool { return d.Cmp(o) == 0 }

// Less reports whether d < o.
func (d Decimal) Less(o Decimal) bool { return d.Cmp(o) < 0 }

func pow10(n int) *big.Int {
	return new(big.Int).Exp(ten, big.NewInt(int64(n)), nil)
}

// String formats d as plain decimal digits without an exponent or trailing
// zeros, exactly like Decimal.description.
func (d Decimal) String() string {
	if d.IsZero() {
		return "0"
	}
	digits := new(big.Int).Abs(d.mant).String()
	var b strings.Builder
	if d.mant.Sign() < 0 {
		b.WriteByte('-')
	}
	switch {
	case d.exp >= 0:
		b.WriteString(digits)
		b.WriteString(strings.Repeat("0", d.exp))
	case -d.exp >= len(digits):
		b.WriteString("0.")
		b.WriteString(strings.Repeat("0", -d.exp-len(digits)))
		b.WriteString(digits)
	default:
		split := len(digits) + d.exp
		b.WriteString(digits[:split])
		b.WriteByte('.')
		b.WriteString(digits[split:])
	}
	return b.String()
}

// Parse reads the longest numeric prefix of s the way Decimal(string:locale:)
// does with the POSIX locale: optional leading spaces, an optional sign,
// digits with an optional fraction, and an optional exponent. Trailing text is
// ignored, a lone sign yields 0, and text with no number is ErrSyntax. Digits
// beyond 128 bits are truncated; an exponent outside [-128, 127] is
// ErrOverflow.
func Parse(s string) (Decimal, error) {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n' || s[i] == '\r') {
		i++
	}
	neg := false
	signed := false
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		neg = s[i] == '-'
		signed = true
		i++
	}
	var digits strings.Builder
	exp := 0
	sawDigit := false
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		digits.WriteByte(s[i])
		sawDigit = true
		i++
	}
	if i < len(s) && s[i] == '.' {
		j := i + 1
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			digits.WriteByte(s[j])
			exp--
			sawDigit = true
			j++
		}
		if sawDigit {
			i = j
		}
	}
	if !sawDigit {
		if signed {
			return Decimal{}, nil
		}
		return Decimal{}, ErrSyntax
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		j := i + 1
		eneg := false
		if j < len(s) && (s[j] == '+' || s[j] == '-') {
			eneg = s[j] == '-'
			j++
		}
		start := j
		e := 0
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			if e < 100000 {
				e = e*10 + int(s[j]-'0')
			}
			j++
		}
		if j > start {
			if eneg {
				e = -e
			}
			exp += e
		}
	}
	m, _ := new(big.Int).SetString(digits.String(), 10)
	if neg {
		m.Neg(m)
	}
	d := normalize(m, exp)
	if d.bad {
		return Decimal{}, ErrOverflow
	}
	return d, nil
}

// MarshalJSON writes d as a bare JSON number, like Foundation's encoder.
func (d Decimal) MarshalJSON() ([]byte, error) {
	if d.bad {
		return nil, ErrOverflow
	}
	return []byte(d.String()), nil
}

// UnmarshalJSON reads a JSON number exactly (not through float64).
func (d *Decimal) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "" || s == "null" || s[0] == '"' || s[0] == 't' || s[0] == 'f' || s[0] == '[' || s[0] == '{' {
		return ErrSyntax
	}
	v, err := Parse(s)
	if err != nil {
		return err
	}
	*d = v
	return nil
}
