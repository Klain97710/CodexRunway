package app

import (
	"errors"
	"os"
	"strconv"
	"time"

	"github.com/licoy/quosparr/internal/platform"
)

// RelaunchEnv marks the processes that Quosparr itself started. mygo's
// Relaunch (used after an update) starts the new executable while the old
// process is still finishing its quit, and the new one inherits the
// environment of the old one, so the old one sets this at start-up: the new
// process then knows it may have to wait for the single-instance lock.
// Nothing else sets it, and it is removed on first read so that it never
// reaches processes started by the new one.
const RelaunchEnv = "QUOSPARR_STARTED_BY"

// Lock waiting after a relaunch.
const (
	relaunchLockWait = 20 * time.Second
	relaunchLockPoll = 100 * time.Millisecond
)

// MarkRelaunchable tags this process's environment, for the process mygo
// starts when the app relaunches itself.
func MarkRelaunchable() { _ = os.Setenv(RelaunchEnv, strconv.Itoa(os.Getpid())) }

// TakeRelaunchMark reports whether a Quosparr process started this one,
// and clears the mark.
func TakeRelaunchMark() bool {
	v := os.Getenv(RelaunchEnv)
	_ = os.Unsetenv(RelaunchEnv)
	return v != "" && v != strconv.Itoa(os.Getpid())
}

// AcquireInstanceLock takes the single-instance lock. A process started by
// a relaunch waits (up to relaunchLockWait) for the old process to release
// it; it never removes the lock file, so Swift and Go instances stay
// mutually exclusive. Any other process fails at once with
// platform.ErrLocked when the lock is held.
func AcquireInstanceLock(path string, relaunched bool) (*platform.Lock, error) {
	return acquireLockWait(path, relaunched, relaunchLockWait, relaunchLockPoll, time.Sleep)
}

func acquireLockWait(path string, wait bool, limit, poll time.Duration, sleep func(time.Duration)) (*platform.Lock, error) {
	var waited time.Duration
	for {
		lock, err := platform.AcquireLock(path)
		if err == nil || !wait || !errors.Is(err, platform.ErrLocked) || waited >= limit {
			return lock, err
		}
		sleep(poll)
		waited += poll
	}
}
