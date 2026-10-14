package app

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/platform"
)

func TestRelaunchMark(t *testing.T) {
	t.Setenv(RelaunchEnv, "")
	if TakeRelaunchMark() {
		t.Error("no mark yet")
	}
	t.Setenv(RelaunchEnv, "424242")
	if !TakeRelaunchMark() {
		t.Error("the mark of another process was not seen")
	}
	if os.Getenv(RelaunchEnv) != "" || TakeRelaunchMark() {
		t.Error("the mark must be cleared on first read")
	}
	MarkRelaunchable()
	if os.Getenv(RelaunchEnv) != strconv.Itoa(os.Getpid()) {
		t.Error("MarkRelaunchable did not set the mark")
	}
	if TakeRelaunchMark() {
		t.Error("a process's own mark is not a relaunch")
	}
}

func TestRelaunchedProcessWaitsForTheOldOneToReleaseTheLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "quosparr.lock")
	old, err := platform.AcquireLock(path)
	if err != nil {
		t.Fatal(err)
	}
	sleeps := 0
	sleep := func(time.Duration) {
		sleeps++
		if sleeps == 3 {
			_ = old.Release() // the old process finishes quitting
		}
	}
	lock, err := acquireLockWait(path, true, time.Second, 10*time.Millisecond, sleep)
	if err != nil || lock == nil {
		t.Fatalf("lock %v err %v after %d waits", lock, err, sleeps)
	}
	defer lock.Release()
	if sleeps != 3 {
		t.Errorf("waited %d times", sleeps)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("the lock file must never be removed: %v", err)
	}
}

func TestOrdinaryStartDoesNotWait(t *testing.T) {
	path := filepath.Join(t.TempDir(), "quosparr.lock")
	held, err := platform.AcquireLock(path)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Release()
	_, err = acquireLockWait(path, false, time.Second, time.Millisecond, func(time.Duration) { t.Error("slept") })
	if !errors.Is(err, platform.ErrLocked) {
		t.Errorf("err = %v", err)
	}
}

func TestRelaunchWaitIsBounded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "quosparr.lock")
	held, err := platform.AcquireLock(path)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Release()
	n := 0
	_, err = acquireLockWait(path, true, 50*time.Millisecond, 10*time.Millisecond, func(time.Duration) { n++ })
	if !errors.Is(err, platform.ErrLocked) || n != 5 {
		t.Errorf("err %v after %d waits", err, n)
	}
}
