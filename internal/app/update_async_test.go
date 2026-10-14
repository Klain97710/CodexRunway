package app

import (
	"slices"
	"sync"
	"testing"
	"time"
)

func TestRequestedAutomaticChecksPreserveLatestValueAndOrder(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	var calls []bool
	svc := &UpdateService{setAuto: func(on bool) {
		calls = append(calls, on)
		if len(calls) == 1 {
			close(started)
			<-release
		}
	}}
	svc.RequestAutomaticChecks(false, true)
	<-started
	svc.RequestAutomaticChecks(true, true)
	svc.RequestAutomaticChecks(false, true)
	svc.RequestAutomaticChecks(true, true)
	close(release)
	svc.FlushAutomaticChecks()
	if !slices.Equal(calls, []bool{false, true}) {
		t.Fatalf("rapid switch values=%v", calls)
	}
	// An unusable proxy must still turn checks off, then reapply the user's
	// true value once it is usable again.
	svc.RequestAutomaticChecks(true, false)
	svc.FlushAutomaticChecks()
	svc.RequestAutomaticChecks(true, true)
	svc.FlushAutomaticChecks()
	if !slices.Equal(calls, []bool{false, true, false, true}) {
		t.Fatalf("proxy reapply values=%v", calls)
	}
}

func TestAutomaticChecksRollbackFlushDoesNotWaitForUIDelivery(t *testing.T) {
	e := newEnv(t, nil)
	if err := e.s.Load(); err != nil {
		t.Fatal(err)
	}
	var calls []bool
	queued, release := make(chan struct{}, 4), make(chan struct{})
	var deliveries sync.WaitGroup
	svc := &UpdateService{setAuto: func(on bool) {
		calls = append(calls, on)
		// MyGo updater.update notifies subscribers from its own goroutine,
		// so state-file writes must finish even when UI delivery is blocked.
		deliveries.Add(1)
		go func() { defer deliveries.Done(); queued <- struct{}{}; <-release }()
	}}
	e.s.sys.SetAutomaticUpdateChecks = func(on bool) { svc.RequestAutomaticChecks(on, true) }
	e.mem.fail.Store(true)
	e.s.SetAutomaticUpdateChecks(false)
	<-queued
	// App.shutdown flushes settings first, allowing a failed save to submit
	// the rollback before OnQuit flushes the updater worker.
	e.s.Flush()
	done := make(chan struct{})
	go func() { svc.FlushAutomaticChecks(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		close(release)
		t.Fatal("updater flush waited for main-thread delivery")
	}
	close(release)
	deliveries.Wait()
	if !slices.Equal(calls, []bool{false, true}) || !e.s.Prefs().AutomaticallyChecksForUpdates || e.s.SaveError() == nil {
		t.Fatalf("save rollback lost: updater values=%v prefs=%v error=%v", calls, e.s.Prefs().AutomaticallyChecksForUpdates, e.s.SaveError())
	}
	e.q.drain()
}
