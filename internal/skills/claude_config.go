package skills

import (
	"encoding/json"
	"os"
	"strings"
)

type claudeSettings struct {
	EnabledPlugins map[string]bool   `json:"enabledPlugins"`
	SkillOverrides map[string]string `json:"skillOverrides"`
	Permissions    struct {
		Deny []string `json:"deny"`
	} `json:"permissions"`
}

func readClaudeSettings(path string) claudeSettings {
	data, err := os.ReadFile(path)
	if err != nil {
		return claudeSettings{}
	}
	var settings claudeSettings
	if json.Unmarshal(data, &settings) != nil {
		return claudeSettings{}
	}
	return settings
}

func applyClaudeOverrides(skills []Skill, settings claudeSettings) {
	denyAll, denied := claudeDeniedSkills(settings.Permissions.Deny)
	for i := range skills {
		skill := &skills[i]
		if isClaudeLoadable(*skill) && skill.Scope != ScopePlugin && skill.Plugin == "" {
			if value, ok := settings.SkillOverrides[skill.Name]; ok {
				switch value {
				case "off":
					skill.State, skill.Reason = StateOff, ReasonOverride
				case "name-only", "user-invocable-only":
					skill.Fields = append(skill.Fields, Field{Key: "skillOverrides", Value: value})
				}
			}
		}
		if isClaudeLoadable(*skill) && (denyAll || denied[skill.InvokeName]) {
			skill.State, skill.Reason = StateOff, ReasonPermissionDeny
			skill.Toggleable = false
			skill.ToggleBlock = ReasonPermissionDeny
		}
	}
}

func isClaudeLoadable(skill Skill) bool {
	return skill.State != StateNotLoaded && skill.Reason != ReasonInvalidFront && skill.Reason != ReasonMissingDesc
}

func claudeDeniedSkills(rules []string) (bool, map[string]bool) {
	denied := make(map[string]bool)
	for _, raw := range rules {
		rule := strings.TrimSpace(raw)
		if rule == "Skill" {
			return true, denied
		}
		if !strings.HasPrefix(rule, "Skill(") || !strings.HasSuffix(rule, ")") {
			continue
		}
		name := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(rule, "Skill("), ")"))
		if name = strings.TrimSuffix(name, " *"); name != "" {
			denied[name] = true
		}
	}
	return false, denied
}

func applyClaudeAdminShadow(skills []Skill) {
	admins := make(map[string]string)
	for _, skill := range skills {
		if skill.Scope == ScopeAdmin && isClaudeLoadable(skill) && skill.State != StateOff {
			admins[skill.InvokeName] = skill.File
		}
	}
	for i := range skills {
		if skills[i].Scope == ScopeUser && skills[i].Plugin == "" && isClaudeLoadable(skills[i]) && skills[i].State == StateOn {
			if winner := admins[skills[i].InvokeName]; winner != "" {
				skills[i].State, skills[i].Reason = StateOverridden, ReasonShadowed
				skills[i].OverriddenBy = winner
			}
		}
	}
}
