package claude

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode"

	"github.com/licoy/quosparr/internal/platform"
)

// ErrSkillNameInvalid means a skill name is empty or contains a control rune.
var ErrSkillNameInvalid = errors.New("claude: skill name is empty or contains control characters")

// SetSkillOverride disables a skill with "off". Enabling removes any explicit
// state because absent means the default "on", including reduced visibility states.
func SetSkillOverride(data []byte, name string, enabled bool) ([]byte, error) {
	if err := validateSkillName(name); err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	root, err := rawSettingsObject(data)
	if err != nil {
		return nil, err
	}
	overrides := map[string]json.RawMessage{}
	if raw, ok := root["skillOverrides"]; ok {
		if !isJSONObject(raw) || json.Unmarshal(raw, &overrides) != nil || overrides == nil {
			return nil, ErrSettingsInvalid
		}
	}
	if enabled {
		if _, ok := overrides[name]; !ok {
			return data, nil
		}
		delete(overrides, name)
		if len(overrides) == 0 {
			delete(root, "skillOverrides")
		} else {
			root["skillOverrides"] = marshalCompact(overrides)
		}
	} else {
		if stringField(overrides[name]) == "off" {
			return data, nil
		}
		overrides[name] = rawJSONString("off")
		root["skillOverrides"] = marshalCompact(overrides)
	}
	out, err := encodeSettings(root)
	if err != nil {
		return nil, err
	}
	if _, err := rawSettingsObject(out); err != nil {
		return nil, err
	}
	return out, nil
}

func validateSkillName(name string) error {
	if strings.TrimSpace(name) == "" {
		return ErrSkillNameInvalid
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return ErrSkillNameInvalid
		}
	}
	return nil
}

// SetSkillEnabledFile updates skillOverrides in settings.json with backup.
func (s *Store) SetSkillEnabledFile(name string, enabled bool, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return platform.EditFileWithBackup(s.SettingsPath(), s.settingsBackup(), now, func(old []byte) ([]byte, error) {
		return SetSkillOverride(old, name, enabled)
	})
}
