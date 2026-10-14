package migration

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/licoy/quosparr/internal/platform"
)

func TestPersonalMigrationIsInPlaceWithHeldSwiftLock(t *testing.T) {
	home := t.TempDir()
	paths := []string{filepath.Join(home, ".codex", "auth.json"), filepath.Join(home, ".codex-runway", "accounts", "fixture", "auth.json")}
	fixture := []byte(`{"fixture":"synthetic-only"}`)
	for _, p := range paths {
		if err := platform.WriteFileAtomic(p, fixture, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	lock, err := platform.AcquireLock(filepath.Join(home, ".codex-runway", "codex-runway.lock"))
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Release()
	if moved, err := Run(home); err != nil || moved {
		t.Fatalf("in-place migration: moved=%v err=%v", moved, err)
	}
	if _, err := platform.AcquireLock(platform.LockPath(home)); err != platform.ErrLocked {
		t.Fatal("Swift and Go locks do not exclude each other")
	}
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil || !bytes.Equal(data, fixture) {
			t.Fatal("installation rewrote auth")
		}
	}
	if _, err := os.Stat(filepath.Join(home, ".quosparr")); !os.IsNotExist(err) {
		t.Fatal("created renamed data tree")
	}
}
