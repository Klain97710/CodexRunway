package codex

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"time"
	"unicode"

	"github.com/licoy/quosparr/internal/platform"
	"github.com/licoy/quosparr/internal/tomlstmt"
)

var (
	// ErrSkillConfigUnsupported means skills.config cannot be edited safely.
	ErrSkillConfigUnsupported = errors.New("codex: config.toml defines skills.config in a form that cannot be edited safely; adjust it manually")
	// ErrSkillConfigTarget means the requested skill identity is invalid.
	ErrSkillConfigTarget = errors.New("codex: skill config target must be an absolute SKILL.md path and nonempty name")
)

type skillConfigBlock struct {
	start, end int
	entry      map[string]any
}

// SetSkillConfigEnabled edits ordered [[skills.config]] rules for one skill.
func SetSkillConfigEnabled(data []byte, skillFile, name string, enabled bool) ([]byte, error) {
	if strings.ContainsFunc(name, unicode.IsControl) {
		return nil, ErrSkillConfigTarget
	}
	name = strings.TrimSpace(name)
	if !validSkillTarget(skillFile, name) {
		return nil, ErrSkillConfigTarget
	}
	before, err := decodeConfig(data)
	if err != nil {
		return nil, err
	}
	items, blocks, err := parseSkillConfig(data, before)
	if err != nil {
		return nil, err
	}
	edits, pathMatched, err := skillConfigEdits(data, items, blocks, skillFile, enabled)
	if err != nil {
		return nil, err
	}
	out := tomlstmt.ApplyEdits(data, edits)
	if !pathMatched && !enabled {
		out = appendSkillRule(out, skillFile, false)
	}
	state, err := effectiveSkillState(out, skillFile, name)
	if err != nil {
		return nil, err
	}
	if state != enabled {
		out = appendSkillRule(out, skillFile, enabled)
	}
	if err := validateSkillEdit(before, blocks, out, skillFile, name, enabled); err != nil {
		return nil, err
	}
	return out, nil
}

func validSkillTarget(skillFile, name string) bool {
	if !filepath.IsAbs(skillFile) || filepath.Base(skillFile) != "SKILL.md" || name == "" {
		return false
	}
	return true
}

func skillConfigEdits(data []byte, items []tomlstmt.Item, blocks []skillConfigBlock, skillFile string, enabled bool) ([]tomlstmt.Edit, bool, error) {
	var edits []tomlstmt.Edit
	matched := false
	for _, block := range blocks {
		path, _ := block.entry["path"].(string)
		if !matchesSkillPath(path, skillFile) {
			continue
		}
		matched = true
		if blockHasMultiline(items, block) {
			return nil, false, ErrSkillConfigUnsupported
		}
		edit, ok := editSkillBlock(data, items, block, enabled)
		if ok {
			edits = append(edits, edit)
		}
	}
	return edits, matched, nil
}

func editSkillBlock(data []byte, items []tomlstmt.Item, block skillConfigBlock, enabled bool) (tomlstmt.Edit, bool) {
	_, hasEnabled := block.entry["enabled"]
	pure := len(block.entry) == 1 || len(block.entry) == 2 && hasEnabled
	if enabled && pure {
		start := items[block.start].Start
		if block.end == len(items) && block.start > 0 && items[block.start-1].Kind == tomlstmt.Blank {
			start = items[block.start-1].Start
		}
		return tomlstmt.Edit{Start: start, End: items[block.end-1].End}, true
	}
	for i := block.start + 1; i < block.end; i++ {
		if items[i].Kind == tomlstmt.KeyVal && equalPath(items[i].Path, "enabled") {
			text := "enabled = " + boolText(enabled)
			if data[items[i].End-1] == '\n' {
				text += tomlstmt.DetectEOL(data)
			}
			return tomlstmt.Edit{Start: items[i].Start, End: items[i].End, Text: text}, string(data[items[i].Start:items[i].End]) != text
		}
	}
	pos := items[block.end-1].End
	prefix := ""
	if pos > 0 && data[pos-1] != '\n' {
		prefix = tomlstmt.DetectEOL(data)
	}
	return tomlstmt.Edit{Start: pos, End: pos, Text: prefix + "enabled = " + boolText(enabled) + tomlstmt.DetectEOL(data)}, true
}

func appendSkillRule(data []byte, skillFile string, enabled bool) []byte {
	eol := tomlstmt.DetectEOL(data)
	out := append([]byte(nil), data...)
	if len(out) > 0 && out[len(out)-1] != '\n' {
		out = append(out, eol...)
	}
	if len(out) > 0 && !tomlstmt.EndsWithBlankLine(out) {
		out = append(out, eol...)
	}
	text := "[[skills.config]]" + eol + "path = " + tomlstmt.Quote(skillFile) + eol + "enabled = " + boolText(enabled) + eol
	return append(out, text...)
}

func effectiveSkillState(data []byte, skillFile, name string) (bool, error) {
	config, err := decodeConfig(data)
	if err != nil {
		return false, err
	}
	_, blocks, err := parseSkillConfig(data, config)
	if err != nil {
		return false, err
	}
	enabled := true
	for _, block := range blocks {
		path, hasPath := block.entry["path"].(string)
		ruleName, hasName := block.entry["name"].(string)
		value, hasEnabled := block.entry["enabled"].(bool)
		if hasPath == hasName || !hasEnabled {
			continue
		}
		if hasPath && matchesSkillPath(path, skillFile) || hasName && strings.TrimSpace(ruleName) == name {
			enabled = value
		}
	}
	return enabled, nil
}

func validateSkillEdit(before map[string]any, blocks []skillConfigBlock, out []byte, skillFile, name string, enabled bool) error {
	after, err := decodeConfig(out)
	if err != nil || !reflect.DeepEqual(withoutSkillConfig(before), withoutSkillConfig(after)) {
		return ErrProviderConfigInvalid
	}
	entries, err := skillEntries(after)
	if err != nil || !reflect.DeepEqual(entries, expectedSkillEntries(blocks, skillFile, name, enabled)) {
		return ErrProviderConfigInvalid
	}
	state, err := effectiveSkillState(out, skillFile, name)
	if err != nil || state != enabled {
		return ErrProviderConfigInvalid
	}
	return nil
}

func expectedSkillEntries(blocks []skillConfigBlock, skillFile, name string, enabled bool) []map[string]any {
	var entries []map[string]any
	pathMatched := false
	for _, block := range blocks {
		entry := cloneSkillEntry(block.entry)
		path, _ := entry["path"].(string)
		if !matchesSkillPath(path, skillFile) {
			entries = append(entries, entry)
			continue
		}
		pathMatched = true
		_, hasEnabled := entry["enabled"]
		if enabled && (len(entry) == 1 || len(entry) == 2 && hasEnabled) {
			continue
		}
		entry["enabled"] = enabled
		entries = append(entries, entry)
	}
	if !pathMatched && !enabled {
		entries = append(entries, skillPathEntry(skillFile, false))
	}
	if skillEntriesState(entries, skillFile, name) != enabled {
		entries = append(entries, skillPathEntry(skillFile, enabled))
	}
	return entries
}

func skillEntriesState(entries []map[string]any, skillFile, name string) bool {
	state := true
	for _, entry := range entries {
		path, hasPath := entry["path"].(string)
		ruleName, hasName := entry["name"].(string)
		value, ok := entry["enabled"].(bool)
		if ok && hasPath != hasName && (hasPath && matchesSkillPath(path, skillFile) || hasName && strings.TrimSpace(ruleName) == name) {
			state = value
		}
	}
	return state
}

func cloneSkillEntry(entry map[string]any) map[string]any {
	out := make(map[string]any, len(entry)+1)
	for key, value := range entry {
		out[key] = value
	}
	return out
}

func skillPathEntry(path string, enabled bool) map[string]any {
	return map[string]any{"path": path, "enabled": enabled}
}

func withoutSkillConfig(config map[string]any) map[string]any {
	out := make(map[string]any, len(config))
	for key, value := range config {
		out[key] = value
	}
	if skills, ok := config["skills"].(map[string]any); ok {
		copySkills := make(map[string]any, len(skills))
		for key, value := range skills {
			if key != "config" {
				copySkills[key] = value
			}
		}
		if len(copySkills) == 0 {
			delete(out, "skills")
		} else {
			out["skills"] = copySkills
		}
	}
	return out
}

// SetSkillEnabledFile edits config.toml under the Store lock with backup.
func (s *Store) SetSkillEnabledFile(skillFile, name string, enabled bool, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return platform.EditFileWithBackup(s.ConfigPath(), s.configBackup(), now, func(old []byte) ([]byte, error) {
		return SetSkillConfigEnabled(old, skillFile, name, enabled)
	})
}

func boolText(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func matchesSkillPath(path, skillFile string) bool {
	clean := filepath.Clean(path)
	return clean == filepath.Clean(skillFile) || clean == filepath.Clean(filepath.Dir(skillFile))
}

func equalPath(path []string, want string) bool { return len(path) == 1 && path[0] == want }

func blockHasMultiline(items []tomlstmt.Item, block skillConfigBlock) bool {
	for i := block.start; i < block.end; i++ {
		if items[i].MLString {
			return true
		}
	}
	return false
}
