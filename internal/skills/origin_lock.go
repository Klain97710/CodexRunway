package skills

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"time"
)

type originLockCache struct {
	env     Env
	loaded  bool
	entries map[string]originLockEntry
}

type originLockFile struct {
	Skills map[string]originLockEntry `json:"skills"`
}

type originLockEntry struct {
	Source      string `json:"source"`
	SourceType  string `json:"sourceType"`
	SourceURL   string `json:"sourceUrl"`
	SkillPath   string `json:"skillPath"`
	Ref         string `json:"ref"`
	Version     string `json:"version"`
	InstalledAt string `json:"installedAt"`
	UpdatedAt   string `json:"updatedAt"`
}

func (c *originLockCache) candidate(skill Skill) originCandidate {
	if skill.Scope != ScopeUser {
		return originCandidate{}
	}
	c.load()
	// The skills CLI keys installs by folder, a direct child of
	// ~/.agents/skills or of an agent's skills folder; a skill elsewhere
	// that only shares the name is not one of them.
	var entry originLockEntry
	var ok bool
	if filepath.Dir(skill.Dir) == filepath.Clean(skill.Root) {
		entry, ok = c.entries[filepath.Base(skill.Dir)]
	}
	if real := filepath.Dir(skill.RealFile); !ok && skill.RealFile != "" && filepath.Dir(real) == filepath.Join(c.env.Home, ".agents", "skills") {
		entry, ok = c.entries[filepath.Base(real)]
	}
	if !ok {
		return originCandidate{}
	}
	return lockCandidate(entry)
}

func (c *originLockCache) load() {
	if c.loaded {
		return
	}
	c.loaded = true
	c.entries = make(map[string]originLockEntry)
	c.merge(filepath.Join(c.env.Home, ".agents", ".skill-lock.json"))
	if state := strings.TrimSpace(c.env.Getenv("XDG_STATE_HOME")); state != "" {
		c.merge(filepath.Join(state, "skills", ".skill-lock.json"))
	}
}

func (c *originLockCache) merge(path string) {
	data, err := readOriginManifest(path)
	if err != nil {
		return
	}
	var lock originLockFile
	if json.Unmarshal(data, &lock) != nil {
		return
	}
	for key, entry := range lock.Skills {
		c.entries[key] = entry
	}
}

func lockCandidate(entry originLockEntry) originCandidate {
	raw := strings.TrimSpace(entry.SourceURL)
	if _, _, ok := NormalizeRepoURL(raw); !ok && strings.EqualFold(strings.TrimSpace(entry.SourceType), "github") {
		source := strings.Trim(strings.TrimSpace(entry.Source), "/")
		if !strings.Contains(source, "..") {
			raw = "https://github.com/" + source
		}
	}
	candidate := originCandidate{from: OriginLock, ref: strings.TrimSpace(entry.Ref), branch: strings.TrimSpace(entry.Ref),
		version: strings.TrimSpace(entry.Version), installer: "skills",
		installedAt: parseOriginTime(entry.InstalledAt), updatedAt: parseOriginTime(entry.UpdatedAt)}
	if repo, _, ok := NormalizeRepoURL(raw); ok {
		candidate.repo = repo
	}
	rawSkillPath := filepath.ToSlash(strings.TrimSpace(entry.SkillPath))
	if rawSkillPath == "SKILL.md" {
		rawSkillPath = ""
	} else {
		rawSkillPath = strings.TrimSuffix(rawSkillPath, "/SKILL.md")
	}
	if skillPath := cleanOriginSkillPath(rawSkillPath); skillPath != "" {
		candidate.skillPath = skillPath
	}
	return candidate
}

func parseOriginTime(raw string) time.Time {
	value, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(raw))
	if err != nil {
		return time.Time{}
	}
	return value
}
