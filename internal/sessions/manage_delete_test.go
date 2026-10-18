package sessions

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDeleteManagedMovesScannedFile(t *testing.T) {
	root := t.TempDir()
	path := writeManagedTestFile(t, root, "one.jsonl", "session")
	session := managedTestSession(t, ProviderCodex, root, path, path)
	trashDir := t.TempDir()
	results, err := DeleteManaged(context.Background(), []ManagedSession{session}, managedTestRoots(ProviderCodex, root), renameTrash(trashDir))
	if err != nil || len(results) != 1 || results[0].Err != nil || results[0].Removed != 1 || results[0].Total != 1 {
		t.Fatalf("DeleteManaged() = %#v, %v", results, err)
	}
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("source remains: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(trashDir, filepath.Base(path))); err != nil {
		t.Fatalf("trash item: %v", err)
	}
}

func TestDeleteManagedRejectsUnsafeOrChangedFiles(t *testing.T) {
	t.Run("traversal", func(t *testing.T) {
		root, outside := t.TempDir(), t.TempDir()
		path := writeManagedTestFile(t, outside, "outside.jsonl", "session")
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		session := ManagedSession{Item: Item{ID: "one"}, Provider: ProviderCodex, Root: root, Path: path, Files: []ManagedFile{{Path: path, Size: info.Size(), ModTime: info.ModTime(), Info: info}}}
		assertManagedDeleteRejected(t, root, session, "outside")
	})
	t.Run("symbolic link", func(t *testing.T) {
		root := t.TempDir()
		target := writeManagedTestFile(t, root, "target.jsonl", "session")
		link := filepath.Join(root, "link.jsonl")
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(link)
		if err != nil {
			t.Fatal(err)
		}
		session := ManagedSession{Item: Item{ID: "one"}, Provider: ProviderCodex, Root: root, Path: link, Files: []ManagedFile{{Path: link, Size: info.Size(), ModTime: info.ModTime(), Info: info}}}
		assertManagedDeleteRejected(t, root, session, "symbolic link")
	})
	t.Run("symbolic parent", func(t *testing.T) {
		root := t.TempDir()
		realDir := filepath.Join(root, "real")
		if err := os.Mkdir(realDir, 0o700); err != nil {
			t.Fatal(err)
		}
		target := writeManagedTestFile(t, realDir, "one.jsonl", "session")
		linkedDir := filepath.Join(root, "linked")
		if err := os.Symlink(realDir, linkedDir); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(linkedDir, "one.jsonl")
		info, err := os.Lstat(target)
		if err != nil {
			t.Fatal(err)
		}
		session := ManagedSession{Item: Item{ID: "one"}, Provider: ProviderCodex, Root: root, Path: path, Files: []ManagedFile{{Path: path, Size: info.Size(), ModTime: info.ModTime(), Info: info}}}
		assertManagedDeleteRejected(t, root, session, "parent")
	})
	t.Run("root path", func(t *testing.T) {
		root := t.TempDir()
		path := writeManagedTestFile(t, root, "one.jsonl", "session")
		session := managedTestSession(t, ProviderCodex, root, root, path)
		assertManagedDeleteRejected(t, root, session, "outside")
	})
	t.Run("identity replacement", func(t *testing.T) {
		root := t.TempDir()
		path := writeManagedTestFile(t, root, "one.jsonl", "session")
		session := managedTestSession(t, ProviderCodex, root, path, path)
		oldTime := session.Files[0].ModTime
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		writeManagedTestFile(t, root, "one.jsonl", "session")
		if err := os.Chtimes(path, oldTime, oldTime); err != nil {
			t.Fatal(err)
		}
		assertManagedDeleteRejected(t, root, session, "identity changed")
	})
	t.Run("modification time", func(t *testing.T) {
		root := t.TempDir()
		path := writeManagedTestFile(t, root, "one.jsonl", "session")
		session := managedTestSession(t, ProviderCodex, root, path, path)
		changed := session.Files[0].ModTime.Add(time.Second)
		if err := os.Chtimes(path, changed, changed); err != nil {
			t.Fatal(err)
		}
		assertManagedDeleteRejected(t, root, session, "changed after scanning")
	})
	t.Run("size", func(t *testing.T) {
		root := t.TempDir()
		path := writeManagedTestFile(t, root, "one.jsonl", "session")
		session := managedTestSession(t, ProviderCodex, root, path, path)
		if err := os.WriteFile(path, []byte("session grew"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, session.Files[0].ModTime, session.Files[0].ModTime); err != nil {
			t.Fatal(err)
		}
		assertManagedDeleteRejected(t, root, session, "changed after scanning")
	})
}

func TestDeleteManagedReportsPartialTrashFailure(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "session")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	summary := writeManagedTestFile(t, dir, "summary.json", "summary")
	events := writeManagedTestFile(t, dir, "events.jsonl", "events")
	updates := writeManagedTestFile(t, dir, "updates.jsonl", "updates")
	session := managedTestSession(t, ProviderGrok, root, dir, summary, events, updates)
	originalOrder := []string{session.Files[0].Path, session.Files[1].Path, session.Files[2].Path}
	trashDir := t.TempDir()
	trash := func(path string) error {
		if path == updates {
			return errors.New("trash failed")
		}
		return renameTrash(trashDir)(path)
	}
	results, err := DeleteManaged(context.Background(), []ManagedSession{session}, managedTestRoots(ProviderGrok, root), trash)
	if err == nil || len(results) != 1 || results[0].Err == nil || results[0].Removed != 1 || results[0].Total != 3 || !strings.Contains(results[0].Err.Error(), "moved 1 of 3") {
		t.Fatalf("DeleteManaged() = %#v, %v", results, err)
	}
	if _, statErr := os.Lstat(events); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("events file remains: %v", statErr)
	}
	if _, statErr := os.Lstat(updates); statErr != nil {
		t.Fatalf("failed updates file missing: %v", statErr)
	}
	if _, statErr := os.Lstat(summary); statErr != nil {
		t.Fatalf("summary must remain for a retry: %v", statErr)
	}
	for i, path := range originalOrder {
		if session.Files[i].Path != path {
			t.Fatalf("input Files order changed: %#v", session.Files)
		}
	}
}

func TestDeleteManagedRejectsOverlappingBatch(t *testing.T) {
	root := t.TempDir()
	path := writeManagedTestFile(t, root, "one.jsonl", "session")
	first := managedTestSession(t, ProviderCodex, root, path, path)
	second := first
	second.ID = "two"
	called := false
	results, err := DeleteManaged(context.Background(), []ManagedSession{first, second}, managedTestRoots(ProviderCodex, root), func(string) error {
		called = true
		return nil
	})
	if err == nil || called || len(results) != 2 || results[0].Err == nil || results[1].Err == nil {
		t.Fatalf("DeleteManaged() = %#v, %v, called=%t", results, err, called)
	}
}

func TestDeleteManagedCancelledBeforeTrash(t *testing.T) {
	root := t.TempDir()
	path := writeManagedTestFile(t, root, "one.jsonl", "session")
	session := managedTestSession(t, ProviderCodex, root, path, path)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	results, err := DeleteManaged(ctx, []ManagedSession{session}, managedTestRoots(ProviderCodex, root), func(string) error {
		called = true
		return nil
	})
	if !errors.Is(err, context.Canceled) || !errors.Is(results[0].Err, context.Canceled) || called {
		t.Fatalf("DeleteManaged() = %#v, %v, called=%t", results, err, called)
	}
}

func assertManagedDeleteRejected(t *testing.T, trustedRoot string, session ManagedSession, want string) {
	t.Helper()
	called := false
	results, err := DeleteManaged(context.Background(), []ManagedSession{session}, managedTestRoots(session.Provider, trustedRoot), func(string) error {
		called = true
		return nil
	})
	if err == nil || len(results) != 1 || results[0].Err == nil || !strings.Contains(results[0].Err.Error(), want) || called {
		t.Fatalf("DeleteManaged() = %#v, %v, called=%t", results, err, called)
	}
}

func managedTestSession(t *testing.T, provider, root, path string, files ...string) ManagedSession {
	t.Helper()
	session := ManagedSession{Item: Item{ID: "one"}, Provider: provider, Root: root, Path: path}
	for _, file := range files {
		snapshot, err := SnapshotManagedFile(root, file)
		if err != nil {
			t.Fatal(err)
		}
		session.Files = append(session.Files, snapshot)
	}
	return session
}

func writeManagedTestFile(t *testing.T, root, name, contents string) string {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func renameTrash(dir string) func(string) error {
	return func(path string) error { return os.Rename(path, filepath.Join(dir, filepath.Base(path))) }
}

func managedTestRoots(provider, root string) map[string][]string {
	return map[string][]string{provider: {root}}
}
