package app

import (
	"context"
	"fmt"
)

// Seams are the startup steps that other packages plug into. A nil step is
// skipped. They run in the order of plan section 3.5, after the single
// instance lock is held:
//
//	LoadConfig -> MigrateLegacy -> VerifyProxy -> ReconcileSystem -> StartBackground
//
// Settings.Load reads and validates preferences through internal/store,
// including the legacy UserDefaults import when needed. Settings.LoadProxy
// verifies proxy credentials. App.Ready reconciles the login item with the
// real system state without re-enabling what the user turned off.
type Seams struct {
	LoadConfig      func() error
	MigrateLegacy   func() error
	VerifyProxy     func() error
	ReconcileSystem func() error
	// StartBackground starts refresh schedulers and returns a stop func.
	// It is not called when config or proxy are unusable.
	StartBackground func(ctx context.Context) (stop func())
}

// Problem is a startup failure the panel can show. A broken config or
// proxy leaves the app running in a diagnosable state instead of exiting.
type Problem struct {
	Step string
	Err  error
}

func (p Problem) String() string { return fmt.Sprintf("%s: %v", p.Step, p.Err) }

// StartupReport is what RunStartup found.
type StartupReport struct {
	Problems []Problem
	// BackgroundAllowed is false when config or proxy failed: network and
	// write tasks must not run on top of state we could not read.
	BackgroundAllowed bool
}

// RunSteps runs the steps before background tasks, in order. A failing
// step is recorded and the later steps still run, except that a failed
// config load skips the migration, which writes.
func (s Seams) RunSteps() StartupReport {
	r := StartupReport{BackgroundAllowed: true}
	run := func(name string, fn func() error, blocksBackground bool) bool {
		if fn == nil {
			return true
		}
		if err := fn(); err != nil {
			r.Problems = append(r.Problems, Problem{Step: name, Err: err})
			if blocksBackground {
				r.BackgroundAllowed = false
			}
			return false
		}
		return true
	}
	if run("config", s.LoadConfig, true) {
		run("legacy migration", s.MigrateLegacy, true)
	}
	run("proxy credentials", s.VerifyProxy, true)
	run("system state", s.ReconcileSystem, false)
	return r
}
