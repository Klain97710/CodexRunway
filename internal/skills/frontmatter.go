package skills

import (
	"bytes"
	"errors"
	"strconv"
	"strings"
)

var errInvalidFrontmatter = errors.New("skills: invalid frontmatter")

type frontmatter struct {
	values   map[string]string
	metadata map[string]string
	nested   map[string]map[string]string
}

func parseFrontmatter(data []byte) (frontmatter, int, error) {
	front := frontmatter{values: make(map[string]string), metadata: make(map[string]string), nested: make(map[string]map[string]string)}
	lines, offsets := splitLines(data)
	if len(lines) == 0 || lines[0] != "---" {
		return front, 0, errInvalidFrontmatter
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if lines[i] == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		return front, 0, errInvalidFrontmatter
	}
	for i := 1; i < end; {
		line := lines[i]
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			i++
			continue
		}
		if leadingSpaces(line) != 0 {
			return front, 0, errInvalidFrontmatter
		}
		key, raw, ok := splitKeyValue(line)
		if !ok || key == "" {
			return front, 0, errInvalidFrontmatter
		}
		if strings.TrimSpace(raw) == "" && i+1 < end && leadingSpaces(lines[i+1]) > 0 && !strings.HasPrefix(strings.TrimSpace(lines[i+1]), "-") {
			mapping := make(map[string]string)
			i++
			for i < end && (strings.TrimSpace(lines[i]) == "" || leadingSpaces(lines[i]) > 0) {
				if strings.TrimSpace(lines[i]) != "" {
					mk, mv, ok := splitKeyValue(strings.TrimSpace(lines[i]))
					if !ok || mk == "" {
						return front, 0, errInvalidFrontmatter
					}
					value, next, err := parseValue(lines, i, end, leadingSpaces(lines[i]), mv)
					if err != nil {
						return front, 0, err
					}
					mapping[mk] = value
					i = next
					continue
				}
				i++
			}
			front.nested[key] = mapping
			if key == "metadata" {
				front.metadata = mapping
			}
			continue
		}
		value, next, err := parseValue(lines, i, end, 0, raw)
		if err != nil {
			return front, 0, err
		}
		front.values[key] = value
		i = next
	}
	bodyStart := offsets[end] + len(lines[end])
	if bodyStart < len(data) && data[bodyStart] == '\r' {
		bodyStart++
	}
	if bodyStart < len(data) && data[bodyStart] == '\n' {
		bodyStart++
	}
	return front, bodyStart, nil
}

func parseValue(lines []string, index, end, indent int, raw string) (string, int, error) {
	raw = strings.TrimSpace(raw)
	if raw == ">" || raw == ">-" || raw == ">+" || raw == "|" || raw == "|-" || raw == "|+" {
		return parseBlock(lines, index+1, end, indent, strings.HasPrefix(raw, ">"))
	}
	if raw == "" {
		items := make([]string, 0)
		i := index + 1
		for i < end && (leadingSpaces(lines[i]) > indent || strings.HasPrefix(strings.TrimSpace(lines[i]), "-")) {
			trimmed := strings.TrimSpace(lines[i])
			if trimmed != "" {
				if !strings.HasPrefix(trimmed, "-") {
					break
				}
				item, err := parseScalar(strings.TrimSpace(strings.TrimPrefix(trimmed, "-")))
				if err != nil {
					return "", 0, err
				}
				items = append(items, item)
			}
			i++
		}
		return strings.Join(items, ", "), i, nil
	}
	value, err := parseScalar(raw)
	return value, index + 1, err
}

func parseBlock(lines []string, index, end, parentIndent int, folded bool) (string, int, error) {
	var values []string
	for index < end {
		line := lines[index]
		if strings.TrimSpace(line) != "" && leadingSpaces(line) <= parentIndent {
			break
		}
		values = append(values, strings.TrimSpace(line))
		index++
	}
	if folded {
		return strings.Join(strings.Fields(strings.Join(values, " ")), " "), index, nil
	}
	return strings.Join(strings.Fields(strings.Join(values, "\n")), " "), index, nil
}

func parseScalar(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "[") {
		if !strings.HasSuffix(raw, "]") {
			return "", errInvalidFrontmatter
		}
		parts, err := splitInlineList(strings.TrimSpace(raw[1 : len(raw)-1]))
		if err != nil {
			return "", err
		}
		return strings.Join(parts, ", "), nil
	}
	if strings.HasPrefix(raw, "\"") {
		quoted, err := quotedPrefix(raw, '"')
		if err != nil {
			return "", err
		}
		value, err := strconv.Unquote(quoted)
		if err != nil {
			return "", errInvalidFrontmatter
		}
		return singleLine(value), nil
	}
	if strings.HasPrefix(raw, "'") {
		quoted, err := quotedPrefix(raw, '\'')
		if err != nil {
			return "", err
		}
		return singleLine(strings.ReplaceAll(quoted[1:len(quoted)-1], "''", "'")), nil
	}
	if strings.ContainsAny(raw, "\r\n") {
		return "", errInvalidFrontmatter
	}
	return singleLine(strings.TrimSpace(stripPlainComment(raw))), nil
}

func splitInlineList(raw string) ([]string, error) {
	if raw == "" {
		return nil, nil
	}
	var parts []string
	start := 0
	quote := byte(0)
	escaped := false
	for i := 0; i < len(raw); i++ {
		switch {
		case escaped:
			escaped = false
		case quote == '"' && raw[i] == '\\':
			escaped = true
		case quote == '\'' && raw[i] == '\'' && i+1 < len(raw) && raw[i+1] == '\'':
			i++
		case quote != 0 && raw[i] == quote:
			quote = 0
		case quote == 0 && (raw[i] == '\'' || raw[i] == '"'):
			quote = raw[i]
		case quote == 0 && raw[i] == ',':
			value, err := parseScalar(raw[start:i])
			if err != nil {
				return nil, err
			}
			parts = append(parts, value)
			start = i + 1
		}
	}
	if quote != 0 {
		return nil, errInvalidFrontmatter
	}
	value, err := parseScalar(raw[start:])
	if err != nil {
		return nil, err
	}
	return append(parts, value), nil
}

func splitLines(data []byte) ([]string, []int) {
	var lines []string
	var offsets []int
	for start := 0; start < len(data); {
		end := bytes.IndexByte(data[start:], '\n')
		if end < 0 {
			end = len(data)
		} else {
			end += start
		}
		line := data[start:end]
		line = bytes.TrimSuffix(line, []byte{'\r'})
		lines = append(lines, string(line))
		offsets = append(offsets, start)
		if end == len(data) {
			break
		}
		start = end + 1
	}
	return lines, offsets
}

func splitKeyValue(line string) (string, string, bool) {
	index := strings.IndexByte(line, ':')
	if index < 0 {
		return "", "", false
	}
	key := strings.TrimSpace(line[:index])
	if strings.ContainsAny(key, " \t[]{}") {
		return "", "", false
	}
	return key, line[index+1:], true
}

func stripPlainComment(value string) string {
	if index := strings.Index(value, " #"); index >= 0 {
		return value[:index]
	}
	return value
}

func quotedPrefix(raw string, quote byte) (string, error) {
	escaped := false
	for i := 1; i < len(raw); i++ {
		if escaped {
			escaped = false
			continue
		}
		if quote == '"' && raw[i] == '\\' {
			escaped = true
			continue
		}
		if quote == '\'' && raw[i] == '\'' && i+1 < len(raw) && raw[i+1] == '\'' {
			i++
			continue
		}
		if raw[i] == quote {
			rest := strings.TrimSpace(raw[i+1:])
			if rest != "" && !strings.HasPrefix(rest, "#") {
				return "", errInvalidFrontmatter
			}
			return raw[:i+1], nil
		}
	}
	return "", errInvalidFrontmatter
}

func singleLine(value string) string { return strings.Join(strings.Fields(value), " ") }

func leadingSpaces(line string) int {
	count := 0
	for count < len(line) && line[count] == ' ' {
		count++
	}
	if count < len(line) && line[count] == '\t' {
		return -1
	}
	return count
}

func (f frontmatter) field(key string) string { return f.values[key] }

func (f frontmatter) displayFields() []Field {
	keys := []string{"allowed-tools", "disallowed-tools", "model", "effort", "argument-hint", "when_to_use", "when-to-use", "user-invocable", "disable-model-invocation", "context", "license", "compatibility"}
	fields := make([]Field, 0, len(keys)+1)
	for _, key := range keys {
		if value, ok := f.values[key]; ok {
			fields = append(fields, Field{Key: key, Value: value})
		}
	}
	if value, ok := f.metadata["author"]; ok {
		fields = append(fields, Field{Key: "metadata.author", Value: value})
	}
	return fields
}
