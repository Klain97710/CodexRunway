package runway

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/codexapi"
	"github.com/licoy/quosparr/internal/speed"
	"github.com/licoy/quosparr/internal/store"
	"github.com/licoy/quosparr/internal/usage"
)

// speedReport is a one-group report built the way the live source builds it.
func speedReport(now time.Time) speed.Report {
	return speed.Build([]speed.Session{{ID: "s1", Title: "t", UpdatedAt: now, Samples: []speed.Sample{
		{At: now.Add(-time.Hour), Model: "m", Effort: "high", TTFT: 2 * time.Second, HasTTFT: true, OutputTokens: 100, Decode: 2 * time.Second},
	}}}, now)
}

// fakeSpeed counts the scans and returns what the test sets.
type fakeSpeed struct {
	calls atomic.Int32
	err   atomic.Pointer[error]
}

func (f *fakeSpeed) scan(_ context.Context, now time.Time) (speed.Report, error) {
	f.calls.Add(1)
	if e := f.err.Load(); e != nil {
		return speed.Report{}, *e
	}
	return speedReport(now), nil
}

func TestFullRefreshLoadsSpeedWhenShown(t *testing.T) {
	f := newFake(t)
	var fs fakeSpeed
	s, _ := newService(t, f, func(o *Options) { o.Sources.ScanSpeed = fs.scan })
	showPanel(s)
	c := refreshAll(t, s).Codex
	if fs.calls.Load() != 1 {
		t.Fatalf("scans = %d, want 1", fs.calls.Load())
	}
	if c.Speed == nil || c.Speed.Empty() || c.Speed.Groups[0].Model != "m" || c.SpeedError != "" {
		t.Fatalf("speed = %+v, error %q", c.Speed, c.SpeedError)
	}
	if !c.Speed.CalculatedAt.Equal(t0) {
		t.Fatalf("calculated at %v, want the service clock %v", c.Speed.CalculatedAt, t0)
	}
}

func TestFullRefreshSkipsSpeedWhenHidden(t *testing.T) {
	f := newFake(t)
	var fs fakeSpeed
	s, _ := newService(t, f, func(o *Options) {
		o.Preferences.ShowsSpeedAnalysis = false
		o.Sources.ScanSpeed = fs.scan
	})
	c := refreshAll(t, s).Codex
	if fs.calls.Load() != 0 || c.Speed != nil {
		t.Fatalf("scans = %d, speed = %+v", fs.calls.Load(), c.Speed)
	}
}

func TestSpeedWithoutASourceIsNotAnError(t *testing.T) {
	f := newFake(t)
	s, _ := newService(t, f, nil)
	c := refreshAll(t, s).Codex
	if c.Speed != nil || c.SpeedError != "" {
		t.Fatalf("speed = %+v, error %q", c.Speed, c.SpeedError)
	}
	wait(t, s.RefreshSpeed(true))
}

func TestSpeedErrorIsShownAndClearedByTheNextScan(t *testing.T) {
	f := newFake(t)
	var fs fakeSpeed
	boom := errors.New("sessions unreadable")
	fs.err.Store(&boom)
	s, _ := newService(t, f, func(o *Options) { o.Sources.ScanSpeed = fs.scan })
	showPanel(s)
	c := refreshAll(t, s).Codex
	if c.Speed != nil || c.SpeedError != "sessions unreadable" || c.Quota == nil {
		t.Fatalf("speed = %+v, error %q, quota %v", c.Speed, c.SpeedError, c.Quota != nil)
	}
	fs.err.Store(nil)
	wait(t, s.RefreshSpeed(true))
	c = s.Snapshot().Codex
	if c.Speed == nil || c.SpeedError != "" {
		t.Fatalf("speed = %+v, error %q", c.Speed, c.SpeedError)
	}
}

func TestRefreshSpeedHonoursTheFreshnessWindow(t *testing.T) {
	f := newFake(t)
	var fs fakeSpeed
	s, _ := newService(t, f, func(o *Options) { o.Sources.ScanSpeed = fs.scan })
	refreshAll(t, s)
	wait(t, s.RefreshSpeed(false))
	if fs.calls.Load() != 1 {
		t.Fatalf("a fresh result was scanned again: %d", fs.calls.Load())
	}
	f.clk.Advance(visibleSectionTTL)
	wait(t, s.RefreshSpeed(false))
	if fs.calls.Load() != 2 {
		t.Fatalf("a stale result was not scanned again: %d", fs.calls.Load())
	}
	wait(t, s.RefreshSpeed(true))
	if fs.calls.Load() != 3 {
		t.Fatalf("a forced refresh was skipped: %d", fs.calls.Load())
	}
}

func TestRefreshSpeedShowsTheSectionAsBusy(t *testing.T) {
	f := newFake(t)
	g := newGate()
	s, _ := newService(t, f, func(o *Options) {
		o.Sources.ScanSpeed = func(context.Context, time.Time) (speed.Report, error) {
			g.wait()
			return speedReport(t0), nil
		}
	})
	done := s.RefreshSpeed(true)
	g.waitStarted(t)
	if !s.Snapshot().Refresh.Has(SectionSpeed) {
		t.Fatal("a running scan is not shown as refreshing")
	}
	g.open()
	wait(t, done)
	settle(t, s)
	snap := s.Snapshot()
	if snap.Refresh.Has(SectionSpeed) || snap.Codex.Speed == nil {
		t.Fatalf("after the scan: refreshing %v, speed %v", snap.Refresh.Has(SectionSpeed), snap.Codex.Speed != nil)
	}
}

func TestRefreshSpeedIsSkippedDuringAFullRefresh(t *testing.T) {
	f := newFake(t)
	var fs fakeSpeed
	gate := newGate()
	f.quotaFn = func(context.Context, codexapi.Auth) (codexapi.QuotaSnapshot, error) {
		gate.wait()
		return f.quota("plus", 10), nil
	}
	s, _ := newService(t, f, func(o *Options) { o.Sources.ScanSpeed = fs.scan })
	showPanel(s)
	full := s.Refresh(usage.RefreshForce)
	gate.waitStarted(t)
	wait(t, s.RefreshSpeed(true))
	gate.open()
	wait(t, full)
	settle(t, s)
	if fs.calls.Load() != 1 {
		t.Fatalf("scans = %d, want only the one of the full refresh", fs.calls.Load())
	}
}

func TestEnablingSpeedLoadsItForTheSelectedProvider(t *testing.T) {
	f := newFake(t)
	var fs fakeSpeed
	s, _ := newService(t, f, func(o *Options) {
		o.Preferences.ShowsSpeedAnalysis = false
		o.Sources.ScanSpeed = fs.scan
	})
	p := s.Preferences()
	p.ShowsSpeedAnalysis = true
	s.SetPreferences(p)
	settle(t, s)
	if fs.calls.Load() != 1 || s.Snapshot().Codex.Speed == nil {
		t.Fatalf("scans = %d, speed %v", fs.calls.Load(), s.Snapshot().Codex.Speed)
	}
	// Setting it on again changes nothing.
	s.SetPreferences(p)
	settle(t, s)
	if fs.calls.Load() != 1 {
		t.Fatalf("an unchanged preference scanned again: %d", fs.calls.Load())
	}
}

func TestEnablingSpeedWhileGrokIsSelectedRescansGrok(t *testing.T) {
	f := newFake(t)
	var fs fakeSpeed
	g := &fakeGrok{}
	s, _ := newService(t, f, func(o *Options) {
		o.Grok = g
		o.Preferences.SelectedProvider = store.ProviderGrok
		o.Preferences.ShowsSpeedAnalysis = false
		o.Sources.ScanSpeed = fs.scan
	})
	p := s.Preferences()
	p.ShowsSpeedAnalysis = true
	s.SetPreferences(p)
	settle(t, s)
	if g.localCount() != 1 || g.count() != 0 || fs.calls.Load() != 0 {
		t.Fatalf("grok local scans = %d, quota refreshes = %d, codex speed scans = %d", g.localCount(), g.count(), fs.calls.Load())
	}
}

func TestGrokSpeedPassesThroughTheSnapshot(t *testing.T) {
	f := newFake(t)
	r := speedReport(t0)
	g := &fakeGrok{state: GrokState{Speed: &r}}
	s, _ := newService(t, f, func(o *Options) { o.Grok = g; o.Preferences.SelectedProvider = store.ProviderGrok })
	if got := refreshAll(t, s).Grok.Speed; got == nil || got.Empty() {
		t.Fatalf("grok speed = %+v", got)
	}
}
