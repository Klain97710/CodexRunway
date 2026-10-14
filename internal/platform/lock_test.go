package platform

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func lockPath(t *testing.T) string {
	t.Helper()
	return LockPath(t.TempDir())
}

func TestSecondAcquireFailsWhileHeld(t *testing.T) {
	path := lockPath(t)
	first, err := AcquireLock(path)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Release()

	// A separate open file description in the same process must conflict,
	// as another process would.
	second, err := AcquireLock(path)
	if !errors.Is(err, ErrLocked) {
		if second != nil {
			second.Release()
		}
		t.Fatalf("second acquire: got %v, want ErrLocked", err)
	}
}

func TestReleaseAllowsReacquireAndKeepsFile(t *testing.T) {
	path := lockPath(t)
	first, err := AcquireLock(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Release(); err != nil {
		t.Fatal(err)
	}
	if err := first.Release(); err != nil {
		t.Fatalf("second release: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("lock file must not be deleted: %v", err)
	}
	again, err := AcquireLock(path)
	if err != nil {
		t.Fatalf("reacquire: %v", err)
	}
	again.Release()
}

func TestAcquireCreatesPrivateDirectoryAndFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX modes")
	}
	path := lockPath(t)
	l, err := AcquireLock(path)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Release()
	di, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if got := di.Mode().Perm(); got != 0o700 {
		t.Errorf("dir mode %o, want 700", got)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := fi.Mode().Perm(); got != 0o600 {
		t.Errorf("file mode %o, want 600", got)
	}
}

func TestProbeLock(t *testing.T) {
	path := lockPath(t)
	if st, err := ProbeLock(path); err != nil || st != LockMissing {
		t.Fatalf("missing: %v %v", st, err)
	}
	if _, err := os.Stat(filepath.Dir(path)); err == nil {
		t.Fatal("probe must not create the directory")
	}
	l, err := AcquireLock(path)
	if err != nil {
		t.Fatal(err)
	}
	if st, err := ProbeLock(path); err != nil || st != LockHeld {
		t.Fatalf("held: %v %v", st, err)
	}
	l.Release()
	if st, err := ProbeLock(path); err != nil || st != LockFree {
		t.Fatalf("free: %v %v", st, err)
	}
	// The probe must not leave the lock held.
	again, err := AcquireLock(path)
	if err != nil {
		t.Fatalf("acquire after probe: %v", err)
	}
	again.Release()
}
