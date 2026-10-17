package skills

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type codexConfig struct {
	Skills struct {
		Config  []codexSkillConfig `toml:"config"`
		Bundled struct {
			Enabled *bool `toml:"enabled"`
		} `toml:"bundled"`
	} `toml:"skills"`
	Plugins map[string]struct {
		Enabled *bool `toml:"enabled"`
	} `toml:"plugins"`
}

type codexSkillConfig struct {
	Path    string `toml:"path"`
	Name    string `toml:"name"`
	Enabled *bool  `toml:"enabled"`
}

func readCodexConfig(path string) codexConfig {
	data, err := os.ReadFile(path)
	if err != nil {
		return codexConfig{}
	}
	var config codexConfig
	if toml.Unmarshal(data, &config) != nil {
		return codexConfig{}
	}
	return config
}

func applyCodexConfig(skills []Skill, config codexConfig) {
	for i := range skills {
		skill := &skills[i]
		matched, enabled, reason := codexSkillRule(*skill, config.Skills.Config)
		if matched && isCodexLoadable(*skill) {
			setCodexState(skill, enabled, reason)
		}
		if skill.Scope == ScopeSystem && isCodexLoadable(*skill) && config.Skills.Bundled.Enabled != nil && !*config.Skills.Bundled.Enabled {
			setCodexState(skill, false, ReasonBundledOff)
		}
	}
}

func codexSkillRule(skill Skill, rules []codexSkillConfig) (bool, bool, Reason) {
	matched, enabled, reason := false, true, ReasonNone
	for _, rule := range rules {
		path := strings.TrimSpace(rule.Path)
		name := strings.TrimSpace(rule.Name)
		if rule.Enabled == nil || (path == "") == (name == "") {
			continue
		}
		if path != "" && canonicalPath(path) == canonicalPath(skill.File) {
			matched, enabled, reason = true, *rule.Enabled, ReasonConfigPath
		}
		if name != "" && name == skill.InvokeName {
			matched, enabled, reason = true, *rule.Enabled, ReasonConfigName
		}
	}
	return matched, enabled, reason
}

func isCodexLoadable(skill Skill) bool {
	return skill.State != StateNotLoaded && skill.Reason != ReasonInvalidFront && skill.Reason != ReasonMissingDesc
}

func setCodexState(skill *Skill, enabled bool, reason Reason) {
	if enabled {
		skill.State, skill.Reason = StateOn, ReasonNone
		return
	}
	skill.State, skill.Reason = StateOff, reason
}

func canonicalPath(path string) string {
	path = filepath.Clean(strings.TrimSpace(path))
	if real, err := filepath.EvalSymlinks(path); err == nil {
		return filepath.Clean(real)
	}
	return path
}
