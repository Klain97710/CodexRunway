package codex

import "github.com/licoy/quosparr/internal/tomlstmt"

// A deliberately small TOML statement lexer. It does not interpret values; it
// only finds where each statement starts and ends (honouring strings, comments
// and multi-line arrays or strings) so provider_config.go can edit whole
// statements while leaving every other byte alone. Input is always validated
// by a real TOML parser first and after, so a lexer slip fails closed.

type tomlKind = tomlstmt.Kind

const (
	tomlBlank   = tomlstmt.Blank
	tomlComment = tomlstmt.Comment
	tomlKeyVal  = tomlstmt.KeyVal
	tomlHeader  = tomlstmt.Header
)

type tomlItem struct {
	start, end int // byte range, end includes the line terminator
	kind       tomlKind
	path       []string // key path of a key/value, or table path of a header
	array      bool     // [[header]]
	mlString   bool     // contains a multi-line string
}

func lexTOML(data []byte) ([]tomlItem, error) {
	items, err := tomlstmt.Lex(data)
	if err != nil {
		return nil, err
	}
	out := make([]tomlItem, len(items))
	for i, item := range items {
		out[i] = tomlItem{item.Start, item.End, item.Kind, item.Path, item.Array, item.MLString}
	}
	return out, nil
}
