package skills

import (
	"context"
	"path/filepath"
	"testing"
)

func TestClaudeOverrideAndDenyTable(t *testing.T) {
	tests := []struct {
		name       string
		settings   string
		wantState  State
		wantReason Reason
		wantField  bool
	}{
		{"override off", `{"skillOverrides":{"target":"off"}}`, StateOff, ReasonOverride, false},
		{"name only", `{"skillOverrides":{"target":"name-only"}}`, StateOn, ReasonNone, true},
		{"user only", `{"skillOverrides":{"target":"user-invocable-only"}}`, StateOn, ReasonNone, true},
		{"deny exact", `{"permissions":{"deny":["Skill(target)"]}}`, StateOff, ReasonPermissionDeny, false},
		{"deny arguments", `{"permissions":{"deny":["Skill(target *)"]}}`, StateOff, ReasonPermissionDeny, false},
		{"unrelated deny", `{"permissions":{"deny":["Skill(other)"]}}`, StateOn, ReasonNone, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			env := claudeTestEnv(t)
			claudeWriteSkill(t, filepath.Join(env.ClaudeDir, "skills", "target"), "target", "target")
			claudeWriteFile(t, filepath.Join(env.ClaudeDir, "settings.json"), test.settings)
			summary, err := ScanClaude(context.Background(), env)
			if err != nil {
				t.Fatal(err)
			}
			skill := claudeFindSkill(t, summary, "target", "")
			if skill.State != test.wantState || skill.Reason != test.wantReason || claudeHasField(skill, "skillOverrides") != test.wantField {
				t.Fatalf("skill = %#v", skill)
			}
		})
	}
}
