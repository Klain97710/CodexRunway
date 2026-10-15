//go:build unix

package app

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/codex"
	"github.com/licoy/quosparr/internal/codexapi"
	"github.com/licoy/quosparr/internal/grok"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/personal"
	"github.com/licoy/quosparr/internal/runway"
	"github.com/licoy/quosparr/internal/store"
	"github.com/licoy/quosparr/internal/usage"
)

// The FIFO stands in for a stalled real index read; this drives the same
// Settings.Update -> OnChange -> pushPreferences path as panel/settings UI.
func TestLivePreferencesDoNotReadGrokIndexOnUIThread(t *testing.T) {
	e := newEnv(t, nil)
	p := e.s.Prefs()
	p.ShowsQuotaEstimateSummary = false
	e.s.prefs = p
	st := grok.NewStoreAt(t.TempDir(), t.TempDir())
	if err := st.SaveIndex(grok.NewAccountIndex()); err != nil {
		t.Fatal(err)
	}
	g := newGrokBridge(grok.NewModule(st), p, nil)
	g.reload(false)
	svc := runway.New(runway.Options{Features: personal.Features{Grok: true, Claude: true}, Preferences: p, Grok: g})
	defer svc.Close()
	rt := &liveApp{svc: svc, grok: g, reset: newResetBridge(e.s, p, nil)}
	e.s.OnChange(func() { rt.pushPreferences(e.s) })
	if err := os.Remove(st.IndexPath()); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(st.IndexPath(), 0600); err != nil {
		t.Fatal(err)
	}
	pipe, err := os.OpenFile(st.IndexPath(), os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer pipe.Close()
	done := make(chan struct{})
	started := time.Now()
	go func() { e.s.SetAccountPrivacyMode(true); close(done) }()
	select {
	case <-done:
		t.Logf("privacy callback returned in %s", time.Since(started))
	case <-time.After(100 * time.Millisecond):
		t.Error("privacy callback waited for Grok account index disk read")
	}
	if _, err := pipe.Write([]byte(`{"version":1,"accounts":[]}`)); err != nil {
		t.Fatal(err)
	}
	pipe.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("callback did not complete after disk read released")
	}
	e.settle()
	rt.closePreferences()
	if !svc.Preferences().AccountPrivacyModeEnabled {
		t.Fatal("latest preferences not applied")
	}
}

func TestGrokReloadUsesLatestLanguage(t *testing.T) {
	p := store.DefaultPreferences()
	p.Language = store.LanguageEnglish
	st := grok.NewStoreAt(t.TempDir(), t.TempDir())
	g := newGrokBridge(grok.NewModule(st), p, nil)
	g.reload(false)
	if err := syscall.Mkfifo(st.IndexPath(), 0600); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { g.reload(false); close(done) }()
	// Opening a write-only FIFO returns when the real Store.LoadIndex reader
	// is in flight; no sleeps or fake provider callbacks are needed.
	pipe, err := os.OpenFile(st.IndexPath(), os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	p.Language = store.LanguageSimplifiedChinese
	g.setPreferences(p, nil)
	if _, err := pipe.Write([]byte(`{"version":1,"accounts":[]}`)); err != nil {
		t.Fatal(err)
	}
	pipe.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("reload did not finish")
	}
	want := l10n.ForPreference(string(p.Language), nil).Text(l10n.KeyGrokNotLoggedIn)
	if got := g.State(time.Now()).LastError; got != want {
		t.Fatalf("old language returned after reload: %q, want %q", got, want)
	}
}

func TestLiveUIActionsReturnDuringBackgroundHistoryRead(t *testing.T) {
	for _, action := range []string{"preferences", "refresh", "full-refresh", "settings-refresh", "settings-repair"} {
		t.Run(action, func(t *testing.T) {
			e := newEnv(t, nil)
			p := quietLivePreferences()
			p.ShowsQuotaEstimateSummary = true
			e.s.prefs = p
			path := filepath.Join(t.TempDir(), "history.json")
			if err := syscall.Mkfifo(path, 0600); err != nil {
				t.Fatal(err)
			}
			auth := codex.APIKeyAuth("synthetic-local-test")
			g := newGrokBridge(nil, p, nil)
			svc := runway.New(runway.Options{Features: personal.Features{Grok: true, Claude: true}, Preferences: p, Grok: g, EstimateHistoryPath: path, Sources: runway.Sources{
				LoadValidAuth: func(context.Context, bool, *codex.Auth) (codex.Auth, error) { return auth, nil },
				FetchQuota: func(context.Context, codexapi.Auth) (codexapi.QuotaSnapshot, error) {
					return codexapi.QuotaSnapshot{}, nil
				},
				FetchDailyUsage: func(context.Context, codexapi.Auth, string, string, usage.Window, time.Time) (usage.Summary, error) {
					return usage.Summary{}, nil
				},
				FetchResetCredits: func(context.Context, codexapi.Auth) (codexapi.ResetCreditsSnapshot, error) {
					return codexapi.ResetCreditsSnapshot{}, nil
				},
				FetchProfileTokens: func(context.Context, codexapi.Auth) (usage.ProfileTokenUsage, error) {
					return usage.ProfileTokenUsage{}, nil
				},
				ScanCost: func(context.Context, []usage.Query, time.Time, usage.RefreshPolicy, *usage.ProgressReporter) (map[string]usage.Summary, error) {
					return nil, nil
				},
			}})
			defer svc.Close()
			rt := &liveApp{svc: svc, grok: g, reset: newResetBridge(e.s, p, nil)}
			e.s.OnChange(func() { rt.pushPreferences(e.s) })
			previous := currentRuntime()
			setLive(rt)
			defer setLive(previous)
			background := svc.RefreshQuotaEstimate()
			pipe, err := os.OpenFile(path, os.O_WRONLY, 0600)
			if err != nil {
				t.Fatal(err)
			}
			done := make(chan struct{})
			started := time.Now()
			go func() {
				switch action {
				case "preferences":
					e.s.SetAccountPrivacyMode(true)
				case "refresh":
					rt.panelActions(e.s).RefreshSection(runway.SectionQuota)
				case "full-refresh":
					rt.panelActions(e.s).Refresh()
				case "settings-refresh":
					RequestLiveRefresh()
				case "settings-repair":
					RequestLiveSessionRepair()
				}
				close(done)
			}()
			select {
			case <-done:
				t.Logf("%s callback returned in %s", action, time.Since(started))
			case <-time.After(100 * time.Millisecond):
				t.Errorf("%s callback waited for background history disk read holding service lock", action)
			}
			payload := []byte(`{"accounts":{}}`)
			if _, err := pipe.Write(payload); err != nil {
				t.Fatal(err)
			}
			// The first reader already owns the FIFO inode; future refreshes
			// must read a regular file, including during race-enabled runs.
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, payload, 0600); err != nil {
				t.Fatal(err)
			}
			pipe.Close()
			select {
			case <-background:
			case <-time.After(time.Second):
				t.Fatal("background history read did not finish")
			}
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("UI callback did not finish after disk release")
			}
			e.settle()
			rt.closePreferences()
		})
	}
}
