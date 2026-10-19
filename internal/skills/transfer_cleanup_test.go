package skills

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestTransferCleanupPreservesNewTargetContent(t *testing.T) {
	for _, test := range []struct {
		name    string
		replace bool
	}{
		{name: "new file"},
		{name: "replaced file", replace: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			rootPath := t.TempDir()
			skillDir := filepath.Join(rootPath, "demo")
			ownedPath := filepath.Join(skillDir, "owned.txt")
			writeTransferFile(t, ownedPath, "owned", 0o644)
			root, err := os.OpenRoot(rootPath)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			dirInfo, err := root.Lstat("demo")
			if err != nil {
				t.Fatal(err)
			}
			ownedInfo, err := root.Lstat(filepath.Join("demo", "owned.txt"))
			if err != nil {
				t.Fatal(err)
			}
			created := &transferTarget{root: root, name: "demo", created: []transferCreated{
				{path: "demo", info: dirInfo},
				{path: filepath.Join("demo", "owned.txt"), info: ownedInfo},
			}}

			writeTransferFile(t, filepath.Join(skillDir, "new.txt"), "new", 0o644)
			if test.replace {
				if err := os.Rename(ownedPath, filepath.Join(skillDir, "old-owned.txt")); err != nil {
					t.Fatal(err)
				}
				writeTransferFile(t, ownedPath, "replacement", 0o644)
			}
			if err := created.cleanup(); err == nil {
				t.Fatal("cleanup should report retained new target content")
			}
			assertTransferFileContains(t, filepath.Join(skillDir, "new.txt"), "new")
			if test.replace {
				assertTransferFileContains(t, ownedPath, "replacement")
			} else if _, err := os.Lstat(ownedPath); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("unchanged owned file should be removed: %v", err)
			}
		})
	}
}

func TestTransferSkillCopiesSafeSkillFileLink(t *testing.T) {
	env := transferTestEnv(t)
	sourceDir := filepath.Join(env.ClaudeDir, "skills", "linked-file")
	payload := filepath.Join(sourceDir, "payload", "SKILL.md")
	writeTransferFile(t, payload, "---\nname: linked-file\ndescription: linked file\n---\n", 0o644)
	if err := os.Symlink(filepath.Join("payload", "SKILL.md"), filepath.Join(sourceDir, "SKILL.md")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	summary, err := ScanClaude(t.Context(), env)
	if err != nil {
		t.Fatal(err)
	}
	var source Skill
	for _, skill := range summary.Skills {
		if skill.File == filepath.Join(sourceDir, "SKILL.md") {
			source = skill
			break
		}
	}
	if source.File == "" {
		t.Fatal("linked SKILL.md was not scanned")
	}
	result, err := TransferSkill(env, TransferRequest{Skill: source, Target: ProviderCodex}, nil)
	if err != nil {
		t.Fatal(err)
	}
	link, err := os.Readlink(filepath.Join(result.Path, "SKILL.md"))
	if err != nil || link != filepath.Join("payload", "SKILL.md") {
		t.Fatalf("copied SKILL.md link = %q, %v", link, err)
	}
	assertTransferFileContains(t, filepath.Join(result.Path, "SKILL.md"), "name: linked-file")
}
