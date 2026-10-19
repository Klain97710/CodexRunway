package grok

import (
	"bytes"
	"errors"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/BurntSushi/toml"

	"github.com/licoy/quosparr/internal/platform"
	"github.com/licoy/quosparr/internal/tomlstmt"
)

var (
	ErrSkillConfigUnsupported = errors.New("grok: config.toml defines skills.disabled in a form that cannot be edited safely; adjust it manually")
	ErrSkillConfigInvalid     = errors.New("grok: config.toml cannot be parsed or edited safely")
	ErrSkillNameInvalid       = errors.New("grok: skill name is invalid")
)

type skillLayout struct {
	header, disabled, lastKey int
}

// SetSkillDisabled updates [skills].disabled while preserving unrelated bytes.
func SetSkillDisabled(data []byte, name string, disabled bool) ([]byte, error) {
	if !utf8.ValidString(name) || strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return nil, ErrSkillNameInvalid
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrSkillNameInvalid
	}

	before, err := decodeSkillConfig(data)
	if err != nil {
		return nil, err
	}
	items, err := tomlstmt.Lex(data)
	if err != nil {
		return nil, ErrSkillConfigUnsupported
	}
	layout, err := analyzeSkillLayout(items)
	if err != nil {
		return nil, err
	}
	names, present, err := disabledSkillNames(before)
	if err != nil {
		return nil, err
	}

	want := append([]string(nil), names...)
	if disabled {
		if containsSkill(want, name) {
			return data, nil
		}
		want = append(want, name)
	} else {
		if !present || !containsSkill(want, name) {
			return data, nil
		}
		want = removeSkill(want, name)
	}

	out := editDisabledSkills(data, items, layout, want)
	after, err := decodeSkillConfig(out)
	if err != nil || !skillEditExpected(before, after, want) {
		return nil, ErrSkillConfigInvalid
	}
	return out, nil
}

func decodeSkillConfig(data []byte) (map[string]any, error) {
	config := map[string]any{}
	if err := toml.Unmarshal(data, &config); err != nil {
		if items, lexErr := tomlstmt.Lex(data); lexErr == nil && hasDuplicateSkillsTable(items) {
			return nil, ErrSkillConfigUnsupported
		}
		return nil, ErrSkillConfigInvalid
	}
	return config, nil
}

func disabledSkillNames(config map[string]any) ([]string, bool, error) {
	value, ok := config["skills"]
	if !ok {
		return nil, false, nil
	}
	table, ok := value.(map[string]any)
	if !ok {
		return nil, false, ErrSkillConfigUnsupported
	}
	value, ok = table["disabled"]
	if !ok {
		return nil, false, nil
	}
	values, ok := value.([]any)
	if !ok {
		return nil, false, ErrSkillConfigUnsupported
	}
	names := make([]string, len(values))
	for i, value := range values {
		name, ok := value.(string)
		if !ok {
			return nil, false, ErrSkillConfigUnsupported
		}
		names[i] = name
	}
	return names, true, nil
}

func analyzeSkillLayout(items []tomlstmt.Item) (skillLayout, error) {
	layout := skillLayout{header: -1, disabled: -1, lastKey: -1}
	var table []string
	for i, item := range items {
		switch item.Kind {
		case tomlstmt.Header:
			table = item.Path
			if len(table) >= 2 && table[0] == "skills" && table[1] == "disabled" {
				return layout, ErrSkillConfigUnsupported
			}
			if samePath(table, "skills") {
				if item.Array || layout.header >= 0 {
					return layout, ErrSkillConfigUnsupported
				}
				layout.header = i
			}
		case tomlstmt.KeyVal:
			full := append(append([]string(nil), table...), item.Path...)
			if len(table) == 0 && len(item.Path) > 0 && item.Path[0] == "skills" {
				return layout, ErrSkillConfigUnsupported
			}
			if samePath(table, "skills") {
				layout.lastKey = i
			}
			if samePath(full, "skills", "disabled") && samePath(table, "skills") && samePath(item.Path, "disabled") {
				layout.disabled = i
				continue
			}
			if len(full) >= 2 && full[0] == "skills" && full[1] == "disabled" {
				return layout, ErrSkillConfigUnsupported
			}
		}
	}
	return layout, nil
}

func samePath(path []string, want ...string) bool {
	if len(path) != len(want) {
		return false
	}
	for i := range want {
		if path[i] != want[i] {
			return false
		}
	}
	return true
}

func containsSkill(names []string, name string) bool {
	for _, existing := range names {
		if existing == name {
			return true
		}
	}
	return false
}

func removeSkill(names []string, name string) []string {
	out := make([]string, 0, len(names))
	for _, existing := range names {
		if existing != name {
			out = append(out, existing)
		}
	}
	return out
}

func editDisabledSkills(data []byte, items []tomlstmt.Item, layout skillLayout, names []string) []byte {
	if len(names) == 0 {
		if removableEmptySkillsTable(data, items, layout) {
			start := items[layout.header].Start
			if layout.header > 0 && items[layout.header-1].Kind == tomlstmt.Blank {
				start = items[layout.header-1].Start
			}
			return tomlstmt.ApplyEdits(data, []tomlstmt.Edit{{Start: start, End: len(data)}})
		}
		item := items[layout.disabled]
		return tomlstmt.ApplyEdits(data, []tomlstmt.Edit{{Start: item.Start, End: item.End}})
	}

	eol := tomlstmt.DetectEOL(data)
	line := disabledLine(names)
	if layout.disabled >= 0 {
		item := items[layout.disabled]
		if item.End > item.Start && data[item.End-1] == '\n' {
			line += eol
		}
		return tomlstmt.ApplyEdits(data, []tomlstmt.Edit{{Start: item.Start, End: item.End, Text: line}})
	}
	if layout.header >= 0 {
		at := layout.header
		if layout.lastKey >= 0 {
			at = layout.lastKey
		}
		item := items[at]
		text := line + eol
		if item.End == item.Start || data[item.End-1] != '\n' {
			text = eol + text
		}
		return tomlstmt.ApplyEdits(data, []tomlstmt.Edit{{Start: item.End, End: item.End, Text: text}})
	}

	text := "[skills]" + eol + line + eol
	out := append([]byte(nil), data...)
	if len(out) > 0 {
		if out[len(out)-1] != '\n' {
			out = append(out, eol...)
		}
		if !tomlstmt.EndsWithBlankLine(out) {
			out = append(out, eol...)
		}
	}
	return append(out, text...)
}

func disabledLine(names []string) string {
	quoted := make([]string, len(names))
	for i, name := range names {
		quoted[i] = tomlstmt.Quote(name)
	}
	return "disabled = [" + strings.Join(quoted, ", ") + "]"
}

func removableEmptySkillsTable(data []byte, items []tomlstmt.Item, layout skillLayout) bool {
	if layout.header < 0 || layout.disabled < 0 {
		return false
	}
	header := items[layout.header]
	if bytes.Contains(data[header.Start:header.End], []byte("#")) {
		return false
	}
	for i := layout.header + 1; i < len(items); i++ {
		if i == layout.disabled || items[i].Kind == tomlstmt.Blank {
			continue
		}
		return false
	}
	return true
}

func skillEditExpected(before, after map[string]any, want []string) bool {
	got, present, err := disabledSkillNames(after)
	if err != nil || present != (len(want) > 0) || !slices.Equal(got, want) {
		return false
	}
	return reflect.DeepEqual(withoutDisabledSkills(before), withoutDisabledSkills(after))
}

func withoutDisabledSkills(config map[string]any) map[string]any {
	out := make(map[string]any, len(config))
	for key, value := range config {
		out[key] = value
	}
	if table, ok := config["skills"].(map[string]any); ok {
		copyTable := make(map[string]any, len(table))
		for key, value := range table {
			if key != "disabled" {
				copyTable[key] = value
			}
		}
		if len(copyTable) == 0 {
			delete(out, "skills")
		} else {
			out["skills"] = copyTable
		}
	}
	return out
}

// SetSkillEnabledFile changes config.toml using a backup and an atomic write.
func (s *Store) SetSkillEnabledFile(name string, enabled bool, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	path := filepath.Join(s.OfficialHome(), "config.toml")
	backup := platform.BackupPolicy{
		Dir:    filepath.Join(filepath.Dir(s.root), "backups"),
		Prefix: "grok-config-",
		Ext:    ".toml",
		Keep:   10,
	}
	return platform.EditFileWithBackup(path, backup, now, func(old []byte) ([]byte, error) {
		return SetSkillDisabled(old, name, !enabled)
	})
}
