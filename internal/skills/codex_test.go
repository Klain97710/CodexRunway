package skills

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCodexRootsPriorityAndRecursiveScan(t *testing.T) {
	env := codexTestEnv(t)
	env.GOOS = "linux"
	codexWriteSkill(t, filepath.Join(env.CodexHome, "skills", "z", "deep"), "deep", "codex")
	codexWriteSkill(t, filepath.Join(env.Home, ".agents", "skills", "agent"), "agent", "agents")
	codexWriteSkill(t, filepath.Join(env.CodexHome, "skills", ".hidden", "skip"), "skip", "hidden")
	codexWriteSkill(t, filepath.Join(env.CodexHome, "skills", "one", "two", "three", "four", "five", "six", "seven"), "too-deep", "deep")
	codexWriteSkill(t, filepath.Join(env.CodexHome, "skills", ".system", "builtin"), "builtin", "system")
	codexWriteSkill(t, filepath.Join(codexAdminRoot, "managed"), "managed", "admin")

	summary, err := ScanCodex(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"deep", "agent", "builtin", "managed"} {
		codexFindSkill(t, summary, name, "")
	}
	codexAssertNoSkill(t, summary, "skip")
	codexAssertNoSkill(t, summary, "too-deep")
	if !codexHasIssueKind(summary, IssueTruncated) {
		t.Fatalf("depth limit issue missing: %#v", summary.Issues)
	}
	if got := []Scope{summary.Skills[0].Scope, summary.Skills[1].Scope, summary.Skills[2].Scope, summary.Skills[3].Scope}; fmt.Sprint(got) != "[user user system admin]" {
		t.Fatalf("scope priority = %v", got)
	}
	if root := codexFindRoot(t, summary, filepath.Join(env.CodexHome, "skills")); root.Count != 1 {
		t.Fatalf("codex root count = %#v", root)
	}
	if root := codexFindRoot(t, summary, filepath.Join(env.CodexHome, "skills", ".system")); root.Count != 1 {
		t.Fatalf("system root count = %#v", root)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ScanCodex(ctx, env); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled scan error = %v", err)
	}
	if _, err := ScanCodex(ctx, Env{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled empty-env scan error = %v", err)
	}
}

func TestCodexConfigOrderBundledAndCanonicalPath(t *testing.T) {
	env := codexTestEnv(t)
	userFile := codexWriteSkill(t, filepath.Join(env.CodexHome, "skills", "alpha"), "alpha", "user")
	codexWriteSkill(t, filepath.Join(env.CodexHome, "skills", ".system", "builtin"), "builtin", "system")
	config := fmt.Sprintf(`
[[skills.config]]
name = "alpha"
enabled = false
[[skills.config]]
path = %q
enabled = true
[[skills.config]]
name = "alpha"
enabled = false
[skills.bundled]
enabled = false
`, filepath.Join(filepath.Dir(userFile), ".", "SKILL.md"))
	codexWriteFile(t, filepath.Join(env.CodexHome, "config.toml"), config)

	summary, err := ScanCodex(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	alpha := codexFindSkill(t, summary, "alpha", "")
	if alpha.State != StateOff || alpha.Reason != ReasonConfigName {
		t.Fatalf("ordered config state = %#v", alpha)
	}
	builtin := codexFindSkill(t, summary, "builtin", "")
	if builtin.State != StateOff || builtin.Reason != ReasonBundledOff {
		t.Fatalf("bundled state = %#v", builtin)
	}

	codexWriteFile(t, filepath.Join(env.CodexHome, "config.toml"), "[[skills.config]\nname =")
	summary, err = ScanCodex(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	if skill := codexFindSkill(t, summary, "alpha", ""); skill.State != StateOn {
		t.Fatalf("malformed config was not ignored: %#v", skill)
	}
}

func TestCodexPluginCollectionsCacheAndDedup(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation is not generally available to unprivileged Windows tests")
	}
	env := codexTestEnv(t)
	collection := filepath.Join(env.CodexHome, "skills", "collection")
	codexWriteFile(t, filepath.Join(collection, ".codex-plugin", "plugin.json"), `{"name":"suite"}`)
	codexWriteSkill(t, filepath.Join(collection, "skills", "local"), "local", "local")
	shared := codexWriteSkill(t, filepath.Join(env.CodexHome, "skills", "shared"), "shared", "shared")
	if err := os.MkdirAll(filepath.Join(env.Home, ".agents", "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Dir(shared), filepath.Join(env.Home, ".agents", "skills", "shared-link")); err != nil {
		t.Fatal(err)
	}

	cache := filepath.Join(env.CodexHome, "plugins", "cache", "market", "tool")
	oldRoot := filepath.Join(cache, "1")
	newRoot := filepath.Join(cache, "2")
	codexWriteSkill(t, filepath.Join(oldRoot, "skills", "old"), "old", "old")
	codexWriteFile(t, filepath.Join(newRoot, ".codex-plugin", "plugin.json"), `{"name":"manifest-name"}`)
	codexWriteSkill(t, filepath.Join(newRoot, "skills", "new"), "new", "new")
	codexWriteFile(t, filepath.Join(newRoot, "skills", "new", "references", "SKILL.md"), "not a nested skill")
	codexWriteFile(t, filepath.Join(newRoot, "skills", "broken", "SKILL.md"), "not frontmatter")
	oldTime := time.Unix(100, 0)
	newTime := time.Unix(200, 0)
	if err := os.Chtimes(oldRoot, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(newRoot, newTime, newTime); err != nil {
		t.Fatal(err)
	}
	codexWriteFile(t, filepath.Join(env.CodexHome, "config.toml"), `[plugins."tool@market"]
enabled = false
[plugins."missing@market"]
enabled = true
`)

	summary, err := ScanCodex(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	local := codexFindSkill(t, summary, "local", "suite")
	if local.InvokeName != "suite:local" || !local.Toggleable || local.ToggleBlock != ReasonNone || local.Removal.Path != collection {
		t.Fatalf("collection skill = %#v", local)
	}
	plugin := codexFindSkill(t, summary, "new", "tool")
	if plugin.InvokeName != "tool:new" || plugin.State != StateOff || plugin.Reason != ReasonPluginOff || plugin.Scope != ScopePlugin {
		t.Fatalf("cached plugin = %#v", plugin)
	}
	broken := codexFindSkill(t, summary, "broken", "tool")
	if broken.State != StateNotLoaded || broken.Reason != ReasonInvalidFront {
		t.Fatalf("plugin state hid invalid frontmatter: %#v", broken)
	}
	for _, issue := range summary.Issues {
		if strings.Contains(issue.Path, filepath.Join("new", "references")) || issue.Kind == IssueTruncated && strings.Contains(issue.Path, filepath.Join("skills", "new")) {
			t.Fatalf("direct plugin scan reported nested assets: %#v", issue)
		}
	}
	codexAssertNoSkill(t, summary, "old")
	if len(codexSkillsNamed(summary, "shared")) != 1 {
		t.Fatalf("real-file duplicate was not removed: %#v", codexSkillsNamed(summary, "shared"))
	}
	codexFindRoot(t, summary, filepath.Join(cache, "2", "skills"))
	codexFindRoot(t, summary, filepath.Join(env.CodexHome, "plugins", "cache", "market", "missing"))
}

func TestCodexNameLimitsAndWindowsRoots(t *testing.T) {
	env := codexTestEnv(t)
	env.GOOS = "windows"
	longName := strings.Repeat("界", 65)
	codexWriteSkill(t, filepath.Join(env.CodexHome, "skills", "long"), longName, "long")
	summary, err := ScanCodex(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	for _, root := range summary.Roots {
		if filepath.Clean(root.Path) == filepath.Clean(codexAdminRoot) {
			t.Fatal("Windows scan included admin root")
		}
	}
	for _, issue := range summary.Issues {
		if issue.Kind == IssueNameTooLong && issue.Name == longName {
			return
		}
	}
	t.Fatalf("name-too-long issue missing: %#v", summary.Issues)
}

func codexTestEnv(t *testing.T) Env {
	t.Helper()
	home := t.TempDir()
	previous := codexAdminRoot
	codexAdminRoot = filepath.Join(home, "etc", "codex", "skills")
	t.Cleanup(func() { codexAdminRoot = previous })
	return Env{Home: home, CodexHome: filepath.Join(home, "codex-home"), ClaudeDir: filepath.Join(home, "claude-home"), GOOS: runtime.GOOS, Now: time.Unix(1_700_000_000, 0)}
}

func codexWriteSkill(t *testing.T, dir, name, description string) string {
	t.Helper()
	file := filepath.Join(dir, "SKILL.md")
	codexWriteFile(t, file, "---\nname: "+name+"\ndescription: "+description+"\n---\n\nBody.\n")
	return file
}

func codexWriteFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func codexFindSkill(t *testing.T, summary Summary, name, plugin string) Skill {
	t.Helper()
	for _, skill := range summary.Skills {
		if skill.Name == name && skill.Plugin == plugin {
			return skill
		}
	}
	t.Fatalf("skill %q plugin %q not found in %#v", name, plugin, summary.Skills)
	return Skill{}
}

func codexSkillsNamed(summary Summary, name string) []Skill {
	var found []Skill
	for _, skill := range summary.Skills {
		if skill.Name == name {
			found = append(found, skill)
		}
	}
	return found
}

func codexFindRoot(t *testing.T, summary Summary, path string) RootInfo {
	t.Helper()
	for _, root := range summary.Roots {
		if filepath.Clean(root.Path) == filepath.Clean(path) {
			return root
		}
	}
	t.Fatalf("root %q not found in %#v", path, summary.Roots)
	return RootInfo{}
}

func codexAssertNoSkill(t *testing.T, summary Summary, name string) {
	t.Helper()
	if skills := codexSkillsNamed(summary, name); len(skills) != 0 {
		t.Fatalf("unexpected skill %q: %#v", name, skills)
	}
}

func codexHasIssueKind(summary Summary, kind IssueKind) bool {
	for _, issue := range summary.Issues {
		if issue.Kind == kind {
			return true
		}
	}
	return false
}
