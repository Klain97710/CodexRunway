package grok

import (
	"errors"
	"math"
	"strconv"
	"unicode/utf16"
	"unicode/utf8"
)

// jsonKind tags a node of the lenient JSON tree.
type jsonKind uint8

const (
	jsonNull jsonKind = iota
	jsonBool
	jsonNumber
	jsonString
	jsonArray
	jsonObject
)

// jsonNode is one parsed JSON value. Numbers keep their source text so each
// reader can apply the Swift decoder it replaces (Int64, Double, rounding).
type jsonNode struct {
	kind jsonKind
	b    bool
	s    string // string value, or the number token
	bad  bool   // a string whose content cannot be read (lazy parsing only)
	arr  []*jsonNode
	obj  map[string]*jsonNode
}

// maxJSONContainerDepth is the deepest container nesting the system decoder
// accepts: a top-level object holding 512 nested arrays still decodes.
const maxJSONContainerDepth = 513

var errJSONSyntax = errors.New("grok: malformed JSON")

// parseLooseJSON parses data the way the Foundation JSON decoders used by the
// Swift code do: a leading byte order mark is skipped, trailing commas are
// accepted in objects and arrays, the first of two equal keys wins, control
// characters inside strings, lone surrogates and numbers that overflow a
// Double are errors, and anything after the value but whitespace is an error.
func parseLooseJSON(data []byte) (*jsonNode, error) {
	return parseJSON(data, false)
}

// parseDecoderJSON parses like JSONDecoder, which validates lazily: a number
// that overflows a Double and a string with bad content are accepted here
// and fail only when that value is read (jsonNode.bad, billingNumberDoubleOK).
func parseDecoderJSON(data []byte) (*jsonNode, error) {
	return parseJSON(data, true)
}

func parseJSON(data []byte, lazy bool) (*jsonNode, error) {
	p := &jsonParser{b: data, lazy: lazy}
	for p.hasPrefixBOM() {
		p.i += 3
	}
	p.skipSpace()
	n, err := p.value(0)
	if err != nil {
		return nil, err
	}
	p.skipSpace()
	if p.i != len(p.b) {
		return nil, errJSONSyntax
	}
	return n, nil
}

type jsonParser struct {
	b    []byte
	i    int
	lazy bool // JSONDecoder flavour, see parseDecoderJSON
}

func (p *jsonParser) hasPrefixBOM() bool {
	return len(p.b)-p.i >= 3 && p.b[p.i] == 0xEF && p.b[p.i+1] == 0xBB && p.b[p.i+2] == 0xBF
}

func (p *jsonParser) skipSpace() {
	for p.i < len(p.b) {
		switch p.b[p.i] {
		case ' ', '\t', '\n', '\r':
			p.i++
		default:
			return
		}
	}
}

func (p *jsonParser) peek() byte {
	if p.i < len(p.b) {
		return p.b[p.i]
	}
	return 0
}

func (p *jsonParser) literal(word string) bool {
	if len(p.b)-p.i >= len(word) && string(p.b[p.i:p.i+len(word)]) == word {
		p.i += len(word)
		return true
	}
	return false
}

func (p *jsonParser) value(depth int) (*jsonNode, error) {
	if p.i >= len(p.b) {
		return nil, errJSONSyntax
	}
	switch c := p.b[p.i]; {
	case c == '{':
		return p.object(depth + 1)
	case c == '[':
		return p.array(depth + 1)
	case c == '"':
		s, bad, err := p.str()
		if err != nil {
			return nil, err
		}
		return &jsonNode{kind: jsonString, s: s, bad: bad}, nil
	case c == 't':
		if p.literal("true") {
			return &jsonNode{kind: jsonBool, b: true}, nil
		}
	case c == 'f':
		if p.literal("false") {
			return &jsonNode{kind: jsonBool}, nil
		}
	case c == 'n':
		if p.literal("null") {
			return &jsonNode{kind: jsonNull}, nil
		}
	case c == '-' || (c >= '0' && c <= '9'):
		return p.number()
	}
	return nil, errJSONSyntax
}

func (p *jsonParser) object(depth int) (*jsonNode, error) {
	if depth > maxJSONContainerDepth {
		return nil, errJSONSyntax
	}
	p.i++ // {
	n := &jsonNode{kind: jsonObject, obj: map[string]*jsonNode{}}
	p.skipSpace()
	if p.peek() == '}' {
		p.i++
		return n, nil
	}
	for {
		p.skipSpace()
		if p.peek() != '"' {
			return nil, errJSONSyntax
		}
		key, badKey, err := p.str()
		if err != nil {
			return nil, err
		}
		p.skipSpace()
		if p.peek() != ':' {
			return nil, errJSONSyntax
		}
		p.i++
		p.skipSpace()
		v, err := p.value(depth)
		if err != nil {
			return nil, err
		}
		if _, dup := n.obj[key]; !dup && !badKey {
			n.obj[key] = v
		}
		p.skipSpace()
		switch p.peek() {
		case ',':
			p.i++
			p.skipSpace()
			if p.peek() == '}' {
				p.i++
				return n, nil
			}
		case '}':
			p.i++
			return n, nil
		default:
			return nil, errJSONSyntax
		}
	}
}

func (p *jsonParser) array(depth int) (*jsonNode, error) {
	if depth > maxJSONContainerDepth {
		return nil, errJSONSyntax
	}
	p.i++ // [
	n := &jsonNode{kind: jsonArray}
	p.skipSpace()
	if p.peek() == ']' {
		p.i++
		return n, nil
	}
	for {
		p.skipSpace()
		v, err := p.value(depth)
		if err != nil {
			return nil, err
		}
		n.arr = append(n.arr, v)
		p.skipSpace()
		switch p.peek() {
		case ',':
			p.i++
			p.skipSpace()
			if p.peek() == ']' {
				p.i++
				return n, nil
			}
		case ']':
			p.i++
			return n, nil
		default:
			return nil, errJSONSyntax
		}
	}
}

// number scans the strict JSON number grammar and rejects tokens a Double
// cannot hold.
func (p *jsonParser) number() (*jsonNode, error) {
	start := p.i
	if p.peek() == '-' {
		p.i++
	}
	switch c := p.peek(); {
	case c == '0':
		p.i++
	case c >= '1' && c <= '9':
		p.digits()
	default:
		return nil, errJSONSyntax
	}
	if p.peek() == '.' {
		p.i++
		if !p.digits() {
			return nil, errJSONSyntax
		}
	}
	if c := p.peek(); c == 'e' || c == 'E' {
		p.i++
		if c := p.peek(); c == '+' || c == '-' {
			p.i++
		}
		if !p.digits() {
			return nil, errJSONSyntax
		}
	}
	tok := string(p.b[start:p.i])
	if f, err := strconv.ParseFloat(tok, 64); !p.lazy && (err != nil || math.IsInf(f, 0)) {
		return nil, errJSONSyntax
	}
	return &jsonNode{kind: jsonNumber, s: tok}, nil
}

func (p *jsonParser) digits() bool {
	start := p.i
	for p.i < len(p.b) && p.b[p.i] >= '0' && p.b[p.i] <= '9' {
		p.i++
	}
	return p.i > start
}

// str scans a string literal. The literal ends at the first quote not
// preceded by a backslash; its content (control characters, escapes, UTF-8)
// is validated afterwards. A strict parser fails on bad content; a lazy one
// (the JSONDecoder flavour) reports bad == true and carries on, so only the
// value that is read as a String fails.
func (p *jsonParser) str() (s string, bad bool, err error) {
	start := p.i + 1
	i := start
	for i < len(p.b) && p.b[i] != '"' {
		if p.b[i] == '\\' {
			i++
		}
		i++
	}
	if i >= len(p.b) {
		return "", false, errJSONSyntax
	}
	p.i = i + 1
	s, ok := decodeJSONStringContent(p.b[start:i])
	if !ok {
		if p.lazy {
			return "", true, nil
		}
		return "", false, errJSONSyntax
	}
	return s, false, nil
}

// decodeJSONStringContent resolves the escapes of a string literal's content.
func decodeJSONStringContent(raw []byte) (string, bool) {
	sp := &jsonParser{b: raw}
	buf := make([]byte, 0, len(raw))
	for sp.i < len(raw) {
		c := raw[sp.i]
		switch {
		case c < 0x20:
			return "", false
		case c != '\\':
			buf = append(buf, c)
			sp.i++
		default:
			sp.i++
			if sp.i >= len(raw) {
				return "", false
			}
			e := raw[sp.i]
			sp.i++
			switch e {
			case '"', '\\', '/':
				buf = append(buf, e)
			case 'b':
				buf = append(buf, '\b')
			case 'f':
				buf = append(buf, '\f')
			case 'n':
				buf = append(buf, '\n')
			case 'r':
				buf = append(buf, '\r')
			case 't':
				buf = append(buf, '\t')
			case 'u':
				r, err := sp.unicodeEscape()
				if err != nil {
					return "", false
				}
				buf = utf8.AppendRune(buf, r)
			default:
				return "", false
			}
		}
	}
	if !utf8.Valid(buf) {
		return "", false
	}
	return string(buf), true
}

func (p *jsonParser) hex4() (rune, bool) {
	if len(p.b)-p.i < 4 {
		return 0, false
	}
	var r rune
	for _, c := range p.b[p.i : p.i+4] {
		r <<= 4
		switch {
		case c >= '0' && c <= '9':
			r |= rune(c - '0')
		case c >= 'a' && c <= 'f':
			r |= rune(c-'a') + 10
		case c >= 'A' && c <= 'F':
			r |= rune(c-'A') + 10
		default:
			return 0, false
		}
	}
	p.i += 4
	return r, true
}

// unicodeEscape reads the XXXX of a \uXXXX escape (the \u is consumed),
// joining a surrogate pair and rejecting a lone surrogate.
func (p *jsonParser) unicodeEscape() (rune, error) {
	r, ok := p.hex4()
	if !ok {
		return 0, errJSONSyntax
	}
	if utf16.IsSurrogate(r) {
		if r >= 0xDC00 || !(len(p.b)-p.i >= 2 && p.b[p.i] == '\\' && p.b[p.i+1] == 'u') {
			return 0, errJSONSyntax
		}
		p.i += 2
		lo, ok := p.hex4()
		if !ok || lo < 0xDC00 || lo > 0xDFFF {
			return 0, errJSONSyntax
		}
		return utf16.DecodeRune(r, lo), nil
	}
	return r, nil
}

// get returns the value of key when n is an object.
func (n *jsonNode) get(key string) *jsonNode {
	if n == nil || n.kind != jsonObject {
		return nil
	}
	return n.obj[key]
}

// str returns the string value when n is a JSON string.
func (n *jsonNode) str() (string, bool) {
	if n == nil || n.kind != jsonString || n.bad {
		return "", false
	}
	return n.s, true
}

// boolean returns the value when n is a JSON true or false.
func (n *jsonNode) boolean() (bool, bool) {
	if n == nil || n.kind != jsonBool {
		return false, false
	}
	return n.b, true
}

// present reports a node that exists and is not null (decodeIfPresent).
func (n *jsonNode) present() bool { return n != nil && n.kind != jsonNull }
