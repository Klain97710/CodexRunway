package skills

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestReadBodyBoundaries(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "SKILL.md")
	commonWriteFile(t, file, "---\nname: bad: [still body-readable\n---\nmarkdown\n")
	body, truncated, err := ReadBody(Skill{File: file})
	if err != nil || truncated || body != "markdown\n" {
		t.Fatalf("invalid frontmatter body = %q, %v, %v", body, truncated, err)
	}
	commonWriteFile(t, file, "plain body\n")
	body, truncated, err = ReadBody(Skill{File: file})
	if err != nil || truncated || body != "plain body\n" {
		t.Fatalf("plain body = %q, %v, %v", body, truncated, err)
	}
	commonWriteFile(t, file, strings.Repeat("x", MaxBodyBytes+20))
	body, truncated, err = ReadBody(Skill{File: file})
	if err != nil || !truncated || len(body) != MaxBodyBytes {
		t.Fatalf("truncated body length = %d, %v, %v", len(body), truncated, err)
	}
}

func TestReadBodyCRLFBoundary(t *testing.T) {
	file := filepath.Join(t.TempDir(), "SKILL.md")
	commonWriteFile(t, file, "---\r\nname: crlf\r\ndescription: test\r\n---\r\nbody\r\n")
	body, truncated, err := ReadBody(Skill{File: file})
	if err != nil || truncated || body != "body\r\n" {
		t.Fatalf("CRLF body = %q, %v, %v", body, truncated, err)
	}
}

func TestReadBodyRejectsOversizeFrontmatterAndNonSkillFiles(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "SKILL.md")
	commonWriteFile(t, file, "---\ndescription: |\n  "+strings.Repeat("x", MaxFrontmatterBytes)+"\n---\nbody")
	if _, _, err := ReadBody(Skill{File: file}); !errors.Is(err, errInvalidFrontmatter) {
		t.Fatalf("oversize frontmatter error = %v", err)
	}
	credential := filepath.Join(dir, "auth.json")
	commonWriteFile(t, credential, "private-content")
	if _, _, err := ReadBody(Skill{File: credential}); !errors.Is(err, errNotSkillFile) {
		t.Fatalf("credential path error = %v", err)
	}
	env, _ := normalizeEnv(Env{Home: dir})
	if _, _, err := loadSkill(env, ProviderCodex, RootInfo{Path: dir, Scope: ScopeUser}, credential); !errors.Is(err, errNotSkillFile) {
		t.Fatalf("load credential path error = %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "folder", "SKILL.md"), 0o700); err != nil {
		if !os.IsNotExist(err) {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(dir, "folder", "SKILL.md"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := ReadBody(Skill{File: filepath.Join(dir, "folder", "SKILL.md")}); !errors.Is(err, errNotRegular) {
		t.Fatalf("directory error = %v", err)
	}
}

func TestReadBodyRejectsCredentialSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires privileges on Windows")
	}
	dir := t.TempDir()
	credential := filepath.Join(dir, "auth.json")
	commonWriteFile(t, credential, "private-content")
	link := filepath.Join(dir, "SKILL.md")
	if err := os.Symlink(credential, link); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ReadBody(Skill{File: link}); !errors.Is(err, errNotSkillFile) {
		t.Fatalf("credential symlink error = %v", err)
	}
}

func TestMarkSharedIsStableAndProviderUnique(t *testing.T) {
	real := filepath.Join(t.TempDir(), "SKILL.md")
	codex := &Summary{Provider: ProviderCodex, Skills: []Skill{{RealFile: real}, {RealFile: real}}}
	claude := &Summary{Provider: ProviderClaude, Skills: []Skill{{RealFile: real}}}
	grok := &Summary{Provider: ProviderGrok, Skills: []Skill{{RealFile: real}}}
	MarkShared(codex, claude, grok)
	MarkShared(codex, claude, grok)
	for _, skill := range codex.Skills {
		if len(skill.SharedWith) != 2 || skill.SharedWith[0] != ProviderClaude || skill.SharedWith[1] != ProviderGrok {
			t.Fatalf("Codex sharing = %#v", skill.SharedWith)
		}
	}
	if len(claude.Skills[0].SharedWith) != 2 || claude.Skills[0].SharedWith[0] != ProviderCodex || claude.Skills[0].SharedWith[1] != ProviderGrok {
		t.Fatalf("Claude sharing = %#v", claude.Skills[0].SharedWith)
	}
}
