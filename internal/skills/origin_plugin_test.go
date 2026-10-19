package skills

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestPluginOriginManifestRepositoryForms(t *testing.T) {
	tests := []struct {
		name, repository, wantRepo, wantRef, wantPath string
	}{
		{"string", `"https://github.com/acme/tools.git"`, "https://github.com/acme/tools", "", ""},
		{"object", `{"type":"git","url":"git+https://gitlab.com/acme/tools.git?token=x#frag"}`, "https://gitlab.com/acme/tools", "", ""},
		{"tree", `"https://github.com/acme/tools/tree/main"`, "https://github.com/acme/tools", "main", "skills/release"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			env, skill, pluginRoot := pluginOriginFixture(t, ProviderClaude)
			manifestPath := filepath.Join(pluginRoot, ".claude-plugin", "plugin.json")
			pluginWriteOriginTestFile(t, manifestPath, `{"name":"pack","repository":`+test.repository+`,"homepage":"https://docs.example.com/?key=x#part","version":"1.2.3"}`)
			cache := newOriginPluginCache(env)
			if _, ok := cache.pluginManifest(manifestPath); !ok {
				t.Fatalf("manifest not read: %s", manifestPath)
			}
			if root, _, ok := cache.pluginRoot(skill); !ok {
				t.Fatalf("manifest root not found: root=%s real=%s scan=%s", root, skill.RealFile, skill.Root)
			}
			candidate := cache.candidate(skill)
			if candidate.repo != test.wantRepo || candidate.ref != test.wantRef || candidate.skillPath != test.wantPath || candidate.homepage != "https://docs.example.com" || candidate.version != "1.2.3" {
				t.Fatalf("candidate = %#v", candidate)
			}
		})
	}
}

func TestPluginOriginManifestRejectsLinkOversizeAndCaches(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation is not generally available to unprivileged Windows tests")
	}
	env, skill, pluginRoot := pluginOriginFixture(t, ProviderCodex)
	manifest := filepath.Join(pluginRoot, ".codex-plugin", "plugin.json")
	target := filepath.Join(env.Home, "manifest.json")
	pluginWriteOriginTestFile(t, target, `{"repository":"https://github.com/acme/link"}`)
	if err := os.MkdirAll(filepath.Dir(manifest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, manifest); err != nil {
		t.Fatal(err)
	}
	if got := newOriginPluginCache(env).candidate(skill).repo; got != "" {
		t.Fatalf("symlink repository = %q", got)
	}
	if err := os.Remove(manifest); err != nil {
		t.Fatal(err)
	}
	pluginWriteOriginTestFile(t, manifest, `{"repository":"https://github.com/acme/first"}`)
	cache := newOriginPluginCache(env)
	if got := cache.candidate(skill).repo; got != "https://github.com/acme/first" {
		t.Fatalf("first repository = %q", got)
	}
	pluginWriteOriginTestFile(t, manifest, `{"repository":"https://github.com/acme/second"}`)
	if got := cache.candidate(skill).repo; got != "https://github.com/acme/first" {
		t.Fatalf("cached repository = %q", got)
	}
	pluginWriteOriginTestFile(t, manifest, strings.Repeat("x", (64<<10)+1))
	if got := newOriginPluginCache(env).candidate(skill).repo; got != "" {
		t.Fatalf("oversize repository = %q", got)
	}
}

func TestClaudePluginOriginMarketplaceFallback(t *testing.T) {
	env, skill, pluginRoot := pluginOriginFixture(t, ProviderClaude)
	skill.Plugin = "openai-codex"
	pluginWriteOriginTestFile(t, filepath.Join(env.ClaudeDir, "plugins", "installed_plugins.json"), `{"version":2,"plugins":{"openai-codex@official":[{"scope":"user","installPath":"`+filepath.ToSlash(pluginRoot)+`","version":"1.0.6","gitCommitSha":"abcdef1","installedAt":"2026-10-01T00:00:00Z","lastUpdated":"2026-10-16T12:34:56.123Z"}]}}`)
	marketRoot := filepath.Join(env.Home, "official")
	pluginWriteOriginTestFile(t, filepath.Join(env.ClaudeDir, "plugins", "known_marketplaces.json"), `{"official":{"source":{"source":"github","repo":"openai/codex-plugin-cc"},"installLocation":"`+filepath.ToSlash(marketRoot)+`","lastUpdated":"2026-10-16T12:34:56.123Z"}}`)
	pluginWriteOriginTestFile(t, filepath.Join(marketRoot, ".claude-plugin", "marketplace.json"), `{"plugins":[{"name":"openai-codex","source":"plugins/codex","homepage":"https://developers.openai.com/codex","version":"old"}]}`)
	candidate := newOriginPluginCache(env).candidate(skill)
	if candidate.repo != "https://github.com/openai/codex-plugin-cc" || candidate.skillPath != "plugins/codex/skills/release" || candidate.installer != "official" || candidate.version != "1.0.6" || candidate.ref != "abcdef1" {
		t.Fatalf("candidate = %#v", candidate)
	}
	if candidate.updatedAt.Format(time.RFC3339Nano) != "2026-10-16T12:34:56.123Z" {
		t.Fatalf("updatedAt = %v", candidate.updatedAt)
	}
}

func TestClaudePluginOriginKeepsKnownRepoWithoutMarketplaceManifest(t *testing.T) {
	env, skill, pluginRoot := pluginOriginFixture(t, ProviderClaude)
	skill.Plugin = "pack"
	pluginWriteOriginTestFile(t, filepath.Join(env.ClaudeDir, "plugins", "installed_plugins.json"), `{"version":2,"plugins":{"pack@market":[{"scope":"user","installPath":"`+filepath.ToSlash(pluginRoot)+`"}]}}`)
	pluginWriteOriginTestFile(t, filepath.Join(env.ClaudeDir, "plugins", "known_marketplaces.json"), `{"market":{"source":{"source":"github","repo":"acme/plugins"},"installLocation":"`+filepath.ToSlash(filepath.Join(env.Home, "missing"))+`"}}`)
	candidate := newOriginPluginCache(env).candidate(skill)
	if candidate.repo != "https://github.com/acme/plugins" || candidate.skillPath != "" || candidate.installer != "market" {
		t.Fatalf("candidate = %#v", candidate)
	}
}

func TestClaudePluginOriginObjectSource(t *testing.T) {
	env, skill, pluginRoot := pluginOriginFixture(t, ProviderClaude)
	skill.Plugin = "pack"
	pluginWriteOriginTestFile(t, filepath.Join(env.ClaudeDir, "plugins", "installed_plugins.json"), `{"version":2,"plugins":{"pack@market":[{"scope":"user","installPath":"`+filepath.ToSlash(pluginRoot)+`"}]}}`)
	marketRoot := filepath.Join(env.Home, "market")
	pluginWriteOriginTestFile(t, filepath.Join(env.ClaudeDir, "plugins", "known_marketplaces.json"), `{"market":{"source":{"source":"directory"},"installLocation":"`+filepath.ToSlash(marketRoot)+`"}}`)
	pluginWriteOriginTestFile(t, filepath.Join(marketRoot, ".claude-plugin", "marketplace.json"), `{"plugins":[{"name":"pack","source":{"source":"git","url":"https://github.com/acme/plugins?secret=x","path":"packs/core"}}]}`)
	candidate := newOriginPluginCache(env).candidate(skill)
	if candidate.repo != "https://github.com/acme/plugins" || candidate.skillPath != "packs/core/skills/release" {
		t.Fatalf("candidate = %#v", candidate)
	}
}

func TestClaudePluginOriginEndToEnd(t *testing.T) {
	env := claudeTestEnv(t)
	pluginRoot := filepath.Join(env.Home, "plugins", "pack")
	claudeWriteSkill(t, filepath.Join(pluginRoot, "skills", "release"), "release", "plugin release")
	claudeWriteFile(t, filepath.Join(pluginRoot, ".claude-plugin", "plugin.json"), `{"repository":"https://github.com/acme/pack/tree/main/plugin","version":"3.0.0"}`)
	claudeWriteFile(t, filepath.Join(env.ClaudeDir, "plugins", "installed_plugins.json"), `{"version":2,"plugins":{"pack@market":[{"scope":"user","installPath":"`+filepath.ToSlash(pluginRoot)+`","version":"3.1.0"}]}}`)
	summary, err := ScanClaude(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	origin := claudeFindSkill(t, summary, "release", "pack").Origin
	if origin.Repository != "https://github.com/acme/pack" || origin.SkillPath != "plugin/skills/release" || origin.Branch != "" || origin.Version != "3.0.0" || origin.Installer != "market" {
		t.Fatalf("origin = %#v", origin)
	}
}

func TestCodexPluginOriginInstaller(t *testing.T) {
	env := codexTestEnv(t)
	pluginRoot := filepath.Join(env.CodexHome, "plugins", "cache", "market", "pack", "2.0.0")
	codexWriteSkill(t, filepath.Join(pluginRoot, "skills", "release"), "release", "release")
	codexWriteFile(t, filepath.Join(pluginRoot, ".codex-plugin", "plugin.json"), `{"repository":"https://github.com/acme/pack","version":"2.0.0"}`)
	codexWriteFile(t, filepath.Join(env.CodexHome, "config.toml"), "[plugins.\"pack@market\"]\nenabled = true\n")
	summary, err := ScanCodex(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	origin := codexFindSkill(t, summary, "release", "pack").Origin
	if origin.Repository != "https://github.com/acme/pack" || origin.Installer != "market" || origin.Version != "2.0.0" {
		t.Fatalf("origin = %#v", origin)
	}
}

func TestGrokPluginOriginRegistryEndToEnd(t *testing.T) {
	env := grokTestEnv(t)
	base := filepath.Join(env.GrokHome, "installed-plugins")
	pluginRoot := filepath.Join(base, "checkout", "grok")
	grokWriteSkill(t, filepath.Join(pluginRoot, "skills", "release"), "release", "plugin release")
	grokWriteFile(t, filepath.Join(pluginRoot, ".grok-plugin", "plugin.json"), `{"homepage":"https://docs.example.com/grok","version":"manifest-version"}`)
	grokWriteFile(t, filepath.Join(base, "registry.json"), `{"version":1,"repos":{"repo-id":{"kind":{"type":"gIt","url":"ssh://git@github.com/acme/repo.git?token=x#frag","git_ref":"ABCDEF1","commit":"0123456789abcdef","subdir":"grok"},"installed_at":"2026-10-15T01:02:03Z","updated_at":"2026-10-16T01:02:03.123Z","path":"checkout","marketplace":{"source_display_name":"Grok Market"},"plugins":{"acme":{"subdir":"grok","version":"4.5.6"}}}}}`)
	grokWriteFile(t, filepath.Join(env.GrokHome, "config.toml"), "[plugins]\nenabled = [\"acme\"]\n")
	summary, err := ScanGrok(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	skill := grokFindSkill(t, summary, "release", "acme")
	if skill.Origin.Repository != "https://github.com/acme/repo" || skill.Origin.SkillPath != "grok/skills/release" || skill.Origin.Branch != "" || skill.Origin.Version != "4.5.6" || skill.Origin.Installer != "Grok Market" || skill.Origin.Homepage != "https://docs.example.com/grok" {
		t.Fatalf("origin = %#v", skill.Origin)
	}
	if skill.Origin.InstalledAt.Format(time.RFC3339Nano) != "2026-10-15T01:02:03Z" || skill.Origin.UpdatedAt.Format(time.RFC3339Nano) != "2026-10-16T01:02:03.123Z" {
		t.Fatalf("origin times = %#v", skill.Origin)
	}
}

func TestPluginOriginPathAllowsCurrentDirectoryPrefix(t *testing.T) {
	if got := joinOriginPath(".", "skills/release"); got != "skills/release" {
		t.Fatalf("path = %q", got)
	}
	if got := joinOriginPath("plugins/../escape", "skills/release"); got != "" {
		t.Fatalf("unsafe path = %q", got)
	}
}

func pluginOriginFixture(t *testing.T, provider Provider) (Env, Skill, string) {
	t.Helper()
	env := pluginOriginEnv(t)
	pluginRoot := filepath.Join(env.Home, "plugins", string(provider))
	skillFile := filepath.Join(pluginRoot, "skills", "release", "SKILL.md")
	pluginWriteOriginTestFile(t, skillFile, "---\nname: release\ndescription: release\n---\n")
	return env, pluginOriginSkill(provider, pluginRoot, skillFile, "pack"), pluginRoot
}

func pluginOriginEnv(t *testing.T) Env {
	t.Helper()
	home := t.TempDir()
	return Env{Home: home, CodexHome: filepath.Join(home, ".codex"), ClaudeDir: filepath.Join(home, ".claude"), GrokHome: filepath.Join(home, ".grok"), GOOS: runtime.GOOS}
}

func pluginOriginSkill(provider Provider, root, skillFile, plugin string) Skill {
	return Skill{Provider: provider, File: skillFile, RealFile: skillFile, Dir: filepath.Dir(skillFile), Root: filepath.Join(root, "skills"), Scope: ScopePlugin, Plugin: plugin}
}

func pluginWriteOriginTestFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}
