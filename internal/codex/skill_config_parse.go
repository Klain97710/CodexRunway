package codex

import "github.com/licoy/quosparr/internal/tomlstmt"

func parseSkillConfig(data []byte, config map[string]any) ([]tomlstmt.Item, []skillConfigBlock, error) {
	items, err := tomlstmt.Lex(data)
	if err != nil {
		return nil, nil, ErrSkillConfigUnsupported
	}
	starts, err := skillBlockStarts(items)
	if err != nil {
		return nil, nil, err
	}
	entries, err := skillEntries(config)
	if err != nil || len(starts) != len(entries) {
		return nil, nil, ErrSkillConfigUnsupported
	}
	blocks := make([]skillConfigBlock, len(starts))
	for i, start := range starts {
		blocks[i] = skillConfigBlock{start: start, end: tomlstmt.TableEnd(items, start), entry: entries[i]}
		if !validSkillEntry(entries[i]) {
			return nil, nil, ErrSkillConfigUnsupported
		}
	}
	return items, blocks, nil
}

func skillBlockStarts(items []tomlstmt.Item) ([]int, error) {
	var starts []int
	var table []string
	for i, item := range items {
		switch item.Kind {
		case tomlstmt.Header:
			table = item.Path
			if err := validateSkillHeader(item); err != nil {
				return nil, err
			}
			if item.Array && pathIs(item.Path, "skills", "config") {
				starts = append(starts, i)
			}
		case tomlstmt.KeyVal:
			full := append(append([]string(nil), table...), item.Path...)
			if invalidSkillKey(table, full) {
				return nil, ErrSkillConfigUnsupported
			}
		}
	}
	return starts, nil
}

func validateSkillHeader(item tomlstmt.Item) error {
	path := item.Path
	if item.Array && pathIs(path, "skills") || pathHasPrefix(path, "skills", "config") && (!item.Array || len(path) != 2) {
		return ErrSkillConfigUnsupported
	}
	return nil
}

func invalidSkillKey(table, full []string) bool {
	if pathIs(table, "skills", "config") {
		return false
	}
	if len(table) == 0 && len(full) == 1 && full[0] == "skills" {
		return true
	}
	if pathHasPrefix(full, "skills", "config") {
		return true
	}
	return pathIs(table, "skills") && len(full) == 2 && full[1] == "config"
}

func skillEntries(config map[string]any) ([]map[string]any, error) {
	skills, ok := config["skills"].(map[string]any)
	if !ok {
		if _, exists := config["skills"]; exists {
			return nil, ErrSkillConfigUnsupported
		}
		return nil, nil
	}
	raw, exists := skills["config"]
	if !exists {
		return nil, nil
	}
	entries, ok := raw.([]map[string]any)
	if !ok {
		return nil, ErrSkillConfigUnsupported
	}
	return entries, nil
}

func validSkillEntry(entry map[string]any) bool {
	if value, exists := entry["enabled"]; exists {
		_, ok := value.(bool)
		return ok
	}
	return true
}

func pathIs(path []string, want ...string) bool {
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

func pathHasPrefix(path []string, want ...string) bool {
	return len(path) >= len(want) && pathIs(path[:len(want)], want...)
}
