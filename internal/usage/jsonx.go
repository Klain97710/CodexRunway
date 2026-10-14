package usage

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

// Helpers on jsonDec for decoders that read whole documents (the analytics
// response and the profile token usage), sharing the grammar of record.go.

// elements parses an array whose elements are handled by fn. fn must consume
// the element's value. The opening bracket is at the current position;
// trailing commas are accepted like in objects.
func (d *jsonDec) elements(fn func() error) error {
	d.skipSpace()
	if d.peek() != '[' {
		return errMalformed
	}
	if err := d.enter(); err != nil {
		return err
	}
	defer func() { d.depth-- }()
	d.i++
	d.skipSpace()
	if d.peek() == ']' {
		d.i++
		return nil
	}
	for {
		d.skipSpace()
		if err := fn(); err != nil {
			return err
		}
		d.skipSpace()
		switch d.peek() {
		case ',':
			d.i++
			d.skipSpace()
			if d.peek() == ']' {
				d.i++
				return nil
			}
		case ']':
			d.i++
			return nil
		default:
			return errMalformed
		}
	}
}

// optElements is elements for a value that may also be null.
func (d *jsonDec) optElements(fn func() error) error {
	if d.peek() == 'n' {
		if d.literal("null") {
			return nil
		}
		return errMalformed
	}
	return d.elements(fn)
}

// requiredString reads a string value; null and other types are errors.
func (d *jsonDec) requiredString() (string, error) {
	if d.peek() != '"' {
		return "", errMalformed
	}
	return d.decodeString()
}

// lenientString reads a string, or nil for anything else including a
// malformed string (the Swift `try?` pattern). The value is always consumed.
func (d *jsonDec) lenientString(dst **string) error {
	if d.peek() == '"' {
		start, depth := d.i, d.depth
		if s, err := d.decodeString(); err == nil {
			*dst = &s
			return nil
		}
		d.i, d.depth = start, depth
	}
	return d.skip()
}

// flexKind classifies a value read with flex.
type flexKind int

const (
	flexNull flexKind = iota
	flexNumber
	flexString
	flexOther
)

// flexValue is a scalar read without committing to a type.
type flexValue struct {
	kind flexKind
	tok  []byte // number token
	str  string
}

// flex reads null, a number token, a string, or consumes anything else.
func (d *jsonDec) flex() (flexValue, error) {
	switch c := d.peek(); {
	case c == 'n':
		if d.literal("null") {
			return flexValue{kind: flexNull}, nil
		}
		return flexValue{}, errMalformed
	case c == '-' || (c >= '0' && c <= '9'):
		tok := d.numberToken()
		if err := d.afterValue(); err != nil {
			return flexValue{}, err
		}
		return flexValue{kind: flexNumber, tok: tok}, nil
	case c == '"':
		s, err := d.decodeString()
		if err != nil {
			return flexValue{}, err
		}
		return flexValue{kind: flexString, str: s}, nil
	}
	if err := d.skip(); err != nil {
		return flexValue{}, err
	}
	return flexValue{kind: flexOther}, nil
}

// flexInt applies the "flexible integer" rule: null is 0, a number must be an
// exact integer, a string must parse as a decimal Int, anything else fails.
func (v flexValue) flexInt() (int64, bool) {
	switch v.kind {
	case flexNull:
		return 0, true
	case flexNumber:
		return parseIntegralJSON(v.tok)
	case flexString:
		n, err := strconv.ParseInt(v.str, 10, 64)
		return n, err == nil
	}
	return 0, false
}

var (
	// swiftDecimalDouble and swiftHexDouble are the spellings Swift's
	// Double(String) accepts that are finite: strtod-style decimal, and hex
	// mantissas with an optional binary exponent.
	swiftDecimalDouble = regexp.MustCompile(`^[+-]?([0-9]+\.?[0-9]*|\.[0-9]+)([eE][+-]?[0-9]+)?$`)
	swiftHexDouble     = regexp.MustCompile(`^[+-]?0[xX]([0-9a-fA-F]+\.?[0-9a-fA-F]*|\.[0-9a-fA-F]+)([pP][+-]?[0-9]+)?$`)
)

// flexDouble applies the "flexible double" rule: null is 0, a number or a
// numeric string must be finite.
func (v flexValue) flexDouble() (float64, bool) {
	switch v.kind {
	case flexNull:
		return 0, true
	case flexNumber:
		if !validJSONNumber(v.tok) {
			return 0, false
		}
		f, err := strconv.ParseFloat(string(v.tok), 64)
		if err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
			return 0, false
		}
		return f, true
	case flexString:
		s := v.str
		switch {
		case swiftHexDouble.MatchString(s):
			if !strings.ContainsAny(s, "pP") {
				s += "p0"
			}
		case !swiftDecimalDouble.MatchString(s):
			return 0, false
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
			return 0, false
		}
		return f, true
	}
	return 0, false
}

// validJSONNumber checks tok against the JSON number grammar.
func validJSONNumber(tok []byte) bool {
	i := 0
	if i < len(tok) && tok[i] == '-' {
		i++
	}
	if i >= len(tok) {
		return false
	}
	if tok[i] == '0' {
		i++
	} else if tok[i] >= '1' && tok[i] <= '9' {
		for i < len(tok) && tok[i] >= '0' && tok[i] <= '9' {
			i++
		}
	} else {
		return false
	}
	if i < len(tok) && tok[i] == '.' {
		i++
		n := 0
		for i < len(tok) && tok[i] >= '0' && tok[i] <= '9' {
			i++
			n++
		}
		if n == 0 {
			return false
		}
	}
	if i < len(tok) && (tok[i] == 'e' || tok[i] == 'E') {
		i++
		if i < len(tok) && (tok[i] == '+' || tok[i] == '-') {
			i++
		}
		n := 0
		for i < len(tok) && tok[i] >= '0' && tok[i] <= '9' {
			i++
			n++
		}
		if n == 0 {
			return false
		}
	}
	return i == len(tok)
}
