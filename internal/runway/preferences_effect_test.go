package runway

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/export"
	"github.com/licoy/quosparr/internal/sessions"
	"github.com/licoy/quosparr/internal/store"
	"github.com/licoy/quosparr/internal/usage"
)

func TestEnablingSessionModulesLoadsTheirData(t *testing.T) {
	f := newFake(t)
	var reports, recent atomic.Int32
	s, _ := newService(t, f, func(o *Options) {
		o.Preferences.ShowsSessionRepairSummary = false
		o.Preferences.ShowsRecentSessions = false
		o.Sources.DryRunSessions = func(context.Context) (sessions.Report, error) { reports.Add(1); return sessions.Report{}, nil }
		o.Sources.ScanRecentSessions = func(context.Context, int) (sessions.Summary, error) { recent.Add(1); return sessions.Summary{}, nil }
	})
	p := s.Preferences()
	p.ShowsSessionRepairSummary, p.ShowsRecentSessions = true, true
	s.SetPreferences(p)
	settle(t, s)
	if reports.Load() != 1 || recent.Load() != 1 {
		t.Fatalf("enabled modules did not load: report=%d recent=%d", reports.Load(), recent.Load())
	}
}

func TestProviderChangeKeepsTheRefreshRhythm(t *testing.T) {
	f := newFake(t)
	g := &fakeGrok{}
	s, _ := newService(t, f, func(o *Options) { o.Grok = g })
	refreshAll(t, s)
	quota, grok := f.count("quota"), g.count()
	for _, provider := range []store.Provider{store.ProviderGrok, store.ProviderClaude, store.ProviderCodex} {
		p := s.Preferences()
		p.SelectedProvider = provider
		s.SetPreferences(p)
		settle(t, s)
	}
	if f.count("quota") != quota || g.count() != grok {
		t.Fatalf("switching fetched quota: codex %d→%d, grok %d→%d", quota, f.count("quota"), grok, g.count())
	}
	s.Tick(f.clk.Advance(time.Duration(s.Preferences().RefreshIntervalSeconds) * time.Second))
	settle(t, s)
	if f.count("quota") != quota+1 || g.count() != grok+1 {
		t.Fatal("the schedule did not refresh every provider")
	}
}

func TestGrokAutomaticRefreshRearmsAfterCompletion(t *testing.T) {
	f := newFake(t)
	g := &fakeGrok{}
	s, _ := newService(t, f, func(o *Options) { o.Grok = g; o.Preferences.SelectedProvider = store.ProviderGrok })
	wait(t, s.Refresh(usage.RefreshForce))
	settle(t, s)
	s.Tick(f.clk.Advance(time.Second))
	settle(t, s)
	if g.count() != 1 {
		t.Fatalf("refetched before interval: %d", g.count())
	}
	s.Tick(f.clk.Advance(300 * time.Second))
	settle(t, s)
	if g.count() != 2 {
		t.Fatalf("due refreshes = %d", g.count())
	}
	s.Tick(f.clk.Advance(time.Second))
	settle(t, s)
	if g.count() != 2 {
		t.Fatalf("deadline stayed due after completion: %d", g.count())
	}
}

func TestEnablingExportWritesImmediatelyAndReportsFailure(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "write", true: "error"}[fail], func(t *testing.T) {
			f := newFake(t)
			path := filepath.Join(t.TempDir(), "status.json")
			if fail {
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			s, _ := newService(t, f, func(o *Options) { o.StatusPath = path })
			p := s.Preferences()
			p.ExportsStatusJSON = true
			s.SetPreferences(p)
			settle(t, s)
			if fail {
				if s.Snapshot().Codex.LastError == "" {
					t.Fatal("export failure was swallowed")
				}
				return
			}
			if _, err := os.ReadFile(path); err != nil {
				t.Fatalf("export not written: %v", err)
			}
		})
	}
}

func TestWidgetSinkFailureIsVisible(t *testing.T) {
	f := newFake(t)
	s, _ := newService(t, f, func(o *Options) {
		o.WidgetRequirements = export.RequireProviderQuota
		o.WidgetSink = func(export.Snapshot) error { return errors.New("widget disk full") }
	})
	s.SetWidgetRequirements(export.RequireProviderQuota | export.RequireCost)
	settle(t, s)
	if !strings.Contains(s.Snapshot().Codex.LastError, "widget disk full") {
		t.Fatalf("widget failure missing: %s", s.Snapshot().Codex.LastError)
	}
}

// A provider the panel hides and nothing else shows is not fetched; showing
// it again fetches its quota at once.
func TestHiddenPanelProvidersSkipQuotaRefresh(t *testing.T) {
	f := newFake(t)
	g, c := &fakeGrok{}, &fakeClaude{}
	s, _ := newService(t, f, func(o *Options) {
		o.Grok, o.Claude = g, c
		o.Preferences.PanelProviders = []store.Provider{store.ProviderCodex}
	})
	wait(t, s.Refresh(usage.RefreshForce))
	settle(t, s)
	wait(t, s.RefreshQuota())
	settle(t, s)
	if g.count() != 0 || len(c.calls()) != 0 {
		t.Fatalf("hidden providers refreshed: grok %d, claude %d", g.count(), len(c.calls()))
	}
	if f.count("quota") != 2 {
		t.Fatalf("Codex quota refreshed %d times, want 2", f.count("quota"))
	}

	p := s.Preferences()
	p.StatusBarProviderScope = store.ProviderScopeCustom
	p.StatusBarProviders = []store.Provider{store.ProviderCodex, store.ProviderClaude}
	s.SetPreferences(p)
	settle(t, s)
	if g.count() != 0 || len(c.calls()) != 1 {
		t.Fatalf("status bar pick: grok %d, claude %d, want 0 and 1", g.count(), len(c.calls()))
	}

	p = s.Preferences()
	p.PanelProviders = nil
	s.SetPreferences(p)
	settle(t, s)
	if g.count() != 1 || len(c.calls()) != 1 {
		t.Fatalf("showing every provider: grok %d, claude %d, want 1 and 1", g.count(), len(c.calls()))
	}
}
