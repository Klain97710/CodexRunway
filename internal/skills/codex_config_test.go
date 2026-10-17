package skills

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestCodexSelectorPrecedenceTable(t *testing.T) {
	tests := []struct {
		name       string
		collection bool
		config     func(string) string
		wantState  State
		wantReason Reason
	}{
		{
			name: "later path enables",
			config: func(path string) string {
				return fmt.Sprintf("[[skills.config]]\nname='target'\nenabled=false\n[[skills.config]]\npath=%q\nenabled=true\n", path)
			},
			wantState: StateOn,
		},
		{
			name: "later name enables",
			config: func(path string) string {
				return fmt.Sprintf("[[skills.config]]\npath=%q\nenabled=false\n[[skills.config]]\nname='target'\nenabled=true\n", path)
			},
			wantState: StateOn,
		},
		{
			name:       "bare name misses namespace",
			collection: true,
			config:     func(string) string { return "[[skills.config]]\nname='target'\nenabled=false\n" },
			wantState:  StateOn,
		},
		{
			name:       "invoke name matches namespace",
			collection: true,
			config:     func(string) string { return "[[skills.config]]\nname='suite:target'\nenabled=false\n" },
			wantState:  StateOff,
			wantReason: ReasonConfigName,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			env := codexTestEnv(t)
			dir := filepath.Join(env.CodexHome, "skills", "target")
			plugin := ""
			if test.collection {
				dir = filepath.Join(env.CodexHome, "skills", "suite", "skills", "target")
				plugin = "suite"
				codexWriteFile(t, filepath.Join(env.CodexHome, "skills", "suite", ".codex-plugin", "plugin.json"), `{"name":"suite"}`)
			}
			file := codexWriteSkill(t, dir, "target", "target")
			codexWriteFile(t, filepath.Join(env.CodexHome, "config.toml"), test.config(file))
			summary, err := ScanCodex(context.Background(), env)
			if err != nil {
				t.Fatal(err)
			}
			skill := codexFindSkill(t, summary, "target", plugin)
			if skill.State != test.wantState || skill.Reason != test.wantReason {
				t.Fatalf("state = %s/%s, want %s/%s", skill.State, skill.Reason, test.wantState, test.wantReason)
			}
		})
	}
}

func TestCodexConfigCannotReviveInvalidOrBundled(t *testing.T) {
	env := codexTestEnv(t)
	invalid := filepath.Join(env.CodexHome, "skills", "invalid", "SKILL.md")
	codexWriteFile(t, invalid, "not frontmatter")
	codexWriteSkill(t, filepath.Join(env.CodexHome, "skills", ".system", "builtin"), "builtin", "builtin")
	systemInvalid := filepath.Join(env.CodexHome, "skills", ".system", "invalid-system", "SKILL.md")
	codexWriteFile(t, systemInvalid, "not frontmatter")
	codexWriteFile(t, filepath.Join(env.CodexHome, "config.toml"), fmt.Sprintf(`
[[skills.config]]
path=%q
enabled=false
[[skills.config]]
path=%q
enabled=true
[[skills.config]]
name="builtin"
enabled=true
[skills.bundled]
enabled=false
`, invalid, invalid))
	summary, err := ScanCodex(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	invalidSkill := codexFindSkill(t, summary, "invalid", "")
	if invalidSkill.State != StateNotLoaded || invalidSkill.Reason != ReasonInvalidFront {
		t.Fatalf("invalid skill revived: %#v", invalidSkill)
	}
	builtin := codexFindSkill(t, summary, "builtin", "")
	if builtin.State != StateOff || builtin.Reason != ReasonBundledOff {
		t.Fatalf("bundled rule was overridden: %#v", builtin)
	}
	invalidSystemSkill := codexFindSkill(t, summary, "invalid-system", "")
	if invalidSystemSkill.State != StateNotLoaded || invalidSystemSkill.Reason != ReasonInvalidFront {
		t.Fatalf("bundled off hid invalid frontmatter: %#v", invalidSystemSkill)
	}
	if _, err := os.Stat(invalid); err != nil {
		t.Fatal(err)
	}
}

func TestCodexCursorManifestNamespacesWithoutCollectionRemoval(t *testing.T) {
	env := codexTestEnv(t)
	collection := filepath.Join(env.CodexHome, "skills", "cursor-package")
	codexWriteFile(t, filepath.Join(collection, ".cursor-plugin", "plugin.json"), `{"name":"cursor-suite"}`)
	codexWriteSkill(t, filepath.Join(collection, "skills", "target"), "target", "target")
	summary, err := ScanCodex(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	skill := codexFindSkill(t, summary, "target", "cursor-suite")
	if skill.InvokeName != "cursor-suite:target" || skill.Removal.Path != skill.Dir || skill.Removal.IsLink {
		t.Fatalf("cursor namespace/removal = %#v", skill)
	}
}
