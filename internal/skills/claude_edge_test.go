package skills

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestClaudeManifestLimitAndAdminShadowState(t *testing.T) {
	t.Run("manifest entries", func(t *testing.T) {
		env := claudeTestEnv(t)
		collection := filepath.Join(env.ClaudeDir, "skills", "large")
		paths := make([]string, 20001)
		for i := range paths {
			paths[i] = "missing"
		}
		manifest, err := json.Marshal(map[string]any{"name": "large", "skills": paths})
		if err != nil {
			t.Fatal(err)
		}
		claudeWriteFile(t, filepath.Join(collection, ".claude-plugin", "plugin.json"), string(manifest))
		summary, err := ScanClaude(context.Background(), env)
		if err != nil {
			t.Fatal(err)
		}
		if !claudeHasIssue(summary, IssueTruncated, collection) {
			t.Fatalf("manifest limit issue missing: %#v", summary.Issues)
		}
	})

	t.Run("off user remains off", func(t *testing.T) {
		env := claudeTestEnv(t)
		claudeWriteSkill(t, filepath.Join(env.ClaudeDir, "skills", "same"), "same", "user")
		claudeWriteSkill(t, filepath.Join(claudeManagedRootFn(env.GOOS), "same"), "same", "admin")
		claudeWriteFile(t, filepath.Join(env.ClaudeDir, "settings.json"), `{"skillOverrides":{"same":"off"}}`)
		summary, err := ScanClaude(context.Background(), env)
		if err != nil {
			t.Fatal(err)
		}
		for _, skill := range claudeSkillsNamed(summary, "same") {
			if skill.State != StateOff || skill.Reason != ReasonOverride {
				t.Fatalf("shadow replaced off state: %#v", skill)
			}
		}
	})
}

func TestClaudeWindowsManagedRootInfo(t *testing.T) {
	if got := claudeManagedRoot("windows"); got != `C:\Program Files\ClaudeCode\.claude\skills` {
		t.Fatalf("Windows managed path = %q", got)
	}
	fakeManaged := filepath.Join(t.TempDir(), "managed", "skills")
	previous := claudeManagedRootFn
	claudeManagedRootFn = func(string) string { return fakeManaged }
	t.Cleanup(func() { claudeManagedRootFn = previous })
	root := claudeManagedRootInfo(Env{GOOS: "windows"})
	if root.Path != fakeManaged || root.Exists || root.Scope != ScopeAdmin {
		t.Fatalf("missing Windows managed root = %#v", root)
	}
	if err := os.MkdirAll(fakeManaged, 0o755); err != nil {
		t.Fatal(err)
	}
	root = claudeManagedRootInfo(Env{GOOS: "windows"})
	if root.Path != fakeManaged || !root.Exists || root.Scope != ScopeAdmin {
		t.Fatalf("existing Windows managed root = %#v", root)
	}
}
