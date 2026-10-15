package app

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/store"
)

// Exercise the actual general-page view and proxy pill event, with a slow
// OS query. Rendering and local draft edits must never query the login item.
func TestSettingsGeneralRenderAndProxyClickDoNotQueryLogin(t *testing.T) {
	e := newEnv(t, nil)
	var statusCalls, switchCalls atomic.Int32
	sys := e.sys.system()
	sys.LoginStatus = func() (LoginStatus, error) {
		statusCalls.Add(1)
		time.Sleep(80 * time.Millisecond)
		return LoginNotRegistered, nil
	}
	sys.OpenAtLogin = func() (bool, error) {
		switchCalls.Add(1)
		time.Sleep(80 * time.Millisecond)
		return false, nil
	}
	e.s = NewSettings(e.st, sys, e.creds)
	u, tt := openSettingsView(t, e, tabGeneral)
	statusCalls.Store(0)
	switchCalls.Store(0)
	start := time.Now()
	tt.Frame()
	frame := time.Since(start)
	frameCalls := statusCalls.Load() + switchCalls.Load()
	start = time.Now()
	if err := tt.Click(e.s.L().Text(l10n.KeyNetworkProxyHTTP)); err != nil {
		t.Fatal(err)
	}
	click := time.Since(start)
	t.Logf("frame=%s calls=%d; proxy click=%s total calls=%d; writes=%d", frame, frameCalls, click, statusCalls.Load()+switchCalls.Load(), e.mem.Writes())
	if u.draft.Mode != store.ProxyModeHTTP {
		t.Fatal("proxy pill did not change the actual draft")
	}
	if frameCalls != 0 || statusCalls.Load()+switchCalls.Load() != 0 || frame > latencyBudget(50*time.Millisecond) || click > latencyBudget(100*time.Millisecond) {
		t.Fatal("general-page rendering blocks local interaction on login-item I/O")
	}
}

func TestSettingsGeneralLatencyControl(t *testing.T) {
	e := newEnv(t, nil)
	var calls int
	sys := e.sys.system()
	sys.LoginStatus = func() (LoginStatus, error) { calls++; return LoginNotRegistered, nil }
	sys.OpenAtLogin = func() (bool, error) { calls++; return false, nil }
	e.s = NewSettings(e.st, sys, e.creds)
	_, tt := openSettingsView(t, e, tabGeneral)
	calls = 0
	start := time.Now()
	tt.Frame()
	frame := time.Since(start)
	start = time.Now()
	if err := tt.Click(e.s.L().Text(l10n.KeyNetworkProxyHTTP)); err != nil {
		t.Fatal(err)
	}
	t.Logf("zero-delay control: frame=%s click=%s calls=%d writes=%d", frame, time.Since(start), calls, e.mem.Writes())
}

func TestSettingsLoginClickRespondsBeforeSystemCompletes(t *testing.T) {
	e := newEnv(t, nil)
	sys := e.sys.system()
	set := sys.SetOpenAtLogin
	var complete atomic.Bool
	sys.SetOpenAtLogin = func(on bool) error {
		time.Sleep(200 * time.Millisecond)
		err := set(on)
		complete.Store(true)
		return err
	}
	sys.LoginAsync = true
	e.s = NewSettings(e.st, sys, e.creds)
	_, tt := openSettingsView(t, e, tabGeneral)
	// Headless settling otherwise runs 20 spinner animation frames per key
	// event, amplifying race-instrumentation cost unrelated to input latency.
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	label := e.s.L().Text(l10n.KeyLaunchAtLogin)
	for range 20 {
		tt.Key(0, ui.KeyTab)
		if tt.Focused(label) {
			break
		}
	}
	if !tt.Focused(label) {
		t.Fatal("login switch was not focusable")
	}
	start := time.Now()
	tt.Key(0, ui.KeySpace)
	elapsed := time.Since(start)
	t.Logf("login click=%s", elapsed)
	if !e.s.LoginPending() || complete.Load() || e.mem.Writes() != 0 {
		t.Error("input was acknowledged after completion or pending was recorded as success")
	}
	e.settle()
	if elapsed > latencyBudget(100*time.Millisecond) {
		t.Fatal("login toggle waits for backend completion")
	}
}
