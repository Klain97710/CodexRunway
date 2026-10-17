package skills

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestSetRemovalsCollectionAndCovers(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, "skills")
	collection := filepath.Join(root, "collection")
	commonWriteFile(t, filepath.Join(collection, ".git", "HEAD"), "ref")
	commonWriteFile(t, filepath.Join(collection, "nested", ".codex-plugin", "plugin.json"), "{}")
	firstFile := filepath.Join(collection, "nested", "one", "SKILL.md")
	secondFile := filepath.Join(collection, "two", "SKILL.md")
	commonWriteFile(t, firstFile, "---\nname: one\ndescription: one\n---\n")
	commonWriteFile(t, secondFile, "---\nname: two\ndescription: two\n---\n")
	summary := Summary{Skills: []Skill{
		{Scope: ScopeUser, Root: root, Dir: filepath.Dir(firstFile), File: firstFile, InvokeName: "one"},
		{Scope: ScopeUser, Root: root, Dir: filepath.Dir(secondFile), File: secondFile, InvokeName: "two"},
	}}
	finishSummary(&summary)
	for _, skill := range summary.Skills {
		if !skill.Deletable || skill.Removal.Path != collection || len(skill.Removal.Covers) != 2 {
			t.Fatalf("removal = %#v", skill.Removal)
		}
	}
}

func TestDeleteValidationAndTrashError(t *testing.T) {
	home := t.TempDir()
	rootPath := filepath.Join(home, "skills")
	file := filepath.Join(rootPath, "one", "SKILL.md")
	commonWriteFile(t, file, "---\nname: one\ndescription: one\n---\n")
	info, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	root := RootInfo{Path: rootPath, Scope: ScopeUser, Exists: true}
	skill := Skill{Scope: ScopeUser, File: file, Dir: filepath.Dir(file), Root: rootPath, ModTime: info.ModTime(), Deletable: true, Removal: Removal{Path: filepath.Dir(file)}}
	called := ""
	if err := Delete(skill, []RootInfo{root}, func(path string) error { called = path; return nil }); err != nil || called != skill.Removal.Path {
		t.Fatalf("delete = %q, %v", called, err)
	}
	variant := skill
	variant.Removal.Path = filepath.Join(skill.Removal.Path, "..", filepath.Base(skill.Removal.Path))
	if err := Delete(variant, []RootInfo{root}, func(path string) error {
		if path != filepath.Clean(variant.Removal.Path) {
			t.Fatalf("trash received unclean path %q", path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := Delete(Skill{}, []RootInfo{root}, func(string) error { return nil }); !errors.Is(err, ErrNotDeletable) {
		t.Fatalf("non-deletable error = %v", err)
	}
	forged := skill
	forged.Removal.Path = filepath.Join(home, "other")
	if err := os.MkdirAll(forged.Removal.Path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := Delete(forged, []RootInfo{root, {Path: forged.Removal.Path, Scope: ScopeUser, Exists: true}}, func(string) error { t.Fatal("trash called"); return nil }); !errors.Is(err, ErrOutsideRoot) {
		t.Fatalf("forged removal error = %v", err)
	}
	rootSkill := skill
	rootSkill.Removal.Path = rootPath
	if err := Delete(rootSkill, []RootInfo{root}, func(string) error { t.Fatal("trash called"); return nil }); !errors.Is(err, ErrOutsideRoot) {
		t.Fatalf("root removal error = %v", err)
	}
	want := errors.New("trash unavailable")
	if err := Delete(skill, []RootInfo{root}, func(string) error { return want }); !errors.Is(err, want) {
		t.Fatalf("trash error lost wrapping: %v", err)
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("trash failure removed source: %v", err)
	}
	nonUser := skill
	nonUser.Scope = ScopeAdmin
	if err := Delete(nonUser, []RootInfo{root}, func(string) error { t.Fatal("trash called"); return nil }); !errors.Is(err, ErrNotDeletable) {
		t.Fatalf("non-user error = %v", err)
	}

	changedTime := info.ModTime().Add(2 * time.Second)
	if err := os.Chtimes(file, changedTime, changedTime); err != nil {
		t.Fatal(err)
	}
	if err := Delete(skill, []RootInfo{root}, func(string) error { t.Fatal("trash called"); return nil }); !errors.Is(err, ErrChanged) {
		t.Fatalf("changed file error = %v", err)
	}
	if err := os.RemoveAll(skill.Removal.Path); err != nil {
		t.Fatal(err)
	}
	if err := Delete(skill, []RootInfo{root}, func(string) error { t.Fatal("trash called"); return nil }); !errors.Is(err, ErrChanged) {
		t.Fatalf("missing removal error = %v", err)
	}
}

func TestDeleteLinkAndBrokenLink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires privileges on Windows")
	}
	home := t.TempDir()
	rootPath := filepath.Join(home, "skills")
	targetFile := filepath.Join(home, "target", "SKILL.md")
	commonWriteFile(t, targetFile, "---\nname: linked\ndescription: linked\n---\n")
	if err := os.MkdirAll(rootPath, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(rootPath, "linked")
	if err := os.Symlink(filepath.Dir(targetFile), link); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(link, "SKILL.md")
	info, _ := os.Stat(file)
	root := RootInfo{Path: rootPath, Scope: ScopeUser, Exists: true}
	skill := Skill{Scope: ScopeUser, File: file, Dir: filepath.Dir(file), Root: rootPath, LinkPath: link, ModTime: info.ModTime(), Deletable: true, Removal: Removal{Path: link, IsLink: true}}
	if err := Delete(skill, nil, func(path string) error {
		if path != link {
			t.Fatalf("trashed %q", path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	changed := info.ModTime().Add(2 * time.Second)
	if err := os.Chtimes(targetFile, changed, changed); err != nil {
		t.Fatal(err)
	}
	if err := Delete(skill, nil, func(string) error { t.Fatal("trash called"); return nil }); !errors.Is(err, ErrChanged) {
		t.Fatalf("link target mtime error = %v", err)
	}
	if err := os.Remove(targetFile); err != nil {
		t.Fatal(err)
	}
	if err := Delete(skill, []RootInfo{root}, func(string) error { return nil }); err != nil {
		t.Fatalf("dangling skill link = %v", err)
	}

	broken := filepath.Join(rootPath, "broken")
	if err := os.Symlink(filepath.Join(home, "gone"), broken); err != nil {
		t.Fatal(err)
	}
	summary := Summary{Issues: []Issue{{Kind: IssueBrokenLink, Path: broken, Removable: true}}}
	brokenVariant := filepath.Join(rootPath, "unused", "..", "broken")
	if err := DeleteBrokenLink(summary, brokenVariant, func(path string) error {
		if path != broken {
			t.Fatalf("trashed %q", path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := DeleteBrokenLink(Summary{Roots: []RootInfo{root}}, broken, func(string) error { return nil }); !errors.Is(err, ErrNotDeletable) {
		t.Fatalf("unlisted broken link error = %v", err)
	}
	commonWriteFile(t, filepath.Join(home, "gone"), "repaired")
	if err := DeleteBrokenLink(summary, broken, func(string) error { t.Fatal("trash called"); return nil }); !errors.Is(err, ErrChanged) {
		t.Fatalf("repaired link error = %v", err)
	}
	if err := os.Remove(broken); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(broken, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := DeleteBrokenLink(summary, broken, func(string) error { t.Fatal("trash called"); return nil }); !errors.Is(err, ErrChanged) {
		t.Fatalf("changed broken-link type error = %v", err)
	}
	if err := os.Remove(broken); err != nil {
		t.Fatal(err)
	}
	if err := DeleteBrokenLink(summary, broken, func(string) error { t.Fatal("trash called"); return nil }); !errors.Is(err, ErrChanged) {
		t.Fatalf("missing broken-link error = %v", err)
	}
}

func TestDeleteRejectsChangedLinkType(t *testing.T) {
	home := t.TempDir()
	rootPath := filepath.Join(home, "skills")
	path := filepath.Join(rootPath, "one")
	commonWriteFile(t, filepath.Join(path, "SKILL.md"), "---\nname: one\ndescription: one\n---\n")
	info, _ := os.Stat(filepath.Join(path, "SKILL.md"))
	skill := Skill{Scope: ScopeUser, Root: rootPath, File: filepath.Join(path, "SKILL.md"), ModTime: info.ModTime(), Deletable: true, Removal: Removal{Path: path, IsLink: true}}
	err := Delete(skill, []RootInfo{{Path: rootPath, Scope: ScopeUser, Exists: true}}, func(string) error { t.Fatal("trash called"); return nil })
	if !errors.Is(err, ErrChanged) {
		t.Fatalf("changed type error = %v", err)
	}
}

func TestDeleteRejectsParentSymlinkOutsideRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires privileges on Windows")
	}
	home := t.TempDir()
	rootPath := filepath.Join(home, "skills")
	outside := filepath.Join(home, "outside")
	commonWriteFile(t, filepath.Join(outside, "one", "SKILL.md"), "---\nname: one\ndescription: one\n---\n")
	if err := os.MkdirAll(rootPath, 0o700); err != nil {
		t.Fatal(err)
	}
	parentLink := filepath.Join(rootPath, "swapped")
	if err := os.Symlink(outside, parentLink); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(parentLink, "one", "SKILL.md")
	info, _ := os.Stat(file)
	skill := Skill{Scope: ScopeUser, Root: rootPath, File: file, ModTime: info.ModTime(), Deletable: true, Removal: Removal{Path: filepath.Dir(file)}}
	err := Delete(skill, []RootInfo{{Path: rootPath, Scope: ScopeUser, Exists: true}}, func(string) error { t.Fatal("trash called"); return nil })
	if !errors.Is(err, ErrOutsideRoot) {
		t.Fatalf("outside parent error = %v", err)
	}
}
