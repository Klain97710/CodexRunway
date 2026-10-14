package app

import (
	"context"
	"errors"
	"testing"

	"github.com/licoy/quosparr/internal/store"
)

func TestProxyRepairStartsBackgroundOnceAndClearsStartupProblem(t *testing.T) {
	e := newEnv(t, nil)
	_ = e.s.Load()
	e.s.proxyErr = errors.New("invalid proxy")
	starts, stops := 0, 0
	a := New(Options{Settings: e.s, Report: StartupReport{Problems: []Problem{{Step: "proxy credentials", Err: e.s.proxyErr}}}, Seams: Seams{StartBackground: func(context.Context) func() { starts++; return func() { stops++ } }}})
	a.backgroundReady = true
	a.startBackground()
	if starts != 0 {
		t.Fatal("blocked background started")
	}
	e.s.ProxySaved(ProxySaveResult{Config: store.DefaultNetworkProxy()})
	e.s.SetAccountPrivacyMode(true)
	if starts != 1 || len(a.opts.Report.Problems) != 0 || !a.opts.Report.BackgroundAllowed {
		t.Fatalf("repair did not recover once: starts=%d report=%+v", starts, a.opts.Report)
	}
	e.s.proxyErr = errors.New("proxy blocked")
	e.s.changed()
	if stops != 1 {
		t.Fatalf("blocked background did not stop: %d", stops)
	}
}
