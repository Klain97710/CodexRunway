// Package migration performs the one-time, non-destructive brand migration.
package migration

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/licoy/quosparr/internal/platform"
)

const marker = ".brand-migration-v2"
const signature = "quosparr-data-migration-v2\n"

// Run requires the caller to hold platform.AcquireLegacyLock until shutdown.
// All app versions sharing this directory take that stable lock before their
// current-directory lock. Prepare a complete private copy, then switch trees;
// the journal lets the next launch recover either interrupted rename.
// Keep preparation, commit and rollback in this linear function so every
// possible return can be checked against the same transaction boundaries.
func Run(home string) (bool, error) {
	target := platform.RunwayDir(home)
	work := filepath.Join(home, ".quosparr-migration-v2")
	if _, err := directoryExists(target); err != nil {
		return false, err
	}
	if state, err := platform.ProbeLock(platform.LockPath(home)); err != nil {
		return false, err
	} else if state == platform.LockHeld {
		return false, platform.ErrLocked
	}
	if err := recoverWork(work, target); err != nil {
		return false, err
	}
	if done, err := completed(target); err != nil || done {
		return false, err
	}
	source := filepath.Join(home, ".codex-runway")
	oldExists, err := directoryExists(source)
	if err != nil || !oldExists {
		return false, err
	}
	hadTarget, err := directoryExists(target)
	if err != nil {
		return false, err
	}
	if err := platform.EnsurePrivateDirectory(work); err != nil {
		return false, err
	}
	if err := platform.WriteFileAtomic(filepath.Join(work, "owner"), []byte(signature), 0o600); err != nil {
		return false, err
	}
	stage := filepath.Join(work, "new")
	if err := prepare(source, target, stage, hadTarget); err != nil {
		return false, errors.Join(err, os.RemoveAll(work))
	}
	if hadTarget {
		if err := os.Rename(target, filepath.Join(work, "previous")); err != nil {
			return false, err
		}
	}
	if err := os.Rename(stage, target); err != nil {
		return false, errors.Join(fmt.Errorf("install migrated data: %w", err), recoverWork(work, target))
	}
	return true, os.RemoveAll(work)
}

func prepare(source, target, stage string, hadTarget bool) error {
	if err := platform.EnsurePrivateDirectory(stage); err != nil {
		return err
	}
	if hadTarget {
		if err := copyTree(target, stage, false); err != nil {
			return err
		}
	}
	if err := copyTree(source, stage, true); err != nil {
		return err
	}
	return platform.WriteFileAtomic(filepath.Join(stage, marker), []byte(signature), 0o600)
}

func directoryExists(path string) (bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.IsDir() {
		return false, fmt.Errorf("migration path is not a directory: %s", filepath.Base(path))
	}
	return true, nil
}

func completed(target string) (bool, error) {
	if _, err := directoryExists(target); err != nil {
		return false, err
	}
	path := filepath.Join(target, marker)
	if err := regularFile(path); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	if string(data) != signature {
		return false, errors.New("invalid migration completion record")
	}
	return true, nil
}

// Recovery only removes directories carrying this version's ownership record.
// An unrecognised or ambiguous state is retained for manual inspection.
func recoverWork(work, target string) error {
	exists, err := directoryExists(work)
	if err != nil || !exists {
		return err
	}
	entries, err := os.ReadDir(work)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return os.Remove(work)
	}
	if err := regularFile(filepath.Join(work, "owner")); err != nil {
		return err
	}
	owner, err := os.ReadFile(filepath.Join(work, "owner"))
	if err != nil {
		return err
	}
	if string(owner) != signature {
		return errors.New("unrecognised migration staging directory")
	}
	for _, entry := range entries {
		if entry.Name() != "owner" && entry.Name() != "new" && entry.Name() != "previous" {
			return errors.New("unexpected migration staging contents")
		}
	}
	backup := filepath.Join(work, "previous")
	hasBackup, err := directoryExists(backup)
	if err != nil {
		return err
	}
	hasTarget, err := directoryExists(target)
	if err != nil {
		return err
	}
	if hasBackup && !hasTarget {
		if err := os.Rename(backup, target); err != nil {
			return err
		}
	} else if hasBackup {
		done, err := completed(target)
		if err != nil {
			return err
		}
		if !done {
			return errors.New("ambiguous interrupted migration; both data directories retained")
		}
	}
	return os.RemoveAll(work)
}
