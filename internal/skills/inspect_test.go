package skills

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestInspectCountsAndSortsEntries(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "skill")
	inspectWriteFile(t, filepath.Join(dir, "z.txt"), "1234")
	inspectWriteFile(t, filepath.Join(dir, "assets", "a.txt"), "12")

	detail, err := Inspect(context.Background(), Skill{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	wantPaths := []string{"assets", "assets/a.txt", "z.txt"}
	if detail.FileCount != 2 || detail.DirCount != 1 || detail.Size != 6 || detail.Truncated || len(detail.Files) != len(wantPaths) {
		t.Fatalf("detail = %#v", detail)
	}
	for i, want := range wantPaths {
		if detail.Files[i].Path != want {
			t.Fatalf("files[%d] = %#v, want %q", i, detail.Files[i], want)
		}
	}
	if !detail.Files[0].Dir || detail.Files[1].Dir || detail.Files[1].Size != 2 || detail.Files[2].Size != 4 || detail.Files[1].ModTime.IsZero() {
		t.Fatalf("entry metadata = %#v", detail.Files)
	}
}

func TestInspectListsLinksWithoutFollowingThem(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires privileges on Windows")
	}
	home := t.TempDir()
	dir := filepath.Join(home, "skill")
	target := filepath.Join(home, "target")
	for i := 0; i < 20; i++ {
		inspectWriteFile(t, filepath.Join(target, string(rune('a'+i))), "private")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, "linked")); err != nil {
		t.Fatal(err)
	}

	detail, err := Inspect(context.Background(), Skill{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if detail.FileCount != 0 || detail.DirCount != 0 || detail.Size != 0 || len(detail.Files) != 1 || !detail.Files[0].Link || detail.Files[0].Dir || detail.Files[0].Size != 0 {
		t.Fatalf("linked detail = %#v", detail)
	}
}

func TestInspectSkipsKnownDirectoriesAndDSStore(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "skill")
	inspectWriteFile(t, filepath.Join(dir, ".git", "config"), "hidden")
	inspectWriteFile(t, filepath.Join(dir, "node_modules", "package", "index.js"), "hidden")
	inspectWriteFile(t, filepath.Join(dir, ".DS_Store"), "hidden")
	inspectWriteFile(t, filepath.Join(dir, "keep.txt"), "ok")
	inspectWriteFile(t, filepath.Join(dir, "visible", ".DS_Store", "kept"), "x")

	detail, err := Inspect(context.Background(), Skill{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if detail.FileCount != 2 || detail.DirCount != 4 || detail.Size != 3 || len(detail.Files) != 6 {
		t.Fatalf("detail = %#v", detail)
	}
	if detail.Files[0].Path != ".git" || !detail.Files[0].Skipped || detail.Files[1].Path != "keep.txt" || detail.Files[2].Path != "node_modules" || !detail.Files[2].Skipped || detail.Files[3].Path != "visible" || detail.Files[4].Path != "visible/.DS_Store" || !detail.Files[4].Dir || detail.Files[5].Path != "visible/.DS_Store/kept" {
		t.Fatalf("files = %#v", detail.Files)
	}
}

func TestInspectLimits(t *testing.T) {
	t.Run("depth", func(t *testing.T) {
		restoreDetailLimits(t)
		detailDepthLimit = 1
		dir := filepath.Join(t.TempDir(), "skill")
		inspectWriteFile(t, filepath.Join(dir, "one", "two", "file"), "x")
		detail, err := Inspect(context.Background(), Skill{Dir: dir})
		if err != nil || !detail.Truncated || detail.DirCount != 1 || detail.FileCount != 0 || len(detail.Files) != 1 || detail.Files[0].Path != "one" {
			t.Fatalf("depth detail = %#v, %v", detail, err)
		}
	})
	t.Run("walk", func(t *testing.T) {
		restoreDetailLimits(t)
		detailWalkLimit = 2
		dir := filepath.Join(t.TempDir(), "skill")
		for _, name := range []string{"a", "b", "c"} {
			inspectWriteFile(t, filepath.Join(dir, name), name)
		}
		detail, err := Inspect(context.Background(), Skill{Dir: dir})
		if err != nil || !detail.Truncated || detail.FileCount != 2 || len(detail.Files) != 2 {
			t.Fatalf("walk detail = %#v, %v", detail, err)
		}
	})
	t.Run("walk ignores DSStore", func(t *testing.T) {
		restoreDetailLimits(t)
		detailWalkLimit = 1
		dir := filepath.Join(t.TempDir(), "skill")
		inspectWriteFile(t, filepath.Join(dir, ".DS_Store"), "ignored")
		inspectWriteFile(t, filepath.Join(dir, "kept"), "x")
		detail, err := Inspect(context.Background(), Skill{Dir: dir})
		if err != nil || detail.Truncated || detail.FileCount != 1 || len(detail.Files) != 1 || detail.Files[0].Path != "kept" {
			t.Fatalf("DS_Store walk detail = %#v, %v", detail, err)
		}
	})
	t.Run("files", func(t *testing.T) {
		restoreDetailLimits(t)
		detailFileLimit = 2
		dir := filepath.Join(t.TempDir(), "skill")
		for _, name := range []string{"c", "a", "b"} {
			inspectWriteFile(t, filepath.Join(dir, name), name)
		}
		detail, err := Inspect(context.Background(), Skill{Dir: dir})
		if err != nil || !detail.Truncated || detail.FileCount != 3 || len(detail.Files) != 2 || detail.Files[0].Path != "a" || detail.Files[1].Path != "b" {
			t.Fatalf("file detail = %#v, %v", detail, err)
		}
	})
}

func TestInspectContextAndDirectoryValidation(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Inspect(ctx, Skill{Dir: dir}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled error = %v", err)
	}
	file := filepath.Join(t.TempDir(), "skill")
	inspectWriteFile(t, file, "not a directory")
	if _, err := Inspect(context.Background(), Skill{Dir: file}); !errors.Is(err, errNotSkillDir) {
		t.Fatalf("file error = %v", err)
	}
}

func TestInspectResolvesSkillDirectoryLink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires privileges on Windows")
	}
	home := t.TempDir()
	real := filepath.Join(home, "real")
	inspectWriteFile(t, filepath.Join(real, "nested", "file"), "x")
	link := filepath.Join(home, "skill")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	detail, err := Inspect(context.Background(), Skill{Dir: link})
	if err != nil || detail.FileCount != 1 || detail.DirCount != 1 || detail.Files[1].Path != "nested/file" {
		t.Fatalf("linked root detail = %#v, %v", detail, err)
	}
}

func TestInspectReadErrors(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission semantics differ on Windows")
	}
	home := t.TempDir()
	dir := filepath.Join(home, "skill")
	blocked := filepath.Join(dir, "blocked")
	inspectWriteFile(t, filepath.Join(blocked, "secret"), "private")
	inspectWriteFile(t, filepath.Join(dir, "visible"), "ok")
	if err := os.Chmod(blocked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(blocked, 0o700) })
	if handle, err := os.Open(blocked); err == nil {
		handle.Close()
		t.Skip("current user can read mode 000 directories")
	}
	detail, err := Inspect(context.Background(), Skill{Dir: dir})
	if err != nil || detail.FileCount != 1 || detail.DirCount != 1 || len(detail.Files) != 2 {
		t.Fatalf("child read error detail = %#v, %v", detail, err)
	}
	if err := os.Chmod(dir, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	if _, err := Inspect(context.Background(), Skill{Dir: dir}); err == nil {
		t.Fatal("root read error was ignored")
	}
}

func restoreDetailLimits(t *testing.T) {
	t.Helper()
	files, walk, depth := detailFileLimit, detailWalkLimit, detailDepthLimit
	t.Cleanup(func() {
		detailFileLimit, detailWalkLimit, detailDepthLimit = files, walk, depth
	})
}

func inspectWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
