package app

import (
	"github.com/licoy/quosparr/internal/store"
	"testing"
	"time"
)

func TestSettingsFlushDoesNotWaitForSynchronousUIDispatcher(t *testing.T) {
	for _, fail := range []bool{false, true} {
		name := "success"
		if fail {
			name = "rollback"
		}
		t.Run(name, func(t *testing.T) {
			queued := make(chan func(), 4)
			release := make(chan struct{})
			dispatchDone := make(chan struct{}, 4)
			restore := setRunOnMain(func(fn func()) { queued <- fn; <-release; dispatchDone <- struct{}{} })
			defer restore()
			mem := &failDomain{MemoryDomain: store.NewMemoryDomain()}
			s := NewSettings(store.New(mem, nil), System{}, nil)
			if err := s.Load(); err != nil {
				t.Fatal(err)
			}
			s.Update(func(p *store.Preferences) { p.ShowsTokenUsageHeatmap = false })
			var callback func()
			select {
			case callback = <-queued:
			case <-time.After(3 * time.Second):
				t.Fatal("save did not reach synchronous dispatcher")
			}
			mem.fail.Store(fail)
			s.Update(func(p *store.Preferences) { p.ShowsQuotaSummary = false })
			// The UI thread is inside Flush, while the save's synchronous dispatcher
			// cannot complete until this thread executes callback; old drain deadlocks.
			timedOut := make(chan struct{})
			watchdog := time.AfterFunc(2*time.Second, func() { close(timedOut); close(release) })
			s.Flush()
			if !watchdog.Stop() {
				<-timedOut
				t.Fatal("Flush waited for the main dispatcher")
			}
			if s.Prefs().ShowsTokenUsageHeatmap {
				t.Fatal("first saved result was lost")
			}
			if fail && (!s.Prefs().ShowsQuotaSummary || s.SaveError() == nil) {
				t.Fatal("FIFO persisted state was not restored after failure")
			}
			if !fail && (s.Prefs().ShowsQuotaSummary || s.SaveError() != nil) {
				t.Fatal("second disk save did not complete")
			}
			callback()
			close(release)
			<-dispatchDone
		})
	}
}
