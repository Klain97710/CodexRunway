// Package tomlstmt finds and edits complete TOML statements while preserving
// all bytes outside the requested ranges. Callers must validate TOML with a
// full parser before and after editing.
package tomlstmt

import (
	"errors"
	"strconv"
)

// Kind identifies a lexical TOML statement type.
type Kind int

const (
	Blank Kind = iota
	Comment
	KeyVal
	Header
)

// Item is one complete TOML statement including its line terminator.
type Item struct {
	Start, End      int
	Kind            Kind
	Path            []string
	Array, MLString bool
}

// Edit replaces the byte range [Start, End) with Text.
type Edit struct {
	Start, End int
	Text       string
}

var errLex = errors.New("TOML uses syntax that cannot be edited safely")

// Lex splits data into complete statements without interpreting values.
func Lex(data []byte) ([]Item, error) {
	var items []Item
	for pos := 0; pos < len(data); {
		item, err := lexStatement(data, pos)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
		pos = item.End
	}
	return items, nil
}

func lexStatement(data []byte, start int) (Item, error) {
	p := skipBlanks(data, start)
	item := Item{Start: start}
	switch {
	case p >= len(data) || data[p] == '\n' || data[p] == '\r' && p+1 < len(data) && data[p+1] == '\n':
		item.Kind, item.End = Blank, lineEnd(data, p)
	case data[p] == '#':
		item.Kind, item.End = Comment, lineEnd(data, p)
	case data[p] == '[':
		return lexHeader(data, start, p)
	default:
		return lexKeyVal(data, start, p)
	}
	return item, nil
}

func lexHeader(data []byte, start, p int) (Item, error) {
	item := Item{Start: start, Kind: Header}
	p++
	if p < len(data) && data[p] == '[' {
		item.Array, p = true, p+1
	}
	path, p, err := lexKey(data, p)
	if err != nil {
		return item, err
	}
	p = skipBlanks(data, p)
	closers := 1
	if item.Array {
		closers = 2
	}
	for range closers {
		if p >= len(data) || data[p] != ']' {
			return item, errLex
		}
		p++
	}
	p = skipBlanks(data, p)
	if p < len(data) && data[p] != '#' && data[p] != '\n' && data[p] != '\r' {
		return item, errLex
	}
	item.Path, item.End = path, lineEnd(data, p)
	return item, nil
}

func lexKeyVal(data []byte, start, p int) (Item, error) {
	item := Item{Start: start, Kind: KeyVal}
	path, p, err := lexKey(data, p)
	if err != nil {
		return item, err
	}
	p = skipBlanks(data, p)
	if p >= len(data) || data[p] != '=' {
		return item, errLex
	}
	end, ml, err := scanValue(data, p+1)
	if err != nil {
		return item, err
	}
	item.Path, item.End, item.MLString = path, end, ml
	return item, nil
}

func lexKey(data []byte, p int) ([]string, int, error) {
	var parts []string
	for {
		p = skipBlanks(data, p)
		part, next, err := lexKeyPart(data, p)
		if err != nil {
			return nil, p, err
		}
		parts = append(parts, part)
		p = skipBlanks(data, next)
		if p >= len(data) || data[p] != '.' {
			return parts, p, nil
		}
		p++
	}
}

func lexKeyPart(data []byte, p int) (string, int, error) {
	if p >= len(data) {
		return "", p, errLex
	}
	if data[p] == '"' {
		return lexBasicKey(data, p)
	}
	if data[p] == '\'' {
		for i := p + 1; i < len(data) && data[i] != '\n'; i++ {
			if data[i] == '\'' {
				return string(data[p+1 : i]), i + 1, nil
			}
		}
		return "", p, errLex
	}
	i := p
	for i < len(data) && isBareKeyByte(data[i]) {
		i++
	}
	if i == p {
		return "", p, errLex
	}
	return string(data[p:i]), i, nil
}

func lexBasicKey(data []byte, p int) (string, int, error) {
	for i := p + 1; i < len(data) && data[i] != '\n'; i++ {
		if data[i] == '\\' {
			i++
		} else if data[i] == '"' {
			raw := string(data[p : i+1])
			if s, err := strconv.Unquote(raw); err == nil {
				return s, i + 1, nil
			}
			return raw, i + 1, nil
		}
	}
	return "", p, errLex
}

func scanValue(data []byte, p int) (int, bool, error) {
	depth, multiline := 0, false
	for i := p; i < len(data); i++ {
		switch data[i] {
		case '"', '\'':
			next, ml, err := skipString(data, i)
			if err != nil {
				return 0, false, err
			}
			multiline, i = multiline || ml, next-1
		case '#':
			for i < len(data) && data[i] != '\n' {
				i++
			}
			if depth == 0 {
				return min(i+1, len(data)), multiline, nil
			}
		case '[', '{':
			depth++
		case ']', '}':
			depth--
		case '\n':
			if depth <= 0 {
				return i + 1, multiline, nil
			}
		}
	}
	if depth > 0 {
		return 0, false, errLex
	}
	return len(data), multiline, nil
}

func skipString(data []byte, p int) (int, bool, error) {
	quote := data[p]
	if p+2 < len(data) && data[p+1] == quote && data[p+2] == quote {
		for i := p + 3; i < len(data); i++ {
			if quote == '"' && data[i] == '\\' {
				i++
			} else if data[i] == quote && i+2 < len(data) && data[i+1] == quote && data[i+2] == quote {
				for i+3 < len(data) && data[i+3] == quote {
					i++
				}
				return i + 3, true, nil
			}
		}
		return 0, true, errLex
	}
	for i := p + 1; i < len(data) && data[i] != '\n'; i++ {
		if quote == '"' && data[i] == '\\' {
			i++
		} else if data[i] == quote {
			return i + 1, false, nil
		}
	}
	return 0, false, errLex
}

func skipBlanks(data []byte, p int) int {
	for p < len(data) && (data[p] == ' ' || data[p] == '\t') {
		p++
	}
	return p
}

func lineEnd(data []byte, p int) int {
	for p < len(data) && data[p] != '\n' {
		p++
	}
	return min(p+1, len(data))
}

func isBareKeyByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-'
}
