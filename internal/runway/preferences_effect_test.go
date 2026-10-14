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

	"github.com/licoy/quosparr/internal/codexapi"
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

func TestPreferencesProviderChangeRefreshesSelectedProvider(t *testing.T) {
	f := newFake(t)
	g := &fakeGrok{}
	s, _ := newService(t, f, func(o *Options) { o.Grok = g })
	p := s.Preferences()
	p.SelectedProvider = store.ProviderGrok
	s.SetPreferences(p)
	settle(t, s)
	if g.count() != 1 {
		t.Fatalf("selected provider refreshes = %d", g.count())
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

func TestProviderChangeCancelsOldRefreshThenLoadsNewProvider(t *testing.T) {
	f := newFake(t)
	gate := newGate()
	g := &fakeGrok{}
	f.quotaFn = func(ctx context.Context, _ codexapi.Auth) (codexapi.QuotaSnapshot, error) {
		gate.waitCtx(ctx)
		return f.quota("late", 99), nil
	}
	s, _ := newService(t, f, func(o *Options) { o.Grok = g })
	done := s.Refresh(usage.RefreshForce)
	gate.waitStarted(t)
	p := s.Preferences()
	p.SelectedProvider = store.ProviderGrok
	s.SetPreferences(p)
	wait(t, done)
	settle(t, s)
	if g.count() != 1 {
		t.Fatalf("new provider not loaded: %d", g.count())
	}
	if q := s.Snapshot().Codex.Quota; q != nil && q.Plan == "late" {
		t.Fatal("cancelled provider result applied")
	}
}
