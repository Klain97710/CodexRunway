package sessions

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// IndexFileName is Codex's session index inside the Codex home.
const IndexFileName = "session_index.jsonl"

// untitled is the title of a session without a user message.
const untitled = "Untitled"

// indexEntry is one line of session_index.jsonl.
type indexEntry struct {
	ID         string
	ThreadName string
	UpdatedAt  time.Time
}

// decodeIndexLine is SessionIndexEntry.init(from:): "id" and "updated_at" are
// required strings, "thread_name" defaults to "Untitled", and an unparsable
// date becomes the zero time. Any other shape drops the line.
func decodeIndexLine(line string) (indexEntry, bool) {
	var raw struct {
		ID         *string          `json:"id"`
		ThreadName *json.RawMessage `json:"thread_name"`
		UpdatedAt  *string          `json:"updated_at"`
	}
	if err := json.Unmarshal([]byte(line), &raw); err != nil || raw.ID == nil || raw.UpdatedAt == nil {
		return indexEntry{}, false
	}
	entry := indexEntry{ID: *raw.ID, ThreadName: untitled, UpdatedAt: parseDate(*raw.UpdatedAt)}
	if raw.ThreadName != nil && string(*raw.ThreadName) != "null" {
		var name string
		if json.Unmarshal(*raw.ThreadName, &name) != nil {
			return indexEntry{}, false
		}
		entry.ThreadName = name
	}
	return entry, true
}

// parseDate is RunwayDates.parse; an unparsable value is the zero time
// (Swift's distantPast).
func parseDate(s string) time.Time {
	t, ok := parseTimestamp(s)
	if !ok {
		return time.Time{}
	}
	return t
}

func parseTimestamp(s string) (time.Time, bool) {
	t, err := time.Parse(time.RFC3339Nano, s)
	return t, err == nil
}

// readIndex reads session_index.jsonl; a missing file is an empty index, and a
// line that does not decode is skipped.
func readIndex(codexHome string) ([]indexEntry, error) {
	data, err := os.ReadFile(filepath.Join(codexHome, IndexFileName))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var entries []indexEntry
	for _, line := range splitLines(data) {
		if entry, ok := decodeIndexLine(line); ok {
			entries = append(entries, entry)
		}
	}
	return entries, nil
}

// encodeIndexLine writes {"id":…,"thread_name":…,"updated_at":…} with whole
// second timestamps. The key order is the order Codex itself writes. (The
// Swift app's encoder left the order to chance, so no byte layout is shared.)
func encodeIndexLine(e indexEntry) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	buf.WriteString(`{"id":`)
	if err := enc.Encode(e.ID); err != nil {
		return nil, err
	}
	buf.Truncate(buf.Len() - 1) // Encode appends "\n"
	buf.WriteString(`,"thread_name":`)
	if err := enc.Encode(e.ThreadName); err != nil {
		return nil, err
	}
	buf.Truncate(buf.Len() - 1)
	buf.WriteString(`,"updated_at":"`)
	buf.WriteString(e.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"))
	buf.WriteString(`"}`)
	return buf.Bytes(), nil
}

func joinLines(lines [][]byte) []byte {
	return append(bytes.Join(lines, []byte("\n")), '\n')
}

// sessionText reads the text of a message "content": a string, or a list of
// objects whose "text" strings are joined with a space. Any other shape has no
// text.
func sessionText(value any) (string, bool) {
	switch v := value.(type) {
	case string:
		return v, true
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			obj, ok := item.(map[string]any)
			if !ok {
				return "", false
			}
			if text, ok := obj["text"].(string); ok {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, " "), true
	}
	return "", false
}

// decodeLine parses one JSONL line as a JSON object; numbers stay json.Number.
func decodeLine(line string) (map[string]any, bool) {
	dec := json.NewDecoder(strings.NewReader(line))
	dec.UseNumber()
	var obj map[string]any
	if err := dec.Decode(&obj); err != nil || obj == nil {
		return nil, false
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, false // trailing data after the object
	}
	return obj, true
}

func objectOf(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func stringOf(m map[string]any, key string) (string, bool) {
	s, ok := m[key].(string)
	return s, ok
}

// userTitle returns the cleaned title of a user message payload, or "".
func userTitle(payload map[string]any) string {
	if t, _ := stringOf(payload, "type"); t != "message" {
		return ""
	}
	if r, _ := stringOf(payload, "role"); r != "user" {
		return ""
	}
	text, ok := sessionText(payload["content"])
	if !ok {
		return ""
	}
	return cleanTitle(text)
}
