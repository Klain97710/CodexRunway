package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/codex"
	"github.com/licoy/quosparr/internal/grok"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/runway"
)

type delayedSettingsNotification struct{ done chan struct{} }

func (delayedSettingsNotification) Supported() bool { return true }

func (n delayedSettingsNotification) Show(Notification) error {
	time.Sleep(200 * time.Millisecond)
	close(n.done)
	return nil
}

func TestSettingsNotificationAndManualUpdateEventsStayResponsive(t *testing.T) {
	for _, action := range []string{"notification", "update"} {
		t.Run(action, func(t *testing.T) {
			e := newEnv(t, nil)
			tab, key := tabNotifications, l10n.KeyTestNotification
			if action == "update" {
				tab, key = tabAbout, l10n.KeyCheckForUpdates
			}
			u, tt := openSettingsView(t, e, tab)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			done := make(chan struct{})
			if action == "notification" {
				n := &Notifier{backend: delayedSettingsNotification{done}}
				u.opts.Actions.TestNotification = n.TestAction(e.s.L, nil)
			} else {
				svc := &UpdateService{
					enabled:  func() bool { time.Sleep(200 * time.Millisecond); return true },
					checkUI:  func() { close(done) },
					runAsync: func(fn func()) { go fn() },
				}
				u.opts.Actions.CheckForUpdates = func() { svc.CheckNow(e.s.L()) }
			}
			tt.Frame()
			focusSettingsAction(t, tt, e.s.L().Text(key))
			start := time.Now()
			tt.Key(0, ui.KeySpace)
			elapsed := time.Since(start)
			t.Logf("%s event=%s", action, elapsed)
			select {
			case <-done:
				t.Fatal("event waits for system notification or updater eligibility check")
			default:
			}
			if elapsed > latencyBudget(100*time.Millisecond) {
				t.Fatal("event waits for system notification or updater eligibility check")
			}
			if action == "notification" && (!u.noticeOpen || u.notice != tx(e.s.L(), xNotifyRequested)) {
				t.Fatal("notification request was not acknowledged")
			}
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("action did not complete")
			}
		})
	}
}

func TestSettingsFolderAndExternalLinksDispatch(t *testing.T) {
	codexHome, grokHome := t.TempDir(), t.TempDir()
	svc := runway.New(runway.Options{})
	defer svc.Close()
	rt := &liveApp{
		codex:   codex.NewStore(codexHome, t.TempDir()),
		grokMod: grok.NewModule(grok.NewStore(grokHome, t.TempDir())),
		svc:     svc,
	}
	previous := currentRuntime()
	setLive(rt)
	defer setLive(previous)
	e := newEnv(t, nil)
	u, tt := openSettingsView(t, e, tabAccounts)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	var opened string
	u.opts.Actions.OpenFolder = func(path string) { opened = path }
	tt.Frame()
	for _, folder := range []struct {
		kind int
		key  l10n.Key
		path string
	}{{accountKindCodex, l10n.KeyCodexFolder, codexHome}, {accountKindGrok, l10n.KeyGrokFolder, grokHome}} {
		accountFormOf(u).kind = folder.kind
		tt.Frame()
		focusSettingsAction(t, tt, e.s.L().Text(folder.key))
		start := time.Now()
		tt.Key(0, ui.KeySpace)
		t.Logf("folder dispatch event=%s", time.Since(start))
		if opened != folder.path {
			t.Fatalf("folder dispatch opened %q, want %q", opened, folder.path)
		}
	}
	u.tab = tabGeneral
	tt.Frame()
	if tt.HasText(e.s.L().Text(l10n.KeyCodexFolder)) {
		t.Fatal("general page still opens the Codex folder")
	}
	u.tab = tabAbout
	tt.Frame()
	for _, key := range []l10n.Key{l10n.KeyOpenGithub, l10n.KeyFeedbackIssue} {
		focusSettingsAction(t, tt, e.s.L().Text(key))
		start := time.Now()
		tt.Key(0, ui.KeySpace)
		t.Logf("link dispatch event=%s", time.Since(start))
	}
	urls := tt.OpenedURLs()
	if len(urls) != 2 || urls[0] != githubURL || urls[1] != feedbackURL {
		t.Fatal("external links were not dispatched correctly")
	}
}
