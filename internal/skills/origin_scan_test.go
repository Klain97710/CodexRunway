package skills

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestScanCodexFillsGitOriginForValidAndInvalidFrontmatter(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires privileges on Windows")
	}
	home := t.TempDir()
	repo := filepath.Join(home, "checkout")
	writeOriginCoreTestFile(t, filepath.Join(repo, ".git", "config"), "[remote \"origin\"]\nurl=git@github.com:acme/skills.git\n")
	writeOriginCoreTestFile(t, filepath.Join(repo, ".git", "HEAD"), "ref: refs/heads/main\n")
	writeOriginCoreTestFile(t, filepath.Join(repo, "skills", "valid", "SKILL.md"), "---\nname: valid\ndescription: test\nhomepage: https://docs.example.com\n---\n")
	writeOriginCoreTestFile(t, filepath.Join(repo, "skills", "invalid", "SKILL.md"), "not frontmatter")
	root := filepath.Join(home, ".codex", "skills")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"valid", "invalid"} {
		if err := os.Symlink(filepath.Join(repo, "skills", name), filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	summary, err := ScanCodex(context.Background(), Env{Home: home, GOOS: "windows"})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"valid", "invalid"} {
		skill := codexFindSkill(t, summary, name, "")
		if skill.Origin.Repository != "https://github.com/acme/skills" || skill.Origin.From != OriginGit || skill.Origin.Branch != "main" || skill.Origin.SkillPath != "skills/"+name {
			t.Fatalf("%s origin = %#v", name, skill.Origin)
		}
	}
}
