package skills

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"
)

func TestGrokDefaultsAndRoots(t *testing.T) {
	env := grokTestEnv(t)
	summary, err := ScanGrok(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Provider != ProviderGrok || !summary.ScannedAt.Equal(env.Now) {
		t.Fatalf("unexpected summary header: %#v", summary)
	}
	want := []string{
		filepath.Join(env.GrokHome, "skills"),
		filepath.Join(env.Home, ".agents", "skills"),
		filepath.Join(env.Home, ".claude", "skills"),
		filepath.Join(env.Home, ".cursor", "skills"),
		filepath.Join(env.GrokHome, "bundled", "skills"),
	}
	for _, path := range want {
		root := grokFindRoot(t, summary, path)
		if root.Exists || root.Count != 0 {
			t.Fatalf("missing root reported as present: %#v", root)
		}
	}
	if _, err := ScanGrok(context.Background(), Env{}); err == nil {
		t.Fatal("missing Home did not fail")
	}
}

func TestGrokRecursivePathsDepthHiddenAndFile(t *testing.T) {
	env := grokTestEnv(t)
	grokWriteSkill(t, filepath.Join(env.GrokHome, "skills", "nested", "deep", "shown"), "shown", "")
	grokWriteSkill(t, filepath.Join(env.GrokHome, "skills", ".hidden", "hidden"), "hidden", "hidden")
	tooDeep := filepath.Join(env.GrokHome, "skills", "one", "two", "three", "four", "five", "six", "seven")
	grokWriteSkill(t, tooDeep, "too-deep", "deep")
	extra := filepath.Join(env.Home, "extra", "single", "SKILL.md")
	grokWriteFile(t, extra, grokSkillText("single", "single file"))
	grokWriteFile(t, filepath.Join(env.GrokHome, "config.toml"), "[skills]\npaths = [\"~/extra/single/SKILL.md\"]\n")

	summary, err := ScanGrok(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	shown := grokFindSkill(t, summary, "shown", "")
	if shown.Description != "First body paragraph used as description." {
		t.Fatalf("body description = %q", shown.Description)
	}
	for _, issue := range summary.Issues {
		if issue.Kind == IssueMissingDesc && issue.Path == shown.File {
			t.Fatalf("body fallback emitted missing-description issue: %#v", issue)
		}
	}
	grokFindSkill(t, summary, "single", "")
	grokAssertNoSkill(t, summary, "hidden")
	grokAssertNoSkill(t, summary, "too-deep")
	if root := grokFindRoot(t, summary, extra); root.Count != 1 {
		t.Fatalf("single-file root count = %d", root.Count)
	}
}

func TestGrokSortsWithinRootByInvokeNameThenFile(t *testing.T) {
	env := grokTestEnv(t)
	grokWriteSkill(t, filepath.Join(env.GrokHome, "skills", "z-directory"), "alpha", "alpha")
	grokWriteSkill(t, filepath.Join(env.GrokHome, "skills", "a-directory"), "zeta", "zeta")
	grokWriteSkill(t, filepath.Join(env.GrokHome, "skills", "b-directory"), "same", "b")
	grokWriteSkill(t, filepath.Join(env.GrokHome, "skills", "c-directory"), "same", "c")
	summary, err := ScanGrok(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	got := summary.Skills[:4]
	if got[0].InvokeName != "alpha" || got[1].File > got[2].File || got[1].InvokeName != "same" || got[2].InvokeName != "same" || got[3].InvokeName != "zeta" {
		t.Fatalf("within-root order = %#v", got)
	}
}

func TestGrokSymlinkLoopAndBrokenLink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation is not generally available to unprivileged Windows tests")
	}
	env := grokTestEnv(t)
	root := filepath.Join(env.GrokHome, "skills")
	grokWriteSkill(t, filepath.Join(root, "real"), "real", "real")
	if err := os.Symlink(root, filepath.Join(root, "real", "loop")); err != nil {
		t.Fatal(err)
	}
	broken := filepath.Join(root, "broken")
	if err := os.Symlink(filepath.Join(root, "missing"), broken); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(env.Home, "linked-target")
	grokWriteSkill(t, target, "linked", "linked")
	link := filepath.Join(root, "linked")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	summary, err := ScanGrok(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	grokFindSkill(t, summary, "real", "")
	linked := grokFindSkill(t, summary, "linked", "")
	if !linked.Deletable || !linked.Removal.IsLink || linked.Removal.Path != link {
		t.Fatalf("linked removal = %#v", linked.Removal)
	}
	for _, issue := range summary.Issues {
		if issue.Kind == IssueBrokenLink && issue.Path == broken && issue.Removable {
			return
		}
	}
	t.Fatalf("removable broken-link issue missing: %#v", summary.Issues)
}

func TestGrokCompatConfigAndEnvironment(t *testing.T) {
	env := grokTestEnv(t)
	grokWriteSkill(t, filepath.Join(env.Home, ".claude", "skills", "claude-one"), "claude-one", "claude")
	grokWriteSkill(t, filepath.Join(env.Home, ".cursor", "skills", "cursor-one"), "cursor-one", "cursor")
	grokWriteFile(t, filepath.Join(env.GrokHome, "config.toml"), "[compat.claude]\nskills = false\n[compat.cursor]\nskills = true\n")
	env.Getenv = func(key string) string {
		if key == "GROK_CURSOR_SKILLS_ENABLED" {
			return "NO"
		}
		return ""
	}
	summary, err := ScanGrok(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"claude-one", "cursor-one"} {
		skill := grokFindSkill(t, summary, name, "")
		if skill.State != StateOff || skill.Reason != ReasonCompatOff {
			t.Fatalf("%s compat state = %s/%s", name, skill.State, skill.Reason)
		}
	}

	env.Getenv = func(key string) string {
		if key == "GROK_CLAUDE_SKILLS_ENABLED" {
			return "1"
		}
		return ""
	}
	summary, err = ScanGrok(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	if skill := grokFindSkill(t, summary, "claude-one", ""); skill.State != StateOn {
		t.Fatalf("environment did not override config: %#v", skill)
	}
}

func TestGrokCompatEnvironmentFalseWords(t *testing.T) {
	falseWords := []string{"false", "FALSE", "0", "No", "OFF"}
	for _, value := range falseWords {
		t.Run(value, func(t *testing.T) {
			if grokCompatEnabled(value, nil) {
				t.Fatalf("%q resolved enabled", value)
			}
		})
	}
	configured := false
	if !grokCompatEnabled("unexpected", &configured) {
		t.Fatal("a present non-false environment value did not override config")
	}
}

func TestGrokDisabledIgnoreAndShadowPriority(t *testing.T) {
	env := grokTestEnv(t)
	grokWriteSkill(t, filepath.Join(env.GrokHome, "skills", "same"), "same", "grok")
	grokWriteSkill(t, filepath.Join(env.Home, ".agents", "skills", "same"), "same", "agents")
	grokWriteSkill(t, filepath.Join(env.Home, ".claude", "skills", "same"), "same", "claude")
	grokWriteSkill(t, filepath.Join(env.GrokHome, "skills", "disabled"), "disabled", "disabled")
	ignoredDir := filepath.Join(env.Home, "ignored")
	grokWriteSkill(t, filepath.Join(ignoredDir, "ignored"), "ignored", "ignored")
	grokWriteFile(t, filepath.Join(env.GrokHome, "config.toml"), "[skills]\npaths = [\"~/ignored\"]\nignore = [\"~/ignored\"]\ndisabled = [\"disabled\"]\n")

	summary, err := ScanGrok(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	same := grokSkillsNamed(summary, "same")
	if len(same) != 3 || same[0].Description != "grok" {
		t.Fatalf("unexpected same-name priority: %#v", same)
	}
	for _, skill := range same[1:] {
		if skill.State != StateOverridden || skill.Reason != ReasonShadowed || skill.OverriddenBy != same[0].File {
			t.Fatalf("shadow state = %#v", skill)
		}
	}
	disabled := grokFindSkill(t, summary, "disabled", "")
	if disabled.State != StateOff || disabled.Reason != ReasonDisabledList {
		t.Fatalf("disabled state = %#v", disabled)
	}
	ignored := grokFindSkill(t, summary, "ignored", "")
	if ignored.State != StateOff || ignored.Reason != ReasonIgnored || ignored.Toggleable || ignored.ToggleBlock != ReasonIgnored {
		t.Fatalf("ignored state = %#v", ignored)
	}
}

func TestGrokInvalidAndOffSourcesDoNotShadow(t *testing.T) {
	env := grokTestEnv(t)
	invalidDir := filepath.Join(env.GrokHome, "skills", "invalid")
	grokWriteFile(t, filepath.Join(invalidDir, "SKILL.md"), "---\nname: invalid\nbroken\n---\nsecret body\n")
	grokWriteSkill(t, filepath.Join(env.Home, ".claude", "skills", "compat"), "compat", "claude")
	grokWriteSkill(t, filepath.Join(env.Home, ".cursor", "skills", "compat"), "compat", "cursor")
	grokWriteFile(t, filepath.Join(env.GrokHome, "config.toml"), "[skills]\nignore = [\"~/grok-home/skills/invalid\"]\ndisabled = [\"invalid\"]\n[compat.claude]\nskills = false\n")
	summary, err := ScanGrok(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	invalid := grokFindSkill(t, summary, "invalid", "")
	if invalid.State != StateNotLoaded || invalid.Reason != ReasonInvalidFront || invalid.ToggleBlock != ReasonIgnored {
		t.Fatalf("invalid configured skill = %#v", invalid)
	}
	foundIssue := false
	for _, issue := range summary.Issues {
		foundIssue = foundIssue || issue.Kind == IssueInvalidFront && issue.Path == invalid.File
	}
	if !foundIssue {
		t.Fatalf("invalid-front issue missing: %#v", summary.Issues)
	}
	compat := grokSkillsNamed(summary, "compat")
	if len(compat) != 2 || compat[0].Reason != ReasonCompatOff || compat[1].State != StateOn {
		t.Fatalf("off compat source shadowed active source: %#v", compat)
	}
}

func TestGrokCollectionRemovalCovers(t *testing.T) {
	env := grokTestEnv(t)
	collection := filepath.Join(env.GrokHome, "skills", "team")
	grokWriteFile(t, filepath.Join(collection, ".git", "HEAD"), "ref: refs/heads/main\n")
	grokWriteSkill(t, filepath.Join(collection, "one"), "one", "one")
	grokWriteSkill(t, filepath.Join(collection, "two"), "two", "two")
	summary, err := ScanGrok(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"one", "two"} {
		skill := grokFindSkill(t, summary, name, "")
		if !skill.Deletable || skill.Removal.Path != collection || skill.Removal.IsLink {
			t.Fatalf("collection removal for %s = %#v", name, skill.Removal)
		}
		if len(skill.Removal.Covers) != 2 || skill.Removal.Covers[0] != "one" || skill.Removal.Covers[1] != "two" {
			t.Fatalf("collection covers for %s = %#v", name, skill.Removal.Covers)
		}
	}
}

func TestGrokBundledManifestAndFallback(t *testing.T) {
	env := grokTestEnv(t)
	root := filepath.Join(env.GrokHome, "bundled", "skills")
	grokWriteSkill(t, filepath.Join(root, "listed"), "listed", "listed")
	grokWriteSkill(t, filepath.Join(root, "stale"), "stale", "stale")
	grokWriteFile(t, filepath.Join(env.GrokHome, "bundled", "manifest.json"), `{"checksums":{"skills/listed/reference.md":"abc","skills/stale":"not-a-prefix-entry"}}`)
	summary, err := ScanGrok(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	listed := grokFindSkill(t, summary, "listed", "")
	if listed.Scope != ScopeBundled || listed.Deletable || !listed.Toggleable {
		t.Fatalf("bundled flags = %#v", listed)
	}
	grokAssertNoSkill(t, summary, "stale")

	if err := os.Remove(filepath.Join(env.GrokHome, "bundled", "manifest.json")); err != nil {
		t.Fatal(err)
	}
	summary, err = ScanGrok(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	grokFindSkill(t, summary, "stale", "")
}

func TestGrokBundledOneLevelLimit(t *testing.T) {
	env := grokTestEnv(t)
	root := filepath.Join(env.GrokHome, "bundled", "skills")
	for index := 0; index < 2001; index++ {
		path := filepath.Join(root, grokLimitName(index))
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	summary, err := ScanGrok(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	for _, issue := range summary.Issues {
		if issue.Kind == IssueTruncated && issue.Path == root {
			return
		}
	}
	t.Fatalf("one-level directory limit did not report truncation: %#v", summary.Issues)
}

func TestGrokPluginRegistryRelativePathAndState(t *testing.T) {
	env := grokTestEnv(t)
	base := filepath.Join(env.GrokHome, "installed-plugins")
	grokWriteSkill(t, filepath.Join(base, "checkout", "tooling", "skills", "release"), "release", "plugin release")
	grokWriteFile(t, filepath.Join(base, "registry.json"), `{
  "version": 1,
  "repos": {"repo-id": {"path": "checkout", "plugins": {"acme": {"subdir": "tooling", "version": "1.2.3"}}}}
}`)
	grokWriteFile(t, filepath.Join(env.GrokHome, "config.toml"), "[plugins]\nenabled = [\"acme\"]\n")
	summary, err := ScanGrok(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	skill := grokFindSkill(t, summary, "release", "acme")
	if skill.InvokeName != "acme:release" || skill.Scope != ScopePlugin || skill.State != StateOn || skill.Toggleable || skill.ToggleBlock != ReasonPluginCollection {
		t.Fatalf("plugin skill = %#v", skill)
	}
	if root := grokFindRoot(t, summary, filepath.Join(base, "checkout", "tooling", "skills")); root.Count != 1 {
		t.Fatalf("plugin root = %#v", root)
	}

	grokWriteFile(t, filepath.Join(env.GrokHome, "config.toml"), "[plugins]\nenabled = [\"acme\"]\ndisabled = [\"acme\"]\n")
	summary, err = ScanGrok(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	skill = grokFindSkill(t, summary, "release", "acme")
	if skill.State != StateOff || skill.Reason != ReasonPluginOff {
		t.Fatalf("disabled plugin = %#v", skill)
	}
}

func TestGrokDisabledAppliesToPluginAndNativeDoesNotShadowIt(t *testing.T) {
	env := grokTestEnv(t)
	grokWriteSkill(t, filepath.Join(env.GrokHome, "skills", "release"), "release", "native")
	base := filepath.Join(env.GrokHome, "installed-plugins")
	grokWriteSkill(t, filepath.Join(base, "repo", "plugin", "skills", "release"), "release", "plugin")
	grokWriteFile(t, filepath.Join(base, "registry.json"), `{"version":1,"repos":{"repo":{"path":"repo","plugins":{"acme":{"subdir":"plugin"}}}}}`)
	grokWriteFile(t, filepath.Join(env.GrokHome, "config.toml"), "[skills]\ndisabled = [\"release\"]\n[plugins]\nenabled = [\"acme\"]\n")
	summary, err := ScanGrok(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	for _, plugin := range []string{"", "acme"} {
		skill := grokFindSkill(t, summary, "release", plugin)
		if skill.State != StateOff || skill.Reason != ReasonDisabledList {
			t.Fatalf("disabled %q release = %#v", plugin, skill)
		}
	}
}

func TestGrokCancellationAndMalformedConfig(t *testing.T) {
	env := grokTestEnv(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ScanGrok(ctx, Env{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled scan error = %v", err)
	}
	grokWriteFile(t, filepath.Join(env.GrokHome, "config.toml"), "[skills\nsecret-marker = [")
	if _, err := ScanGrok(context.Background(), env); !errors.Is(err, errGrokConfig) || err.Error() != errGrokConfig.Error() {
		t.Fatalf("malformed config error = %v", err)
	}
}

func TestGrokMalformedJSONErrorsDoNotExposeContent(t *testing.T) {
	tests := []struct {
		name string
		path func(Env) string
		want error
	}{
		{"manifest", func(env Env) string { return filepath.Join(env.GrokHome, "bundled", "manifest.json") }, errGrokManifest},
		{"registry", func(env Env) string { return filepath.Join(env.GrokHome, "installed-plugins", "registry.json") }, errGrokRegistry},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			env := grokTestEnv(t)
			grokWriteFile(t, test.path(env), `{"secret-marker":`)
			_, err := ScanGrok(context.Background(), env)
			if !errors.Is(err, test.want) || err.Error() != test.want.Error() {
				t.Fatalf("malformed %s error = %v", test.name, err)
			}
		})
	}
}

func grokLimitName(index int) string {
	return "dir-" + strconv.Itoa(index)
}

func grokTestEnv(t *testing.T) Env {
	t.Helper()
	home := t.TempDir()
	return Env{
		Home:      home,
		GrokHome:  filepath.Join(home, "grok-home"),
		ClaudeDir: filepath.Join(home, "claude-home"),
		GOOS:      runtime.GOOS,
		Now:       time.Unix(1_700_000_000, 0),
	}
}

func grokWriteSkill(t *testing.T, dir, name, description string) {
	t.Helper()
	grokWriteFile(t, filepath.Join(dir, "SKILL.md"), grokSkillText(name, description))
}

func grokSkillText(name, description string) string {
	front := "---\nname: " + name + "\n"
	if description != "" {
		front += "description: " + description + "\n"
	}
	return front + "---\n\nFirst body paragraph used as description.\n\nSecond paragraph.\n"
}

func grokWriteFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func grokFindRoot(t *testing.T, summary Summary, path string) RootInfo {
	t.Helper()
	path = filepath.Clean(path)
	for _, root := range summary.Roots {
		if filepath.Clean(root.Path) == path {
			return root
		}
	}
	t.Fatalf("root %q not found in %#v", path, summary.Roots)
	return RootInfo{}
}

func grokFindSkill(t *testing.T, summary Summary, name, plugin string) Skill {
	t.Helper()
	for _, skill := range summary.Skills {
		if skill.Name == name && skill.Plugin == plugin {
			return skill
		}
	}
	t.Fatalf("skill %q plugin %q not found", name, plugin)
	return Skill{}
}

func grokSkillsNamed(summary Summary, name string) []Skill {
	var found []Skill
	for _, skill := range summary.Skills {
		if skill.Name == name && skill.Scope != ScopePlugin {
			found = append(found, skill)
		}
	}
	return found
}

func grokAssertNoSkill(t *testing.T, summary Summary, name string) {
	t.Helper()
	for _, skill := range summary.Skills {
		if skill.Name == name {
			t.Fatalf("unexpected skill %q: %#v", name, skill)
		}
	}
}
