package grok

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"
	"strconv"

	"github.com/licoy/quosparr/internal/export/swiftjson"
)

// The account side keeps credential files as generic JSON trees so fields this
// package does not model survive a rewrite. A tree holds nil, bool,
// json.Number (the source text of a number), string, []any and map[string]any.

// acctJSONMode picks which Foundation writer a tree is rendered like.
type acctJSONMode int

const (
	// acctSerialization is JSONSerialization with sortedKeys and
	// withoutEscapingSlashes: localized key order, normalized numbers. It
	// writes auth.json files and transfer packs.
	acctSerialization acctJSONMode = iota
	// acctEncoder is JSONEncoder with sortedKeys and withoutEscapingSlashes:
	// byte-ordered keys, numbers as written. It writes grok-index.json.
	acctEncoder
)

var errAcctNotJSON = errors.New("grok: not valid JSON")

// acctParseJSON decodes one JSON document, keeping number text. A leading
// byte order mark is skipped; anything after the value is an error.
func acctParseJSON(data []byte) (any, error) {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, errAcctNotJSON
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errAcctNotJSON
	}
	return v, nil
}

// acctParseObject decodes data, which must be a JSON object.
func acctParseObject(data []byte) (map[string]any, error) {
	v, err := acctParseJSON(data)
	if err != nil {
		return nil, err
	}
	o, ok := v.(map[string]any)
	if !ok {
		return nil, errAcctNotJSON
	}
	return o, nil
}

// acctJSONContainer reports whether data is a JSON array or object, the only
// top-level values JSONSerialization.jsonObject accepts.
func acctJSONContainer(data []byte) (any, bool) {
	v, err := acctParseJSON(data)
	if err != nil {
		return nil, false
	}
	switch v.(type) {
	case map[string]any, []any:
		return v, true
	}
	return nil, false
}

// acctMarshalJSON selects the original Foundation writer's compatibility settings.
func acctMarshalJSON(v any, mode acctJSONMode, pretty bool) ([]byte, error) {
	opt := swiftjson.Options{Pretty: pretty, WithoutEscapingSlashes: true}
	if mode == acctSerialization {
		opt.KeyLess = acctCollateLess
		opt.NormalizeNumbers = true
	}
	return swiftjson.MarshalWithOptions(v, opt)
}

// --- typed reading ---------------------------------------------------------

// acctReader pulls typed fields out of a decoded object with the strictness of
// a Swift synthesized Decodable: a wrong type is an error, null counts as
// absent, a missing required key is an error, and the first error sticks.
type acctReader struct {
	obj  map[string]any
	used map[string]struct{}
	err  error
}

func newAcctReader(o map[string]any) *acctReader {
	return &acctReader{obj: o, used: map[string]struct{}{}}
}

func (r *acctReader) fail(key, problem string) {
	if r.err == nil {
		r.err = errors.New("field " + strconv.Quote(key) + ": " + problem)
	}
}

func (r *acctReader) value(key string) (any, bool) {
	r.used[key] = struct{}{}
	v, ok := r.obj[key]
	if !ok || v == nil {
		return nil, false
	}
	return v, true
}

func (r *acctReader) optString(key string) string {
	v, ok := r.value(key)
	if !ok {
		return ""
	}
	s, isString := v.(string)
	if !isString {
		r.fail(key, "expected a string")
	}
	return s
}

func (r *acctReader) reqString(key string) string {
	if _, ok := r.value(key); !ok {
		r.fail(key, "missing")
		return ""
	}
	return r.optString(key)
}

func (r *acctReader) boolean(key string) bool {
	v, ok := r.value(key)
	if !ok {
		r.fail(key, "missing")
		return false
	}
	b, isBool := v.(bool)
	if !isBool {
		r.fail(key, "expected a boolean")
	}
	return b
}

func (r *acctReader) reqInt(key string) int {
	v, ok := r.value(key)
	if !ok {
		r.fail(key, "missing")
		return 0
	}
	n, isNumber := v.(json.Number)
	if !isNumber {
		r.fail(key, "expected an integer")
		return 0
	}
	i, err := acctParseIntLiteral(string(n))
	if err != nil {
		r.fail(key, "expected an integer")
	}
	return i
}

func (r *acctReader) object(key string) (map[string]any, bool) {
	v, ok := r.value(key)
	if !ok {
		return nil, false
	}
	o, isObject := v.(map[string]any)
	if !isObject {
		r.fail(key, "expected an object")
	}
	return o, isObject
}

// rest returns the keys no accessor asked for: the fields to carry through.
func (r *acctReader) rest() map[string]any {
	var extra map[string]any
	for k, v := range r.obj {
		if _, ok := r.used[k]; !ok {
			if extra == nil {
				extra = map[string]any{}
			}
			extra[k] = v
		}
	}
	return extra
}

// acctParseIntLiteral accepts what Swift's JSONDecoder accepts for an Int: an
// integer literal, or a float literal with an exact integer value.
func acctParseIntLiteral(s string) (int, error) {
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return int(n), nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f != math.Trunc(f) || math.Abs(f) > 1<<53 {
		return 0, errors.New("not an integer")
	}
	return int(f), nil
}

// acctNode returns a copy of extra as the start of an object being written.
func acctNode(extra map[string]any) map[string]any {
	o := make(map[string]any, len(extra)+8)
	for k, v := range extra {
		o[k] = v
	}
	return o
}

func acctInt(n int) json.Number { return json.Number(strconv.Itoa(n)) }
