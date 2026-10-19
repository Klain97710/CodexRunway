package skills

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

var errGrokConfig = errors.New("skills: invalid Grok config")

type grokConfig struct {
	Skills struct {
		Paths    []string `toml:"paths"`
		Ignore   []string `toml:"ignore"`
		Disabled []string `toml:"disabled"`
	} `toml:"skills"`
	Compat struct {
		Claude struct {
			Skills *bool `toml:"skills"`
		} `toml:"claude"`
		Cursor struct {
			Skills *bool `toml:"skills"`
		} `toml:"cursor"`
	} `toml:"compat"`
	Plugins struct {
		Enabled  []string `toml:"enabled"`
		Disabled []string `toml:"disabled"`
	} `toml:"plugins"`
}

func readGrokConfig(path string) (grokConfig, error) {
	var config grokConfig
	_, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return config, nil
	}
	if err != nil {
		return config, err
	}
	if _, err = toml.DecodeFile(path, &config); err != nil {
		return config, errGrokConfig
	}
	return config, nil
}

func grokCompatEnabled(value string, configured *bool) bool {
	if value != "" {
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "false", "0", "no", "off":
			return false
		default:
			return true
		}
	}
	return configured == nil || *configured
}

func expandGrokPath(home, path string) string {
	path = strings.TrimSpace(path)
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		return filepath.Join(home, path[2:])
	}
	return path
}

func applyGrokConfig(home string, config grokConfig, skills []Skill) {
	ignored := make([]string, 0, len(config.Skills.Ignore))
	for _, path := range config.Skills.Ignore {
		if abs, err := filepath.Abs(expandGrokPath(home, path)); err == nil {
			ignored = append(ignored, filepath.Clean(abs))
		}
	}
	for i := range skills {
		skill := &skills[i]
		if skill.State != StateNotLoaded && stringInSlice(config.Skills.Disabled, skill.Name) {
			skill.State, skill.Reason = StateOff, ReasonDisabledList
		}
		if grokPathUnderAny(skill.File, ignored) || grokPathUnderAny(skill.RealFile, ignored) {
			if skill.State != StateNotLoaded {
				skill.State, skill.Reason = StateOff, ReasonIgnored
			}
			skill.Toggleable = false
			skill.ToggleBlock = ReasonIgnored
		}
	}
}

func grokPathUnderAny(path string, prefixes []string) bool {
	if path == "" {
		return false
	}
	path = filepath.Clean(path)
	for _, prefix := range prefixes {
		rel, err := filepath.Rel(prefix, path)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func stringInSlice(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
