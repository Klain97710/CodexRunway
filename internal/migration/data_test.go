package migration

import . "github.com/licoy/quosparr/internal/platform"

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestBrandMigrationPreservesDataAndNeverOverwrites(t *testing.T) {
	home := t.TempDir()
	source := filepath.Join(home, ".codex-runway", "accounts", "sample", "auth.json")
	if err := WriteFileAtomic(source, []byte("synthetic credential fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	lock, err := AcquireLegacyLock(home, AcquireLock)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Release()
	if _, err := AcquireLegacyLock(home, AcquireLock); !errors.Is(err, ErrLocked) {
		t.Fatalf("old application was not excluded: %v", err)
	}
	if copied, err := Run(home); err != nil || !copied {
		t.Fatalf("migration: %v, %v", copied, err)
	}
	target := filepath.Join(RunwayDir(home), "accounts", "sample", "auth.json")
	for _, path := range []string{source, target} {
		data, err := os.ReadFile(path)
		if err != nil || string(data) != "synthetic credential fixture" {
			t.Fatal("migration lost source or destination data")
		}
		if err := VerifyOwnerOnly(path, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(source, []byte("changed old data"), 0o600); err != nil {
		t.Fatal(err)
	}
	if copied, err := Run(home); err != nil || copied {
		t.Fatalf("migration repeated: %v, %v", copied, err)
	}
	data, _ := os.ReadFile(target)
	if string(data) != "synthetic credential fixture" {
		t.Fatal("existing new data was overwritten")
	}
}

func TestBrandMigrationRejectsLinksWithoutPartialInstallation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires Windows developer mode")
	}
	home := t.TempDir()
	source := filepath.Join(home, ".codex-runway")
	if err := os.Mkdir(source, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(home, filepath.Join(source, "link")); err != nil {
		t.Fatal(err)
	}
	if copied, err := Run(home); err == nil || copied {
		t.Fatal("symlink was accepted")
	}
	if _, err := os.Stat(RunwayDir(home)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("partial destination was installed")
	}
}

func TestBrandMigrationContinuesIntoExistingDirectory(t *testing.T) {
	home := t.TempDir()
	old := filepath.Join(home, ".codex-runway")
	current := RunwayDir(home)
	for path, content := range map[string]string{
		filepath.Join(old, "accounts", "older", "auth.json"): "old fixture",
		filepath.Join(old, "preferences.json"):               "old settings",
		filepath.Join(current, "preferences.json"):           "new settings",
	} {
		if err := WriteFileAtomic(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Run(home); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(current, "accounts", "older", "auth.json")); err != nil || string(got) != "old fixture" {
		t.Fatal("existing destination prevented missing old account data from migrating")
	}
	if got, _ := os.ReadFile(filepath.Join(current, "preferences.json")); string(got) != "new settings" {
		t.Fatal("existing new settings were overwritten")
	}
}
