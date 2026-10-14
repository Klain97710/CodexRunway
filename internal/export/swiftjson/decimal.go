package swiftjson

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
)

// maxDecimalDigits is the mantissa size of Foundation's Decimal.
const maxDecimalDigits = 38

// Decimal is an exact decimal number, the Go counterpart of Foundation's
// Decimal. Costs are summed as Decimal so cents never drift the way float64
// sums do. The zero value is 0.
type Decimal struct {
	r big.Rat
}

// ParseDecimal parses a plain or exponent decimal such as "1.25", "-0.00132"
// or "1e3".
func ParseDecimal(s string) (Decimal, error) {
	var d Decimal
	s = strings.TrimSpace(s)
	if s == "" || strings.ContainsRune(s, '/') {
		return d, fmt.Errorf("swiftjson: invalid decimal %q", s)
	}
	if _, ok := d.r.SetString(s); !ok {
		return Decimal{}, fmt.Errorf("swiftjson: invalid decimal %q", s)
	}
	return d, nil
}

// MustDecimal is ParseDecimal for constants; it panics on a bad literal.
func MustDecimal(s string) Decimal {
	d, err := ParseDecimal(s)
	if err != nil {
		panic(err)
	}
	return d
}

// DecimalFromInt returns n as a Decimal.
func DecimalFromInt(n int64) Decimal {
	var d Decimal
	d.r.SetInt64(n)
	return d
}

// DecimalFromRat returns a copy of r as a Decimal.
func DecimalFromRat(r *big.Rat) Decimal {
	var d Decimal
	d.r.Set(r)
	return d
}

// Rat returns a copy of the exact value.
func (d Decimal) Rat() *big.Rat { return new(big.Rat).Set(&d.r) }

// Add returns d+o.
func (d Decimal) Add(o Decimal) Decimal {
	var s Decimal
	s.r.Add(&d.r, &o.r)
	return s
}

// Cmp compares d with o and returns -1, 0 or 1.
func (d Decimal) Cmp(o Decimal) int { return d.r.Cmp(&o.r) }

// Sign returns -1, 0 or 1.
func (d Decimal) Sign() int { return d.r.Sign() }

// Equal reports whether the two values are numerically equal.
func (d Decimal) Equal(o Decimal) bool { return d.r.Cmp(&o.r) == 0 }

// Float64 returns the nearest float64 (Foundation's NSDecimalNumber.doubleValue).
func (d Decimal) Float64() float64 {
	f, _ := d.r.Float64()
	return f
}

// String is the plain-notation description Foundation gives a Decimal: no
// exponent, no trailing zeros ("12.5", "0.00132", "1000"). A value that does
// not terminate (a quotient) is rounded to 38 significant digits.
func (d Decimal) String() string {
	if d.r.Sign() == 0 {
		return "0"
	}
	num := new(big.Int).Abs(d.r.Num())
	den := d.r.Denom()
	intPart, rem := new(big.Int).QuoRem(num, den, new(big.Int))
	var sb strings.Builder
	if d.r.Sign() < 0 {
		sb.WriteByte('-')
	}
	sb.WriteString(intPart.String())
	if rem.Sign() == 0 {
		return sb.String()
	}
	digits := maxDecimalDigits
	if intPart.Sign() != 0 {
		digits -= len(intPart.String())
	}
	frac := make([]byte, 0, digits)
	ten := big.NewInt(10)
	leading := intPart.Sign() == 0
	for len(frac) < digits && rem.Sign() != 0 {
		rem.Mul(rem, ten)
		q, r2 := new(big.Int).QuoRem(rem, den, new(big.Int))
		rem = r2
		frac = append(frac, byte('0'+q.Int64()))
		if leading && q.Sign() == 0 {
			// Leading zeros after the point are not significant digits.
			digits++
			continue
		}
		leading = false
	}
	if rem.Sign() != 0 && len(frac) > 0 {
		rem.Mul(rem, big.NewInt(2))
		if rem.Cmp(den) >= 0 {
			frac = incrementDigits(frac)
		}
	}
	text := strings.TrimRight(string(frac), "0")
	if text == "" {
		return sb.String()
	}
	sb.WriteByte('.')
	sb.WriteString(text)
	return sb.String()
}

// incrementDigits adds one unit in the last place of a fraction. A carry out
// of the whole fraction (38 nines) is ignored; no real cost gets near it.
func incrementDigits(frac []byte) []byte {
	for i := len(frac) - 1; i >= 0; i-- {
		if frac[i] < '9' {
			frac[i]++
			return frac
		}
		frac[i] = '0'
	}
	return frac
}

// MarshalJSON writes the plain-notation number, the form Swift's JSONEncoder
// uses for a Decimal.
func (d Decimal) MarshalJSON() ([]byte, error) { return []byte(d.String()), nil }

// UnmarshalJSON reads a JSON number.
func (d *Decimal) UnmarshalJSON(data []byte) error {
	text := strings.TrimSpace(string(data))
	if text == "" || text == "null" || text[0] == '"' {
		return errors.New("swiftjson: decimal must be a JSON number")
	}
	parsed, err := ParseDecimal(text)
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}
