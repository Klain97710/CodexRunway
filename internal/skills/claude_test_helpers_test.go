package skills

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func claudeTestEnv(t *testing.T) Env {
	t.Helper()
	home := t.TempDir()
	previous := claudeManagedRootFn
	managed := filepath.Join(home, "managed", "skills")
	claudeManagedRootFn = func(string) string { return managed }
	t.Cleanup(func() { claudeManagedRootFn = previous })
	return Env{Home: home, ClaudeDir: filepath.Join(home, "claude-home"), CodexHome: filepath.Join(home, "codex-home"), GOOS: runtime.GOOS, Now: time.Unix(1_700_000_000, 0)}
}

func claudeWriteSkill(t *testing.T, dir, name, description string) string {
	t.Helper()
	file := filepath.Join(dir, "SKILL.md")
	claudeWriteFile(t, file, "---\nname: "+name+"\ndescription: "+description+"\n---\n\nBody.\n")
	return file
}

func claudeWriteFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func claudeFindSkill(t *testing.T, summary Summary, name, plugin string) Skill {
	t.Helper()
	for _, skill := range summary.Skills {
		if skill.Name == name && skill.Plugin == plugin {
			return skill
		}
	}
	t.Fatalf("skill %q plugin %q not found in %#v", name, plugin, summary.Skills)
	return Skill{}
}

func claudeSkillsNamed(summary Summary, name string) []Skill {
	var found []Skill
	for _, skill := range summary.Skills {
		if skill.Name == name {
			found = append(found, skill)
		}
	}
	return found
}

func claudeFindRoot(t *testing.T, summary Summary, path string) RootInfo {
	t.Helper()
	for _, root := range summary.Roots {
		if filepath.Clean(root.Path) == filepath.Clean(path) {
			return root
		}
	}
	t.Fatalf("root %q not found in %#v", path, summary.Roots)
	return RootInfo{}
}

func claudeHasIssue(summary Summary, kind IssueKind, path string) bool {
	for _, issue := range summary.Issues {
		if issue.Kind == kind && filepath.Clean(issue.Path) == filepath.Clean(path) {
			return true
		}
	}
	return false
}

func claudeHasField(skill Skill, key string) bool {
	for _, field := range skill.Fields {
		if field.Key == key {
			return true
		}
	}
	return false
}

func claudeAssertNoSkill(t *testing.T, summary Summary, name string) {
	t.Helper()
	if skills := claudeSkillsNamed(summary, name); len(skills) != 0 {
		t.Fatalf("unexpected skill %q: %#v", name, skills)
	}
}
