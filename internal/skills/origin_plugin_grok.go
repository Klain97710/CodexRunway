package skills

import (
	"encoding/json"
	"path/filepath"
	"strings"
)

type cachedGrokRegistry struct {
	loaded   bool
	registry grokPluginRegistry
	ok       bool
}

func (c *originPluginCache) grokCandidate(skill Skill, root string, candidate originCandidate) originCandidate {
	registry, ok := c.grokRegistry()
	if !ok {
		return candidate
	}
	base := filepath.Join(c.env.GrokHome, "installed-plugins")
	skillDir := cleanEvaluatedPath(filepath.Dir(skill.RealFile))
	for repoID, repo := range registry.Repos {
		repoPath, ok := resolveGrokRepoPath(base, repo.Path)
		if !ok {
			continue
		}
		for name, plugin := range repo.Plugins {
			pluginRoot := cleanEvaluatedPath(filepath.Join(repoPath, plugin.Subdir))
			if name != skill.Plugin || !within(pluginRoot, skillDir) {
				continue
			}
			candidate.from = OriginPlugin
			candidate.installer = strings.TrimSpace(repo.Marketplace.SourceDisplayName)
			if candidate.installer == "" {
				candidate.installer = repoID
			}
			if plugin.Version != "" {
				candidate.version = strings.TrimSpace(plugin.Version)
			}
			if strings.EqualFold(strings.TrimSpace(repo.Kind.Type), "git") {
				if normalized, _, valid := NormalizeRepoURL(repo.Kind.URL); valid {
					candidate.repo = normalized
				}
				gitRef := strings.TrimSpace(repo.Kind.GitRef)
				candidate.ref = gitRef
				if candidate.ref == "" {
					candidate.ref = strings.TrimSpace(repo.Kind.Commit)
				}
				if gitRef != "" && !isGitHash(gitRef) {
					candidate.branch = gitRef
				}
			}
			candidate.skillPath = joinOriginPath(plugin.Subdir, relativeOriginPath(pluginRoot, skillDir))
			candidate.installedAt = parseOriginTime(repo.InstalledAt)
			candidate.updatedAt = parseOriginTime(repo.UpdatedAt)
			return candidate
		}
	}
	return candidate
}

func (c *originPluginCache) grokRegistry() (grokPluginRegistry, bool) {
	if c.grok.loaded {
		return c.grok.registry, c.grok.ok
	}
	c.grok.loaded = true
	path := filepath.Join(c.env.GrokHome, "installed-plugins", "registry.json")
	data, err := readOriginManifest(path)
	if err == nil && json.Unmarshal(data, &c.grok.registry) == nil && c.grok.registry.Version == 1 {
		c.grok.ok = true
	}
	return c.grok.registry, c.grok.ok
}

func isGitHash(value string) bool {
	if len(value) < 7 || len(value) > 40 {
		return false
	}
	for _, char := range value {
		if char >= '0' && char <= '9' || char >= 'a' && char <= 'f' || char >= 'A' && char <= 'F' {
			continue
		}
		return false
	}
	return true
}

func resolveGrokRepoPath(base, raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	if filepath.IsAbs(raw) {
		return filepath.Clean(raw), true
	}
	return filepath.Clean(filepath.Join(base, raw)), true
}
