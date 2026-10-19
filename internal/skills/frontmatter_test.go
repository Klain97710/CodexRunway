package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseFrontmatterSupportedForms(t *testing.T) {
	data := []byte("---\nname: 'release''s helper' # note\ndescription: >-\n  first line\n  second line\nallowed-tools:\n- Read\n- 'Grep, Glob'\ndisallowed-tools: [Write, \"Shell, Exec\"]\nmetadata:\n  author: \"Quosparr\" # note\n  short-description: |+\n    short\n    text\nunknown:\n  nested: accepted\n---\nbody\n")
	front, start, err := parseFrontmatter(data)
	if err != nil {
		t.Fatal(err)
	}
	if front.field("name") != "release's helper" || front.field("description") != "first line second line" {
		t.Fatalf("unexpected scalar parsing: %#v", front.values)
	}
	if front.field("allowed-tools") != "Read, Grep, Glob" || front.field("disallowed-tools") != "Write, Shell, Exec" {
		t.Fatalf("unexpected list parsing: %#v", front.values)
	}
	if front.metadata["short-description"] != "short text" || front.nested["unknown"]["nested"] != "accepted" {
		t.Fatalf("unexpected mapping parsing: %#v", front.nested)
	}
	if string(data[start:]) != "body\n" {
		t.Fatalf("body starts at %d: %q", start, data[start:])
	}
}

func TestParseFrontmatterRejectsMalformedInput(t *testing.T) {
	for _, data := range []string{
		"name: no-delimiter\n",
		" ---\nname: indented-delimiter\n---\n",
		"---\nname: \"unterminated\n---\n",
		"---\ninvalid line\n---\n",
		"---\nname: [one, 'two]\n---\n",
	} {
		if _, _, err := parseFrontmatter([]byte(data)); err == nil {
			t.Fatalf("accepted malformed frontmatter %q", data)
		}
	}
}

func TestFrontmatterBlockMayContainIndentedDelimiter(t *testing.T) {
	for _, tc := range []struct {
		name  string
		style string
		eol   string
	}{
		{name: "literal LF", style: "|", eol: "\n"},
		{name: "folded LF", style: ">", eol: "\n"},
		{name: "literal CRLF", style: "|", eol: "\r\n"},
		{name: "folded CRLF", style: ">", eol: "\r\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			env, _ := normalizeEnv(Env{Home: home})
			file := filepath.Join(home, "skills", "block", "SKILL.md")
			content := strings.Join([]string{"---", "name: block", "description: " + tc.style, "  ---", "  text", "---", "Body", ""}, tc.eol)
			commonWriteFile(t, file, content)
			skill, issues, err := loadSkill(env, ProviderCodex, newRoot(env, filepath.Join(home, "skills"), ScopeUser), file)
			if err != nil || len(issues) != 0 || skill.Description != "--- text" {
				t.Fatalf("block description = %q, %#v, %v", skill.Description, issues, err)
			}
			body, truncated, err := ReadBody(skill)
			if err != nil || truncated || body != "Body"+tc.eol {
				t.Fatalf("block body = %q, %v, %v", body, truncated, err)
			}
		})
	}
}

func TestLoadSkillProviderDescriptionRulesAndFields(t *testing.T) {
	home := t.TempDir()
	env, err := normalizeEnv(Env{Home: home})
	if err != nil {
		t.Fatal(err)
	}
	rootPath := filepath.Join(home, "skills")
	root := newRoot(env, rootPath, ScopeUser)
	file := filepath.Join(rootPath, "sample", "SKILL.md")
	commonWriteFile(t, file, "---\nname: sample\nallowed-tools: []\nmetadata:\n  author: me\n---\n# Heading\n\nBody description.\n")
	root = newRoot(env, rootPath, ScopeUser)

	codex, issues, err := loadSkill(env, ProviderCodex, root, file)
	if err != nil {
		t.Fatal(err)
	}
	if codex.State != StateNotLoaded || codex.Reason != ReasonMissingDesc || len(issues) != 1 {
		t.Fatalf("Codex missing description = %#v, %#v", codex, issues)
	}
	if len(codex.Fields) != 2 || codex.Fields[0].Key != "allowed-tools" || codex.Fields[0].Value != "" {
		t.Fatalf("field whitelist/presence = %#v", codex.Fields)
	}

	claude, issues, err := loadSkill(env, ProviderClaude, root, file)
	if err != nil || claude.State != StateOn || len(issues) != 1 {
		t.Fatalf("Claude missing description = %#v, %#v, %v", claude, issues, err)
	}
	grok, issues, err := loadSkill(env, ProviderGrok, root, file)
	if err != nil || len(issues) != 0 || grok.Description != "Body description." {
		t.Fatalf("Grok body description = %#v, %#v, %v", grok, issues, err)
	}
}

func TestLoadSkillInvalidFrontmatterStillListsDirectory(t *testing.T) {
	home := t.TempDir()
	env, _ := normalizeEnv(Env{Home: home})
	file := filepath.Join(home, "skills", "folder-name", "SKILL.md")
	commonWriteFile(t, file, "---\nname: [broken\n---\nbody\n")
	skill, issues, err := loadSkill(env, ProviderCodex, newRoot(env, filepath.Join(home, "skills"), ScopeUser), file)
	if err != nil {
		t.Fatal(err)
	}
	if skill.Name != "folder-name" || skill.State != StateNotLoaded || skill.Reason != ReasonInvalidFront || len(issues) != 1 || issues[0].Kind != IssueInvalidFront {
		t.Fatalf("invalid skill = %#v, %#v", skill, issues)
	}
}

func TestLoadCodexInterfaceIsAtomic(t *testing.T) {
	home := t.TempDir()
	env, _ := normalizeEnv(Env{Home: home})
	dir := filepath.Join(home, "skills", "tool")
	file := filepath.Join(dir, "SKILL.md")
	commonWriteFile(t, file, "---\nname: tool\ndescription: base\n---\n")
	commonWriteFile(t, filepath.Join(dir, "agents", "openai.yaml"), "interface:\n  display_name: 'Tool UI'\n  short_description: >\n    concise\n    help\npolicy:\n  allow_implicit_invocation: true\n")
	skill, _, err := loadSkill(env, ProviderCodex, newRoot(env, filepath.Join(home, "skills"), ScopeUser), file)
	if err != nil {
		t.Fatal(err)
	}
	if skill.DisplayName != "Tool UI" || skill.ShortDescription != "concise help" || skill.Fields[len(skill.Fields)-1].Key != "policy.allow_implicit_invocation" {
		t.Fatalf("interface metadata = %#v", skill)
	}
	commonWriteFile(t, filepath.Join(dir, "agents", "openai.yaml"), "interface:\n  display_name: changed\npolicy:\n  allow_implicit_invocation: maybe\n")
	skill, _, err = loadSkill(env, ProviderCodex, newRoot(env, filepath.Join(home, "skills"), ScopeUser), file)
	if err != nil || skill.DisplayName != "" {
		t.Fatalf("invalid interface file was partially applied: %#v, %v", skill, err)
	}
}

func commonWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
