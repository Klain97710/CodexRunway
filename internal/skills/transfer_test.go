package skills

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestTransferSkillCopyUsesCustomRootsAndIsDiscoverable(t *testing.T) {
	env := transferTestEnv(t)
	sourceDir := filepath.Join(env.ClaudeDir, "skills", "demo")
	writeTransferSkill(t, sourceDir, "demo")
	script := filepath.Join(sourceDir, "scripts", "run.sh")
	writeTransferFile(t, script, "#!/bin/sh\n", 0o755)
	if runtime.GOOS != "windows" {
		if err := os.Chmod(filepath.Dir(script), 0o555); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(filepath.Dir(script), 0o755) })
	}
	source := transferTestSkill(t, env, ProviderClaude, "demo")

	result, err := TransferSkill(env, TransferRequest{Skill: source, Target: ProviderCodex}, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(env.CodexHome, "skills", "demo")
	if result.Path != want {
		t.Fatalf("Path = %q, want %q", result.Path, want)
	}
	info, err := os.Stat(filepath.Join(want, "scripts", "run.sh"))
	if err != nil || runtime.GOOS != "windows" && info.Mode().Perm() != 0o755 {
		t.Fatalf("executable mode = %v, %v", info, err)
	}
	summary, err := ScanCodex(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(summary.Skills, func(skill Skill) bool { return skill.Name == "demo" && skill.File == filepath.Join(want, "SKILL.md") }) {
		t.Fatalf("target scan did not discover copied skill: %+v", summary.Skills)
	}
}

func TestTransferSkillPreservesSafeChildLink(t *testing.T) {
	env := transferTestEnv(t)
	sourceDir := filepath.Join(env.ClaudeDir, "skills", "safe-link")
	writeTransferSkill(t, sourceDir, "safe-link")
	writeTransferFile(t, filepath.Join(sourceDir, "scripts", "run.sh"), "#!/bin/sh\n", 0o755)
	if err := os.Symlink(filepath.Join("scripts", "run.sh"), filepath.Join(sourceDir, "run")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	source := transferTestSkill(t, env, ProviderClaude, "safe-link")
	result, err := TransferSkill(env, TransferRequest{Skill: source, Target: ProviderCodex}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if link, err := os.Readlink(filepath.Join(result.Path, "run")); err != nil || link != filepath.Join("scripts", "run.sh") {
		t.Fatalf("copied link = %q, %v", link, err)
	}
}

func TestTransferSkillCopiesTopLevelUserLink(t *testing.T) {
	env := transferTestEnv(t)
	realDir := filepath.Join(env.Home, "linked-skill")
	writeTransferSkill(t, realDir, "linked")
	link := filepath.Join(env.ClaudeDir, "skills", "linked")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(realDir, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	source := transferTestSkill(t, env, ProviderClaude, "linked")
	if !CanMoveSkill(source) {
		t.Fatal("top-level user link should be individually movable")
	}
	result, err := TransferSkill(env, TransferRequest{Skill: source, Target: ProviderGrok}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if info, err := os.Lstat(result.Path); err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("target should be a copied directory: %v, %v", info, err)
	}
}

func TestTransferSkillMoveArchivesSource(t *testing.T) {
	env := transferTestEnv(t)
	writeTransferSkill(t, filepath.Join(env.CodexHome, "skills", "move-me"), "move-me")
	summary, err := ScanCodex(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	source := findTransferSkill(t, summary, "move-me")
	result, err := TransferSkill(env, TransferRequest{
		Skill: source, Roots: summary.Roots, Target: ProviderGrok, Move: true,
	}, func(string) error { return errors.New("unexpected system trash") })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(source.Dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("source still exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(result.Path, "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	entries, err := NewTrash(filepath.Join(env.Home, ".quosparr"), env.Home).List(context.Background())
	if err != nil || len(entries) != 1 || entries[0].Original != source.Dir {
		t.Fatalf("trash entries = %+v, %v", entries, err)
	}
}

func TestTransferSkillConflictLeavesBothTreesUnchanged(t *testing.T) {
	env := transferTestEnv(t)
	sourceDir := filepath.Join(env.ClaudeDir, "skills", "same")
	targetDir := filepath.Join(env.GrokHome, "skills", "same")
	writeTransferSkill(t, sourceDir, "same")
	writeTransferSkill(t, targetDir, "existing")
	source := transferTestSkill(t, env, ProviderClaude, "same")

	_, err := TransferSkill(env, TransferRequest{Skill: source, Target: ProviderGrok}, nil)
	if !errors.Is(err, ErrTransferConflict) {
		t.Fatalf("error = %v", err)
	}
	assertTransferFileContains(t, filepath.Join(sourceDir, "SKILL.md"), "name: same")
	assertTransferFileContains(t, filepath.Join(targetDir, "SKILL.md"), "name: existing")
}

func TestTransferSkillRejectsUnsafeChildLinkAndCleansTarget(t *testing.T) {
	env := transferTestEnv(t)
	sourceDir := filepath.Join(env.ClaudeDir, "skills", "unsafe")
	writeTransferSkill(t, sourceDir, "unsafe")
	secret := filepath.Join(env.ClaudeDir, "secret")
	writeTransferFile(t, secret, "private", 0o600)
	if err := os.Symlink(filepath.Join("..", "..", "secret"), filepath.Join(sourceDir, "secret-link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	source := transferTestSkill(t, env, ProviderClaude, "unsafe")

	_, err := TransferSkill(env, TransferRequest{Skill: source, Target: ProviderCodex}, nil)
	if !errors.Is(err, ErrTransferUnsafeLink) {
		t.Fatalf("error = %v", err)
	}
	if _, statErr := os.Lstat(filepath.Join(env.CodexHome, "skills", "unsafe")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("partial target remains: %v", statErr)
	}
}

func TestTransferSkillCollectionCopyButNotMove(t *testing.T) {
	env := transferTestEnv(t)
	collection := filepath.Join(env.ClaudeDir, "skills", "bundle")
	writeTransferFile(t, filepath.Join(collection, ".claude-plugin", "plugin.json"), `{"name":"bundle"}`, 0o644)
	writeTransferSkill(t, filepath.Join(collection, "skills", "one"), "one")
	writeTransferSkill(t, filepath.Join(collection, "skills", "two"), "two")
	summary, err := ScanClaude(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	source := findTransferSkill(t, summary, "bundle:one")
	if CanMoveSkill(source) {
		t.Fatal("one item from a collection must not be movable")
	}
	if _, err := TransferSkill(env, TransferRequest{Skill: source, Roots: summary.Roots, Target: ProviderGrok, Move: true}, func(string) error { return nil }); !errors.Is(err, ErrTransferNotMovable) {
		t.Fatalf("move error = %v", err)
	}
	result, err := TransferSkill(env, TransferRequest{Skill: source, Target: ProviderGrok}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(result.Path) != "one" {
		t.Fatalf("copied collection path = %q", result.Path)
	}
	if _, err := os.Stat(filepath.Join(collection, "skills", "two", "SKILL.md")); err != nil {
		t.Fatalf("other collection item changed: %v", err)
	}
}

func TestTransferSkillMoveFailureReportsPartialResult(t *testing.T) {
	env := transferTestEnv(t)
	sourceDir := filepath.Join(env.ClaudeDir, "skills", "partial")
	writeTransferSkill(t, sourceDir, "partial")
	source := transferTestSkill(t, env, ProviderClaude, "partial")

	result, err := TransferSkill(env, TransferRequest{Skill: source, Target: ProviderCodex, Move: true}, func(string) error { return nil })
	if !errors.Is(err, ErrTransferPartial) {
		t.Fatalf("error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(result.Path, "SKILL.md")); err != nil {
		t.Fatalf("complete target missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(sourceDir, "SKILL.md")); err != nil {
		t.Fatalf("source missing after failed archive: %v", err)
	}
}

func TestTransferSkillRejectsChangedSourceAndTargetRootLink(t *testing.T) {
	env := transferTestEnv(t)
	sourceDir := filepath.Join(env.ClaudeDir, "skills", "changed")
	writeTransferSkill(t, sourceDir, "changed")
	source := transferTestSkill(t, env, ProviderClaude, "changed")
	writeTransferFile(t, source.File, "---\nname: changed\ndescription: changed\n---\n", 0o644)
	mtime := source.ModTime.Add(2 * time.Second)
	if err := os.Chtimes(source.File, mtime, mtime); err != nil {
		t.Fatal(err)
	}
	if _, err := TransferSkill(env, TransferRequest{Skill: source, Target: ProviderCodex}, nil); !errors.Is(err, ErrTransferSource) {
		t.Fatalf("changed source error = %v", err)
	}

	writeTransferSkill(t, sourceDir, "changed")
	source = transferTestSkill(t, env, ProviderClaude, "changed")
	outside := filepath.Join(env.Home, "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(env.CodexHome, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(env.CodexHome, "skills")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := TransferSkill(env, TransferRequest{Skill: source, Target: ProviderCodex}, nil); !errors.Is(err, ErrTransferOverlap) {
		t.Fatalf("target root link error = %v", err)
	}
}

func transferTestEnv(t *testing.T) Env {
	t.Helper()
	home := t.TempDir()
	return Env{Home: home, CodexHome: filepath.Join(home, "custom-codex"), ClaudeDir: filepath.Join(home, "custom-claude"), GrokHome: filepath.Join(home, "custom-grok"), GOOS: "windows", Now: time.Unix(1_800_000_000, 0)}
}

func transferTestSkill(t *testing.T, env Env, provider Provider, name string) Skill {
	t.Helper()
	var summary Summary
	var err error
	switch provider {
	case ProviderCodex:
		summary, err = ScanCodex(context.Background(), env)
	case ProviderClaude:
		summary, err = ScanClaude(context.Background(), env)
	case ProviderGrok:
		summary, err = ScanGrok(context.Background(), env)
	}
	if err != nil {
		t.Fatal(err)
	}
	return findTransferSkill(t, summary, name)
}

func findTransferSkill(t *testing.T, summary Summary, name string) Skill {
	t.Helper()
	for _, skill := range summary.Skills {
		if skill.InvokeName == name {
			return skill
		}
	}
	t.Fatalf("skill %q not found in %+v", name, summary.Skills)
	return Skill{}
}

func writeTransferSkill(t *testing.T, dir, name string) {
	t.Helper()
	writeTransferFile(t, filepath.Join(dir, "SKILL.md"), "---\nname: "+name+"\ndescription: test skill\n---\n", 0o644)
}

func writeTransferFile(t *testing.T, path, body string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
}

func assertTransferFileContains(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), want) {
		t.Fatalf("%s does not contain %q", path, want)
	}
}
