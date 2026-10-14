package runway

import (
	"testing"
	"time"
)

func at(sec int64) time.Time { return time.Unix(sec, 0) }

func TestWidgetCadenceIsIndependent(t *testing.T) {
	initial := at(1000)
	main, widget := NewSchedule(), NewSchedule()
	main.RefreshCompleted(initial, 300*time.Second)
	widget.RefreshCompleted(initial, 600*time.Second)

	done := initial.Add(310 * time.Second)
	if !main.IsDue(done) {
		t.Fatal("main refresh should be due")
	}
	main.RefreshStarted()
	main.RefreshCompleted(done, 300*time.Second)

	if widget.IsDue(initial.Add(599 * time.Second)) {
		t.Fatal("widget due too early")
	}
	if !widget.IsDue(initial.Add(600 * time.Second)) {
		t.Fatal("widget should be due at 600s")
	}
}

func TestAutomaticRefreshIsScheduledFromCompletion(t *testing.T) {
	started := at(1000)
	completed := started.Add(600 * time.Second)
	s := NewSchedule()
	s.IntervalChanged(300*time.Second, started.Add(-300*time.Second))
	if !s.IsDue(started) {
		t.Fatal("due after interval change")
	}
	s.RefreshStarted()
	if s.IsDue(started.Add(300*time.Second)) || s.IsDue(completed) {
		t.Fatal("never due while in flight")
	}
	s.RefreshCompleted(completed, 300*time.Second)
	if s.IsDue(completed.Add(299 * time.Second)) {
		t.Fatal("due too early")
	}
	if !s.IsDue(completed.Add(300 * time.Second)) {
		t.Fatal("due at completion+interval")
	}
}

func TestIntervalChangeStaysAnchoredToCompletion(t *testing.T) {
	completed := at(2000)
	s := NewSchedule()
	s.RefreshStarted()
	s.RefreshCompleted(completed, 300*time.Second)
	s.IntervalChanged(600*time.Second, completed.Add(100*time.Second))
	if s.IsDue(completed.Add(599 * time.Second)) {
		t.Fatal("due too early")
	}
	if !s.IsDue(completed.Add(600 * time.Second)) {
		t.Fatal("due at 600s")
	}
}

func TestIntervalChangeCannotArmWhileRunning(t *testing.T) {
	s := NewSchedule()
	s.RefreshStarted()
	s.IntervalChanged(time.Second, at(3000))
	if s.IsDue(farFuture) {
		t.Fatal("must not be due while in flight")
	}
}
