package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/store"
)

func TestSettingsSelfCheckClickReturnsBeforeDiagnostics(t *testing.T) {
	e := newEnv(t, nil)
	u, tt := openSettingsView(t, e, tabAdvanced)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	var completed atomic.Bool
	var calls atomic.Int32
	u.opts.Actions.SelfCheck = func() string {
		calls.Add(1)
		time.Sleep(200 * time.Millisecond)
		completed.Store(true)
		return "redacted local report"
	}
	tt.Frame()
	focusSettingsAction(t, tt, e.s.L().Text(l10n.KeySelfCheck))
	start := time.Now()
	tt.Key(0, ui.KeySpace)
	elapsed := time.Since(start)
	t.Logf("self-check event=%s completed=%v", elapsed, completed.Load())
	if elapsed > latencyBudget(100*time.Millisecond) || completed.Load() || !u.selfCheckTask.Running || u.noticeOpen {
		t.Fatal("self-check event waits for diagnostics")
	}
	tt.Key(0, ui.KeySpace)
	deadline := time.Now().Add(time.Second)
	for !completed.Load() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	awaitSettingsTask(t, e, &u.selfCheckTask)
	if !u.noticeOpen || u.notice != "redacted local report" || calls.Load() != 1 {
		t.Fatal("self-check completed without showing its report")
	}
}

func TestSettingsSelfCheckUsesRedactedTemporaryHomeReport(t *testing.T) {
	e := newEnv(t, nil)
	u, tt := openSettingsView(t, e, tabAdvanced)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	home := t.TempDir()
	if err := os.Mkdir(filepath.Join(home, ".codex-runway"), 0o700); err != nil {
		t.Fatal(err)
	}
	u.opts.Actions.SelfCheck = func() string {
		var out strings.Builder
		SelfCheck(&out, home, "test")
		return out.String()
	}
	tt.Frame()
	focusSettingsAction(t, tt, e.s.L().Text(l10n.KeySelfCheck))
	start := time.Now()
	tt.Key(0, ui.KeySpace)
	t.Logf("temporary-home actual self-check event=%s", time.Since(start))
	awaitSettingsTask(t, e, &u.selfCheckTask)
	if !u.noticeOpen || !strings.Contains(u.notice, "quosparr dir: ok ("+filepath.Join("~", ".codex-runway")) || strings.Contains(u.notice, home) {
		t.Fatal("temporary-home report was lost or unredacted")
	}
}

func TestSettingsProxyTestCredentialFailureIsVisible(t *testing.T) {
	e := newEnv(t, nil)
	var tested atomic.Bool
	e.s.SetProxyHooks(ProxyHooks{Test: func(context.Context, store.NetworkProxyConfiguration, *ProxyCredentials, string) error {
		tested.Store(true)
		return nil
	}})
	u, tt := openSettingsView(t, e, tabGeneral)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	u.draft = ProxyDraft{Mode: store.ProxyModeHTTP, Host: "localhost", Port: "8080", UsesAuth: true, CredentialID: newUUID()}
	tt.Frame()
	focusSettingsAction(t, tt, e.s.L().Text(l10n.KeyNetworkProxyTest))
	tt.Key(0, ui.KeySpace)
	awaitSettingsTask(t, e, &u.testTask)
	if tested.Load() || !u.proxyMsgErr || u.testTask.Err == nil || u.proxyMsg != e.s.L().Text(l10n.KeyProxyCredentialsUnavailable) {
		t.Fatal("credential read failure was hidden or network test still ran")
	}
}

func TestSettingsProxyTestUsesAndSavesTheTestSite(t *testing.T) {
	e := newEnv(t, nil)
	var site atomic.Value
	e.s.SetProxyHooks(ProxyHooks{Test: func(_ context.Context, _ store.NetworkProxyConfiguration, _ *ProxyCredentials, s string) error {
		site.Store(s)
		return nil
	}})
	u, tt := openSettingsView(t, e, tabGeneral)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	if u.testURL != store.DefaultProxyTestURL {
		t.Fatalf("test site starts as %q", u.testURL)
	}
	u.testURL = "ftp://example.com"
	u.testProxy()
	if u.testTask.Running || site.Load() != nil || !u.proxyMsgErr || u.proxyMsg != tx(e.s.L(), xProxyTestURLInvalid) {
		t.Fatal("an invalid test site was not rejected before the network test")
	}
	u.testURL = " example.com/ping "
	tt.Frame()
	focusSettingsAction(t, tt, e.s.L().Text(l10n.KeyNetworkProxyTest))
	tt.Key(0, ui.KeySpace)
	awaitSettingsTask(t, e, &u.testTask)
	if got := site.Load(); got != "https://example.com/ping" || u.proxyMsgErr {
		t.Fatalf("tested %v, message %q", got, u.proxyMsg)
	}
	if got := e.s.Prefs().ProxyTestURL; got != "https://example.com/ping" {
		t.Fatalf("saved test site = %q", got)
	}
}

type delayedCredentials struct {
	CredentialStore
	delay time.Duration
	loads atomic.Int32
}

func (s *delayedCredentials) Save(id string, c ProxyCredentials) error {
	time.Sleep(s.delay)
	return s.CredentialStore.Save(id, c)
}

func (s *delayedCredentials) Load(id string) (ProxyCredentials, error) {
	s.loads.Add(1)
	time.Sleep(s.delay)
	return s.CredentialStore.Load(id)
}

func TestSettingsProxyTestClickReturnsBeforeCredentialRead(t *testing.T) {
	e := newEnv(t, nil)
	id := newUUID()
	if err := e.creds.Save(id, ProxyCredentials{Username: "test", Password: "fixture"}); err != nil {
		t.Fatal(err)
	}
	// The credential delay stays above the event budget, so a blocking read
	// still fails the check under race instrumentation.
	creds := &delayedCredentials{CredentialStore: e.creds, delay: latencyBudget(200 * time.Millisecond)}
	e.s.creds = creds
	var tested atomic.Bool
	e.s.SetProxyHooks(ProxyHooks{Test: func(context.Context, store.NetworkProxyConfiguration, *ProxyCredentials, string) error {
		tested.Store(true)
		return nil
	}})
	u, tt := openSettingsView(t, e, tabGeneral)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	u.draft = ProxyDraft{Mode: store.ProxyModeHTTP, Host: "localhost", Port: "8080", UsesAuth: true, CredentialID: id}
	tt.Frame()
	focusSettingsAction(t, tt, e.s.L().Text(l10n.KeyNetworkProxyTest))
	start := time.Now()
	tt.Key(0, ui.KeySpace)
	elapsed := time.Since(start)
	t.Logf("proxy-test event=%s loads=%d tested=%v", elapsed, creds.loads.Load(), tested.Load())
	if elapsed > latencyBudget(100*time.Millisecond) || tested.Load() {
		t.Fatal("proxy-test event waits for saved credentials")
	}
	deadline := time.Now().Add(latencyBudget(time.Second))
	for !tested.Load() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !tested.Load() {
		t.Fatal("proxy test action did not run")
	}
	awaitSettingsTask(t, e, &u.testTask)
}

func awaitSettingsTask(t *testing.T, e *env, task *Task) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for task.Running && time.Now().Before(deadline) {
		e.q.drain()
		time.Sleep(time.Millisecond)
	}
	if task.Running {
		t.Fatal("settings task did not finish")
	}
}

func focusSettingsAction(t *testing.T, tt *ui.Tester, label string) {
	t.Helper()
	for range 60 {
		tt.Key(0, ui.KeyTab)
		if tt.Focused(label) {
			return
		}
	}
	t.Fatalf("action %q was not focusable", label)
}

func TestSettingsAutomaticUpdateClickReturnsBeforeUpdaterStateWrite(t *testing.T) {
	e := newEnv(t, nil)
	path := filepath.Join(t.TempDir(), "updater.json")
	var written atomic.Bool
	svc := &UpdateService{setAuto: func(bool) {
		time.Sleep(200 * time.Millisecond)
		if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
			t.Error(err)
		}
		written.Store(true)
	}}
	sys := e.sys.system()
	sys.SetAutomaticUpdateChecks = func(on bool) { svc.RequestAutomaticChecks(on, true) }
	e.s = NewSettings(e.st, sys, e.creds)
	_, tt := openSettingsView(t, e, tabAbout)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	focusSettingsAction(t, tt, e.s.L().Text(l10n.KeyAutomaticallyCheckForUpdates))
	start := time.Now()
	tt.Key(0, ui.KeySpace)
	elapsed := time.Since(start)
	t.Logf("automatic-update event=%s state written=%v", elapsed, written.Load())
	if elapsed > latencyBudget(100*time.Millisecond) || written.Load() {
		t.Fatal("automatic-update event waits for updater state write")
	}
	deadline := time.Now().Add(time.Second)
	for !written.Load() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !written.Load() {
		t.Fatal("updater state was not written")
	}
	e.settle()
	svc.FlushAutomaticChecks()
}

func TestSettingsProxySaveAndLoadEventsStayResponsive(t *testing.T) {
	for _, action := range []string{"save", "load"} {
		t.Run(action, func(t *testing.T) {
			e := newEnv(t, nil)
			id := newUUID()
			if err := e.creds.Save(id, ProxyCredentials{Username: "test", Password: "fixture"}); err != nil {
				t.Fatal(err)
			}
			e.s.creds = &delayedCredentials{CredentialStore: e.creds, delay: 200 * time.Millisecond}
			u, tt := openSettingsView(t, e, tabGeneral)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			u.draft = ProxyDraft{Mode: store.ProxyModeHTTP, Host: "localhost", Port: "8080", UsesAuth: true, CredentialID: id}
			key := l10n.KeyNetworkProxyLoadCredentials
			if action == "save" {
				u.draft.Username, u.draft.Password = "test", "fixture"
				key = l10n.KeyNetworkProxySave
			}
			tt.Frame()
			focusSettingsAction(t, tt, e.s.L().Text(key))
			start := time.Now()
			tt.Key(0, ui.KeySpace)
			elapsed := time.Since(start)
			t.Logf("proxy %s event=%s", action, elapsed)
			if elapsed > latencyBudget(100*time.Millisecond) || !u.proxyTask.Running {
				t.Fatal("proxy save/load event waits for credential store")
			}
			awaitSettingsTask(t, e, &u.proxyTask)
			if u.proxyMsgErr || (action == "load" && u.draft.Username != "test") || (action == "save" && e.s.Proxy().Host != "localhost") {
				t.Fatal("proxy save/load did not preserve its result")
			}
		})
	}
}

func TestSettingsRepairConfirmationDoesNotRunUntilAccepted(t *testing.T) {
	e := newEnv(t, nil)
	u, tt := openSettingsView(t, e, tabAdvanced)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	var repaired int
	u.opts.Actions.RepairIndex = func() { repaired++ }
	tt.Frame()
	label := e.s.L().Text(l10n.KeyRepair)
	focusSettingsAction(t, tt, label)
	start := time.Now()
	tt.Key(0, ui.KeySpace)
	t.Logf("repair confirmation event=%s", time.Since(start))
	if !u.confirmRepair || repaired != 0 {
		t.Fatal("repair ran before confirmation")
	}
	focusSettingsAction(t, tt, e.s.L().Text(l10n.KeyCancel))
	tt.Key(0, ui.KeySpace)
	if u.confirmRepair || repaired != 0 {
		t.Fatal("cancelling repair still ran it")
	}
	focusSettingsAction(t, tt, label)
	tt.Key(0, ui.KeySpace)
	focusSettingsAction(t, tt, label)
	tt.Key(0, ui.KeySpace)
	if u.confirmRepair || repaired != 1 {
		t.Fatal("accepted repair did not run once")
	}
}
