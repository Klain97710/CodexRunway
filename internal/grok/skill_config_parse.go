package grok

import "github.com/licoy/quosparr/internal/tomlstmt"

func hasDuplicateSkillsTable(items []tomlstmt.Item) bool {
	seen := false
	for _, item := range items {
		if item.Kind == tomlstmt.Header && !item.Array && samePath(item.Path, "skills") {
			if seen {
				return true
			}
			seen = true
		}
	}
	return false
}
