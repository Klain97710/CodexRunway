package app

import (
	"sync/atomic"

	"github.com/egoist/mygo"
)

// runOnMain hops to the main thread. Tests, which have no main loop,
// replace the hop with a queue they drain between frames (setRunOnMain).
func runOnMain(fn func()) {
	if hop := mainHop.Load(); hop != nil {
		(*hop)(fn)
		return
	}
	mygo.RunOnMain(fn)
}

// mainDispatcher captures the current UI target before background work starts;
// an old settings save must not dispatch through a later test's replacement.
func mainDispatcher() func(func()) {
	if hop := mainHop.Load(); hop != nil {
		return *hop
	}
	return mygo.RunOnMain
}

// mainHop replaces mygo.RunOnMain when set. It is atomic: goroutines of a
// finished test may still hop while the next test swaps it.
var mainHop atomic.Pointer[func(func())]

// setRunOnMain makes runOnMain call hop, and returns what restores the
// previous one.
func setRunOnMain(hop func(func())) (restore func()) {
	old := mainHop.Swap(&hop)
	return func() { mainHop.Store(old) }
}

// Task tracks one asynchronous operation started from a view: whether it
// runs, and how it ended. Keep tasks in view state so a result cannot land
// on the wrong page.
type Task struct {
	Running bool
	Err     error
}

// runTask starts fn on a goroutine unless the task is already running, then
// calls done with its outcome on the main thread through post (which should
// draw a new frame; nil uses runOnMain).
func runTask(t *Task, post func(func()), fn func() error, done func(error)) {
	if post == nil {
		post = runOnMain
	}
	if t.Running {
		return
	}
	t.Running, t.Err = true, nil
	go func() {
		err := fn()
		post(func() {
			t.Running, t.Err = false, err
			if done != nil {
				done(err)
			}
		})
	}()
}
