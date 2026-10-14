package runway

import (
	"context"
	"github.com/licoy/quosparr/internal/personal"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/codex"
	"github.com/licoy/quosparr/internal/codexapi"
	"github.com/licoy/quosparr/internal/export"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/store"
	"github.com/licoy/quosparr/internal/usage"
	"github.com/licoy/quosparr/internal/usage/decimal"
)

var t0 = time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)

// clock is the fake time of a Service under test.
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func newClock() *clock { return &clock{t: t0} }

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) Advance(d time.Duration) time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
	return c.t
}

// gate holds a fake call until the test releases it. The call ignores its
// context on purpose: it stands for a result that arrives late.
type gate struct {
	off     atomic.Bool // a gate that is off lets calls through
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func newGate() *gate { return &gate{started: make(chan struct{}), release: make(chan struct{})} }

func (g *gate) wait() {
	if g.off.Load() {
		return
	}
	g.once.Do(func() { close(g.started) })
	<-g.release
}

// waitCtx is wait for calls that notice cancellation: it returns on release
// or when ctx ends, and the caller still reports success.
func (g *gate) waitCtx(ctx context.Context) {
	if g.off.Load() {
		return
	}
	g.once.Do(func() { close(g.started) })
	select {
	case <-g.release:
	case <-ctx.Done():
	}
}

func (g *gate) waitStarted(t testing.TB) {
	t.Helper()
	select {
	case <-g.started:
	case <-time.After(5 * time.Second):
		t.Fatal("blocked call never started")
	}
}

func (g *gate) open() { close(g.release) }

// fake implements every Sources function without touching disk or network.
// Tests replace the *Fn fields before creating the Service.
type fake struct {
	clk *clock

	mu       sync.Mutex
	auth     codex.Auth
	authErr  error
	calls    map[string]int
	recorded []string

	quotaFn   func(ctx context.Context, a codexapi.Auth) (codexapi.QuotaSnapshot, error)
	creditsFn func(ctx context.Context, a codexapi.Auth) (codexapi.ResetCreditsSnapshot, error)
	dailyFn   func(ctx context.Context, a codexapi.Auth) (usage.Summary, error)
	profileFn func(ctx context.Context, a codexapi.Auth) (usage.ProfileTokenUsage, error)
	scanFn    func(ctx context.Context, q []usage.Query) (map[string]usage.Summary, error)
}

func newFake(t testing.TB) *fake {
	f := &fake{clk: newClock(), calls: map[string]int{}}
	f.auth = testAuth(t, "acct-a", "a@example.com", "refresh-a")
	f.quotaFn = func(_ context.Context, a codexapi.Auth) (codexapi.QuotaSnapshot, error) {
		return f.quota("plan-"+a.AccountID, 20), nil
	}
	f.creditsFn = func(context.Context, codexapi.Auth) (codexapi.ResetCreditsSnapshot, error) {
		now := f.clk.Now()
		return codexapi.ResetCreditsSnapshot{
			AvailableCount: 1, UpdatedAt: now,
			Credits: []codexapi.ResetCredit{{ID: "c1", Status: "available", ExpiresAt: now.Add(30 * 24 * time.Hour), RemainingSeconds: 30 * 86400}},
		}, nil
	}
	f.dailyFn = func(context.Context, codexapi.Auth) (usage.Summary, error) {
		return usage.Summary{Source: usage.SourceOnlineAnalytics, Confidence: usage.ConfidencePriced,
			Totals: usage.Totals{TotalTokens: 1000, Turns: 2}, CalculatedAt: f.clk.Now()}, nil
	}
	f.profileFn = func(context.Context, codexapi.Auth) (usage.ProfileTokenUsage, error) {
		day := f.clk.Now().UTC().Format("2006-01-02")
		return usage.ProfileTokenUsage{DailyTokens: map[string]int64{day: 500}, StatsAsOf: day}, nil
	}
	f.scanFn = func(_ context.Context, qs []usage.Query) (map[string]usage.Summary, error) {
		usd := decimal.FromInt(2)
		out := map[string]usage.Summary{}
		for _, q := range qs {
			out[q.ID] = usage.Summary{
				Source: usage.SourceLocalSessions, Confidence: usage.ConfidencePriced, Window: q.Window,
				EstimatedUSD: &usd, Totals: usage.Totals{TotalTokens: 1000, Turns: 3}, CalculatedAt: f.clk.Now(),
			}
		}
		return out, nil
	}
	return f
}

// quota is a weekly-cycle reading used at the fake clock's time.
func (f *fake) quota(plan string, usedPercent int) codexapi.QuotaSnapshot {
	now := f.clk.Now()
	return codexapi.QuotaSnapshot{
		Plan:      plan,
		Primary:   codexapi.RateWindow{UsedPercent: usedPercent, UsedPercentExact: float64(usedPercent), WindowMinutes: FiveHourMinutes, ResetsAt: now.Add(2 * time.Hour)},
		Secondary: &codexapi.RateWindow{UsedPercent: 10, UsedPercentExact: 10, WindowMinutes: WeeklyMinutes, ResetsAt: now.Add(3 * 24 * time.Hour)},
		UpdatedAt: now,
	}
}

func (f *fake) count(name string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[name]
}

func (f *fake) hit(name string) {
	f.mu.Lock()
	f.calls[name]++
	f.mu.Unlock()
}

func (f *fake) setAuth(a codex.Auth, err error) {
	f.mu.Lock()
	f.auth, f.authErr = a, err
	f.mu.Unlock()
}

func (f *fake) recordedIDs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.recorded...)
}

func (f *fake) sources() Sources {
	return Sources{
		LoadValidAuth: func(context.Context, bool, *codex.Auth) (codex.Auth, error) {
			f.hit("auth")
			f.mu.Lock()
			defer f.mu.Unlock()
			return f.auth, f.authErr
		},
		FetchQuota: func(ctx context.Context, a codexapi.Auth) (codexapi.QuotaSnapshot, error) {
			f.hit("quota")
			return f.quotaFn(ctx, a)
		},
		FetchResetCredits: func(ctx context.Context, a codexapi.Auth) (codexapi.ResetCreditsSnapshot, error) {
			f.hit("credits")
			return f.creditsFn(ctx, a)
		},
		FetchDailyUsage: func(ctx context.Context, a codexapi.Auth, _, _ string, _ usage.Window, _ time.Time) (usage.Summary, error) {
			f.hit("daily")
			return f.dailyFn(ctx, a)
		},
		FetchProfileTokens: func(ctx context.Context, a codexapi.Auth) (usage.ProfileTokenUsage, error) {
			f.hit("profile")
			return f.profileFn(ctx, a)
		},
		ScanCost: func(ctx context.Context, q []usage.Query, _ time.Time, _ usage.RefreshPolicy, _ *usage.ProgressReporter) (map[string]usage.Summary, error) {
			f.hit("scan")
			return f.scanFn(ctx, q)
		},
		RecordAuth: func(a codex.Auth, _ string, _ time.Time) error {
			f.mu.Lock()
			f.recorded = append(f.recorded, a.Tokens.AccountID)
			f.mu.Unlock()
			return nil
		},
	}
}

// rec collects what the Service reports.
type rec struct {
	mu     sync.Mutex
	snaps  []Snapshot
	events []Event
}

func (r *rec) onSnapshot(s Snapshot) {
	r.mu.Lock()
	r.snaps = append(r.snaps, s)
	r.mu.Unlock()
}

func (r *rec) onEvent(e Event) {
	r.mu.Lock()
	r.events = append(r.events, e)
	r.mu.Unlock()
}

func (r *rec) alertCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, e := range r.events {
		if e.Kind == EventAlerts {
			n += len(e.Alerts)
		}
	}
	return n
}

func (r *rec) fullDone() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, e := range r.events {
		if e.Kind == EventFullRefreshDone {
			n++
		}
	}
	return n
}

func testPrefs() store.Preferences {
	p := store.DefaultPreferences()
	p.Language = store.LanguageEnglish
	return p
}

// newService builds a Service on the fake. tweak may change the options.
func newService(t *testing.T, f *fake, tweak func(*Options)) (*Service, *rec) {
	t.Helper()
	r := &rec{}
	dir := t.TempDir()
	o := Options{Features: personal.Features{Grok: true},
		Sources:     f.sources(),
		Preferences: testPrefs(),
		Now:         f.clk.Now,
		Location:    time.UTC,
		OnSnapshot:  r.onSnapshot,
		OnEvent:     r.onEvent,
	}
	o.StatusPath = dir + "/status.json"
	o.AlertsPath = dir + "/alerts.json"
	if tweak != nil {
		tweak(&o)
	}
	s := New(o)
	t.Cleanup(s.Close)
	return s, r
}

func wait(t testing.TB, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(10 * time.Second):
		t.Fatal("operation did not finish")
	}
}

func settle(t testing.TB, s *Service) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.WaitIdle(ctx); err != nil {
		t.Fatalf("service did not become idle: %v", err)
	}
}

func refreshAll(t testing.TB, s *Service) Snapshot {
	t.Helper()
	wait(t, s.Refresh(usage.RefreshForce))
	settle(t, s)
	return s.Snapshot()
}

var en = l10n.New(l10n.English)

var _ = export.RequireAll

// switchTo fakes the account switcher: it makes auth the current login.
func switchTo(f *fake, auth codex.Auth) func(context.Context, string) (codex.SwitchResult, error) {
	return func(_ context.Context, id string) (codex.SwitchResult, error) {
		f.setAuth(auth, nil)
		return codex.SwitchResult{Auth: auth, Account: codex.ManagedAccount{ID: id, PlanType: "plus"}}, nil
	}
}
