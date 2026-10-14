package platform

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrLocked reports that another process (a Quosparr of any
// implementation) holds the single-instance lock.
var ErrLocked = errors.New("quosparr: another instance holds the lock")

// Lock is the held single-instance lock. The lock file is never deleted:
// the Swift app uses the same file with flock, and removing it while
// another process holds it would let a third process lock a new inode.
type Lock struct {
	f *os.File
}

// AcquireLock takes an exclusive, non-blocking lock on path, creating its
// directory (0700) and the file (0600) when needed. It returns ErrLocked
// when the lock is held elsewhere.
func AcquireLock(path string) (*Lock, error) {
	f, err := openLockFile(path)
	if err != nil {
		return nil, err
	}
	if err := lockFile(f); err != nil {
		_ = f.Close()
		return nil, err
	}
	return &Lock{f: f}, nil
}

// Release drops the lock and closes the file. It is safe to call twice.
func (l *Lock) Release() error {
	if l == nil || l.f == nil {
		return nil
	}
	f := l.f
	l.f = nil
	uerr := unlockFile(f)
	cerr := f.Close()
	if uerr != nil {
		return uerr
	}
	return cerr
}

// LockState is what ProbeLock saw.
type LockState int

const (
	// LockMissing: the lock file does not exist yet.
	LockMissing LockState = iota
	// LockFree: no process holds the lock.
	LockFree
	// LockHeld: another process holds the lock.
	LockHeld
)

// ProbeLock reports whether the lock is held without creating anything:
// a missing file is LockMissing. A free lock is taken and released at once.
func ProbeLock(path string) (LockState, error) {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if errors.Is(err, os.ErrNotExist) {
		return LockMissing, nil
	}
	if err != nil {
		return LockMissing, err
	}
	defer f.Close()
	if err := lockFile(f); err != nil {
		if errors.Is(err, ErrLocked) {
			return LockHeld, nil
		}
		return LockMissing, err
	}
	_ = unlockFile(f)
	return LockFree, nil
}

func openLockFile(path string) (*os.File, error) {
	if err := EnsurePrivateDirectory(filepath.Dir(path)); err != nil {
		return nil, fmt.Errorf("create lock directory: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}
	if err := ProtectOwnerOnly(path, 0o600); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("protect lock file: %w", err)
	}
	return f, nil
}
