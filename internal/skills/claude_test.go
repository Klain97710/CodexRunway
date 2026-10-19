package skills

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestClaudeRootsNestedSyncedAndManagedOverride(t *testing.T) {
	env := claudeTestEnv(t)
	managed := claudeManagedRootFn(env.GOOS)
	claudeWriteSkill(t, filepath.Join(env.ClaudeDir, "skills", "same"), "same", "user")
	claudeWriteSkill(t, filepath.Join(env.ClaudeDir, "skills", "outer", "nested"), "nested", "nested")
	claudeWriteSkill(t, filepath.Join(env.ClaudeDir, "skills", "synced", "cloud"), "cloud", "synced")
	claudeWriteSkill(t, filepath.Join(managed, "same"), "same", "managed")

	summary, err := ScanClaude(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	same := claudeSkillsNamed(summary, "same")
	if len(same) != 2 || same[0].Scope != ScopeAdmin || same[1].Scope != ScopeUser {
		t.Fatalf("managed priority = %#v", same)
	}
	if same[1].State != StateOverridden || same[1].Reason != ReasonShadowed || same[1].OverriddenBy != same[0].File {
		t.Fatalf("user shadow = %#v", same[1])
	}
	if same[0].Toggleable || same[0].ToggleBlock != ReasonReadOnlyScope {
		t.Fatalf("managed toggle = %#v", same[0])
	}
	nested := claudeFindSkill(t, summary, "nested", "")
	if nested.State != StateNotLoaded || nested.Reason != ReasonNested || !claudeHasIssue(summary, IssueNested, nested.File) {
		t.Fatalf("nested skill = %#v issues=%#v", nested, summary.Issues)
	}
	cloud := claudeFindSkill(t, summary, "cloud", "")
	if cloud.Scope != ScopeSynced || cloud.Deletable || !cloud.Toggleable {
		t.Fatalf("synced skill = %#v", cloud)
	}
	if root := claudeFindRoot(t, summary, filepath.Join(env.ClaudeDir, "skills")); root.Count != 2 {
		t.Fatalf("user root count = %#v", root)
	}
	if root := claudeFindRoot(t, summary, filepath.Join(env.ClaudeDir, "skills", "synced")); root.Count != 1 {
		t.Fatalf("synced root count = %#v", root)
	}
}

func TestClaudeOverridesAndPermissionDeny(t *testing.T) {
	env := claudeTestEnv(t)
	for _, name := range []string{"hidden", "named", "manual", "allowed"} {
		claudeWriteSkill(t, filepath.Join(env.ClaudeDir, "skills", name), name, name)
	}
	claudeWriteFile(t, filepath.Join(env.ClaudeDir, "settings.json"), `{
  "skillOverrides": {
    "hidden": "off",
    "named": "name-only",
    "manual": "user-invocable-only"
  },
  "permissions": {"deny": ["Skill(named *)", "Skill(manual)"]}
}`)
	summary, err := ScanClaude(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	if skill := claudeFindSkill(t, summary, "hidden", ""); skill.State != StateOff || skill.Reason != ReasonOverride {
		t.Fatalf("off override = %#v", skill)
	}
	for _, name := range []string{"named", "manual"} {
		skill := claudeFindSkill(t, summary, name, "")
		if skill.State != StateOff || skill.Reason != ReasonPermissionDeny || skill.Toggleable || skill.ToggleBlock != ReasonPermissionDeny {
			t.Fatalf("deny %s = %#v", name, skill)
		}
		if !claudeHasField(skill, "skillOverrides") {
			t.Fatalf("override field missing for %s: %#v", name, skill.Fields)
		}
	}
	if skill := claudeFindSkill(t, summary, "allowed", ""); skill.State != StateOn {
		t.Fatalf("unmatched deny changed skill: %#v", skill)
	}

	claudeWriteFile(t, filepath.Join(env.ClaudeDir, "settings.json"), `{"permissions":{"deny":["Skill"]}}`)
	summary, err = ScanClaude(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	for _, skill := range summary.Skills {
		if isClaudeLoadable(skill) && (skill.State != StateOff || skill.Reason != ReasonPermissionDeny) {
			t.Fatalf("global Skill deny missed %#v", skill)
		}
	}
}

func TestClaudeSkillsDirPlugins(t *testing.T) {
	env := claudeTestEnv(t)
	root := filepath.Join(env.ClaudeDir, "skills")
	explicit := filepath.Join(root, "explicit-plugin")
	claudeWriteFile(t, filepath.Join(explicit, ".claude-plugin", "plugin.json"), `{"name":"suite","skills":["workflows","single/SKILL.md","broken/SKILL.md","auth.json"]}`)
	claudeWriteSkill(t, filepath.Join(explicit, "workflows", "first"), "first", "first")
	claudeWriteSkill(t, filepath.Join(explicit, "single"), "single", "single")
	claudeWriteFile(t, filepath.Join(explicit, "auth.json"), `{"token":"must-not-be-parsed-as-skill"}`)
	claudeWriteFile(t, filepath.Join(explicit, "broken", "SKILL.md"), "not frontmatter")
	unlisted := filepath.Join(explicit, "unlisted", "skip", "SKILL.md")
	claudeWriteFile(t, unlisted, "not frontmatter")

	defaults := filepath.Join(root, "default-plugin")
	claudeWriteFile(t, filepath.Join(defaults, ".claude-plugin", "plugin.json"), `{"name":"defaults"}`)
	claudeWriteSkill(t, filepath.Join(defaults, "skills", "default-skill"), "default-skill", "default")
	claudeWriteSkill(t, filepath.Join(defaults, "skills", "nested", "skip-default"), "skip-default", "skip")
	hidden := filepath.Join(root, ".hidden-plugin")
	claudeWriteFile(t, filepath.Join(hidden, ".claude-plugin", "plugin.json"), `{"name":"hidden"}`)
	claudeWriteSkill(t, filepath.Join(hidden, "skills", "hidden-skill"), "hidden-skill", "hidden")
	claudeWriteFile(t, filepath.Join(env.ClaudeDir, "settings.json"), `{
  "enabledPlugins":{"suite@skills-dir":false},
  "skillOverrides":{"first":"off","default-skill":"off"}
}`)

	summary, err := ScanClaude(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"first", "single"} {
		skill := claudeFindSkill(t, summary, name, "suite")
		if skill.InvokeName != "suite:"+name || skill.State != StateOff || skill.Reason != ReasonPluginOff || skill.ToggleBlock != ReasonPluginCollection {
			t.Fatalf("explicit plugin %s = %#v", name, skill)
		}
		if skill.Removal.Path != explicit || len(skill.Removal.Covers) != 3 {
			t.Fatalf("plugin removal %s = %#v", name, skill.Removal)
		}
	}
	defaultSkill := claudeFindSkill(t, summary, "default-skill", "defaults")
	if defaultSkill.State != StateOn || defaultSkill.Root != root {
		t.Fatalf("default plugin = %#v", defaultSkill)
	}
	broken := claudeFindSkill(t, summary, "broken", "suite")
	if broken.State != StateNotLoaded || broken.Reason != ReasonInvalidFront {
		t.Fatalf("plugin state hid invalid frontmatter: %#v", broken)
	}
	claudeAssertNoSkill(t, summary, "skip")
	claudeAssertNoSkill(t, summary, "skip-default")
	claudeAssertNoSkill(t, summary, "hidden-skill")
	if claudeHasIssue(summary, IssueInvalidFront, unlisted) {
		t.Fatalf("unlisted plugin skill was read: %#v", summary.Issues)
	}
	if userRoot := claudeFindRoot(t, summary, root); userRoot.Count != 4 {
		t.Fatalf("plugin root count = %#v", userRoot)
	}
}

func TestClaudeInstalledPluginsOnlyUserAndDefaultEnabled(t *testing.T) {
	env := claudeTestEnv(t)
	userInstall := filepath.Join(env.Home, "plugins", "user")
	projectInstall := filepath.Join(env.Home, "plugins", "project")
	claudeWriteSkill(t, filepath.Join(userInstall, "skills", "deploy"), "deploy", "user plugin")
	claudeWriteFile(t, filepath.Join(userInstall, "skills", "deploy", "references", "SKILL.md"), "not nested")
	claudeWriteFile(t, filepath.Join(userInstall, "skills", "broken", "SKILL.md"), "not frontmatter")
	claudeWriteSkill(t, filepath.Join(projectInstall, "skills", "ignore"), "ignore", "project plugin")
	claudeWriteFile(t, filepath.Join(env.ClaudeDir, "plugins", "installed_plugins.json"), `{
  "version":2,
  "plugins":{"pack@market":[
    {"scope":"project","installPath":"`+filepath.ToSlash(projectInstall)+`","version":"1"},
    {"scope":"user","installPath":"`+filepath.ToSlash(userInstall)+`","version":"2"}
  ]}
}`)

	summary, err := ScanClaude(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	plugin := claudeFindSkill(t, summary, "deploy", "pack")
	if plugin.InvokeName != "pack:deploy" || plugin.State != StateOn || plugin.Scope != ScopePlugin || plugin.Deletable {
		t.Fatalf("installed plugin = %#v", plugin)
	}
	broken := claudeFindSkill(t, summary, "broken", "pack")
	if broken.State != StateNotLoaded || broken.Reason != ReasonInvalidFront {
		t.Fatalf("installed invalid = %#v", broken)
	}
	claudeAssertNoSkill(t, summary, "ignore")
	if root := claudeFindRoot(t, summary, filepath.Join(userInstall, "skills")); root.Count != 2 {
		t.Fatalf("installed root = %#v", root)
	}

	claudeWriteFile(t, filepath.Join(env.ClaudeDir, "settings.json"), `{"enabledPlugins":{"pack@market":false}}`)
	summary, err = ScanClaude(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	if skill := claudeFindSkill(t, summary, "deploy", "pack"); skill.State != StateOff || skill.Reason != ReasonPluginOff {
		t.Fatalf("disabled installed plugin = %#v", skill)
	}
	broken = claudeFindSkill(t, summary, "broken", "pack")
	if broken.State != StateNotLoaded || broken.Reason != ReasonInvalidFront {
		t.Fatalf("plugin off hid invalid frontmatter: %#v", broken)
	}
}

func TestClaudeDefaultsSymlinkDedupAndCancellation(t *testing.T) {
	if got := claudeManagedRoot("darwin"); got != "/Library/Application Support/ClaudeCode/.claude/skills" {
		t.Fatalf("darwin managed root = %q", got)
	}
	if got := claudeManagedRoot("linux"); got != "/etc/claude-code/.claude/skills" {
		t.Fatalf("linux managed root = %q", got)
	}
	if got := claudeManagedRoot("windows"); got != `C:\Program Files\ClaudeCode\.claude\skills` {
		t.Fatalf("windows managed root = %q", got)
	}
	env := claudeTestEnv(t)
	if runtime.GOOS != "windows" {
		real := claudeWriteSkill(t, filepath.Join(env.ClaudeDir, "skills", "real"), "real", "real")
		if err := os.Symlink(filepath.Dir(real), filepath.Join(env.ClaudeDir, "skills", "alias")); err != nil {
			t.Fatal(err)
		}
		managed := claudeManagedRootFn(env.GOOS)
		if err := os.MkdirAll(managed, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Dir(real), filepath.Join(managed, "real")); err != nil {
			t.Fatal(err)
		}
		pluginSource := filepath.Join(env.Home, "linked-plugin-source")
		claudeWriteFile(t, filepath.Join(pluginSource, ".claude-plugin", "plugin.json"), `{"name":"linked"}`)
		claudeWriteSkill(t, filepath.Join(pluginSource, "skills", "linked-skill"), "linked-skill", "linked")
		pluginLink := filepath.Join(env.ClaudeDir, "skills", "linked-plugin")
		if err := os.Symlink(pluginSource, pluginLink); err != nil {
			t.Fatal(err)
		}
		summary, err := ScanClaude(context.Background(), env)
		if err != nil {
			t.Fatal(err)
		}
		realSkills := claudeSkillsNamed(summary, "real")
		if len(realSkills) != 1 || realSkills[0].Scope != ScopeAdmin || realSkills[0].Deletable {
			t.Fatalf("symlink duplicate was not removed: %#v", claudeSkillsNamed(summary, "real"))
		}
		linked := claudeFindSkill(t, summary, "linked-skill", "linked")
		if linked.LinkPath != pluginLink || !linked.Removal.IsLink || linked.Removal.Path != pluginLink {
			t.Fatalf("linked plugin removal = %#v", linked)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ScanClaude(ctx, env); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled scan error = %v", err)
	}
	if _, err := ScanClaude(ctx, Env{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled empty-env scan error = %v", err)
	}
}
