package skills

import (
	"path/filepath"
	"strings"
)

func (c *originPluginCache) codexCandidate(skill Skill, root string, candidate originCandidate) originCandidate {
	cache := cleanEvaluatedPath(filepath.Join(c.env.CodexHome, "plugins", "cache"))
	rel, err := filepath.Rel(cache, root)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return candidate
	}
	parts := strings.Split(filepath.Clean(rel), string(filepath.Separator))
	if len(parts) < 3 {
		return candidate
	}
	candidate.from = OriginPlugin
	candidate.installer = parts[0]
	return candidate
}
