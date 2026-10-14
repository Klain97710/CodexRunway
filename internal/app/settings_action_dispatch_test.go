package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/l10n"
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
			tab, key := tabDisplay, l10n.KeyTestNotification
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
			if elapsed > 100*time.Millisecond {
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
	e := newEnv(t, nil)
	u, tt := openSettingsView(t, e, tabGeneral)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	var opened string
	u.opts.Actions.OpenFolder = func(path string) { opened = path }
	tt.Frame()
	focusSettingsAction(t, tt, e.s.L().Text(l10n.KeyCodexFolder))
	start := time.Now()
	tt.Key(0, ui.KeySpace)
	t.Logf("folder dispatch event=%s", time.Since(start))
	if opened != codexDir() {
		t.Fatal("folder dispatch lost the requested path")
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
