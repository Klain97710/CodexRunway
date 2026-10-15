package runway

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/codexapi"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/usage"
)

func TestFullRefreshFillsEverySection(t *testing.T) {
	f := newFake(t)
	s, r := newService(t, f, nil)
	showPanel(s)
	snap := refreshAll(t, s)
	c := snap.Codex
	if !c.Authenticated || c.Quota == nil || c.Quota.Plan != "plan-acct-a" {
		t.Fatalf("quota: %+v", c.Quota)
	}
	if c.ResetCredits == nil || c.ResetCreditSummary == nil || len(c.CreditDetails) != 1 {
		t.Error("credits missing")
	}
	if c.Cost == nil || c.DisplayedCost == nil || !strings.Contains(c.CostText, "$2.0000") {
		t.Errorf("cost: %q", c.CostText)
	}
	if c.Estimate == nil {
		t.Error("estimate missing")
	}
	if len(c.Heatmap.AllDevices) == 0 || c.Heatmap.CalculatedAt.IsZero() {
		t.Errorf("heatmap: %+v", c.Heatmap)
	}
	if len(c.Meters) != 2 || c.StatusText == "" || c.LastError != "" {
		t.Errorf("meters/status/error: %d %q %q", len(c.Meters), c.StatusText, c.LastError)
	}
	if snap.Refresh.Refreshing() {
		t.Errorf("still refreshing: %+v", snap.Refresh)
	}
	if r.fullDone() != 1 {
		t.Errorf("full refresh events: %d", r.fullDone())
	}
	if f.count("quota") != 1 || f.count("credits") != 1 {
		t.Errorf("one fetch per leg expected: %v", f.calls)
	}
}

func TestSnapshotSeqIncreasesAndIsImmutable(t *testing.T) {
	f := newFake(t)
	s, _ := newService(t, f, nil)
	first := refreshAll(t, s)
	meters := append([]Meter(nil), first.Codex.Meters...)
	f.quotaFn = func(context.Context, codexapi.Auth) (codexapi.QuotaSnapshot, error) {
		return f.quota("other", 90), nil
	}
	second := refreshAll(t, s)
	if second.Seq <= first.Seq {
		t.Errorf("seq %d !> %d", second.Seq, first.Seq)
	}
	for i, m := range first.Codex.Meters {
		if m.UsedPercent != meters[i].UsedPercent {
			t.Error("an earlier snapshot was modified")
		}
	}
	if second.Codex.Quota.Plan != "other" {
		t.Error("second snapshot did not update")
	}
}

func TestStartPublishesToCallbackAndCloseStops(t *testing.T) {
	f := newFake(t)
	s, r := newService(t, f, func(o *Options) { o.TickInterval = time.Hour })
	s.Start(context.Background())
	deadline := time.After(10 * time.Second)
	for {
		if s.Snapshot().Codex.Quota != nil && !s.Snapshot().Refresh.Refreshing() {
			break
		}
		select {
		case <-deadline:
			t.Fatal("first refresh never finished")
		case <-time.After(5 * time.Millisecond):
		}
	}
	s.Close()
	r.mu.Lock()
	n := len(r.snaps)
	var last uint64
	for _, sn := range r.snaps {
		if sn.Seq < last {
			t.Errorf("callback went backwards: %d after %d", sn.Seq, last)
		}
		last = sn.Seq
	}
	r.mu.Unlock()
	if n == 0 {
		t.Error("OnSnapshot never called")
	}
	if ch := s.Refresh(usage.RefreshForce); ch == nil {
		t.Error("nil channel after close")
	} else {
		wait(t, ch)
	}
}

// ---- stale results ---------------------------------------------------------

func TestLateQuotaOfSwitchedAccountIsDropped(t *testing.T) {
	f := newFake(t)
	authB := testAuth(t, "acct-b", "b@example.com", "refresh-b")
	g := newGate()
	f.quotaFn = func(_ context.Context, a codexapi.Auth) (codexapi.QuotaSnapshot, error) {
		if a.AccountID == "acct-a" {
			g.wait()
		}
		return f.quota("plan-"+a.AccountID, 20), nil
	}
	s, _ := newService(t, f, func(o *Options) {
		o.Sources.SwitchAccount = switchTo(f, authB)
	})
	done := s.RefreshQuota()
	g.waitStarted(t)
	epoch := s.Epoch()

	if err := s.SwitchAccount(context.Background(), "b"); err != nil {
		t.Fatal(err)
	}
	if s.Epoch() == epoch {
		t.Error("switch must start a new generation")
	}
	g.open()
	wait(t, done)
	settle(t, s)

	if q := s.Snapshot().Codex.Quota; q != nil && q.Plan == "plan-acct-a" {
		t.Fatal("late quota of the previous account was applied")
	}
	wait(t, s.Refresh(usage.RefreshForce))
	settle(t, s)
	if q := s.Snapshot().Codex.Quota; q == nil || q.Plan != "plan-acct-b" {
		t.Fatalf("quota after switch: %+v", q)
	}
}

func TestLateCreditsEstimateCostAndProfileAreDropped(t *testing.T) {
	cases := []struct {
		name  string
		hold  func(f *fake, g *gate)
		start func(s *Service) <-chan struct{}
		check func(t *testing.T, c CodexView)
	}{
		{"credits",
			func(f *fake, g *gate) {
				f.creditsFn = func(_ context.Context, a codexapi.Auth) (codexapi.ResetCreditsSnapshot, error) {
					if a.AccountID != "acct-a" {
						return codexapi.ResetCreditsSnapshot{}, errors.New("synthetic new account has no credits")
					}
					if a.AccountID == "acct-a" {
						g.wait()
					}
					return codexapi.ResetCreditsSnapshot{AvailableCount: 9, UpdatedAt: f.clk.Now()}, nil
				}
			},
			func(s *Service) <-chan struct{} { return s.RefreshResetCredits() },
			func(t *testing.T, c CodexView) {
				if c.ResetCredits != nil {
					t.Error("credits applied")
				}
			}},
		{"cost",
			func(f *fake, g *gate) {
				f.dailyFn = func(_ context.Context, a codexapi.Auth) (usage.Summary, error) {
					return usage.Summary{}, errors.New("synthetic new account has no analytics")
				}
				f.scanFn = func(_ context.Context, qs []usage.Query) (map[string]usage.Summary, error) {
					f.mu.Lock()
					account := f.auth.Tokens.AccountID
					f.mu.Unlock()
					if account != "acct-a" {
						return nil, errors.New("synthetic new account has no sessions")
					}
					g.wait()
					m := map[string]usage.Summary{}
					for _, q := range qs {
						m[q.ID] = usage.Summary{Source: usage.SourceLocalSessions, Confidence: usage.ConfidencePriced, Totals: usage.Totals{TotalTokens: 5}}
					}
					return m, nil
				}
			},
			func(s *Service) <-chan struct{} { return s.RefreshCost(usage.RefreshForce) },
			func(t *testing.T, c CodexView) {
				if c.Cost != nil || c.DisplayedCost != nil {
					t.Error("cost applied")
				}
			}},
		{"profile",
			func(f *fake, g *gate) {
				f.profileFn = func(ctx context.Context, a codexapi.Auth) (usage.ProfileTokenUsage, error) {
					if a.AccountID != "acct-a" {
						return usage.ProfileTokenUsage{}, nil
					}
					g.waitCtx(ctx) // the switch cancels it; it still reports success
					return usage.ProfileTokenUsage{DailyTokens: map[string]int64{"2026-09-07": 7}}, nil
				}
			},
			func(s *Service) <-chan struct{} { return s.RefreshTokenHeatmap(usage.RefreshForce) },
			func(t *testing.T, c CodexView) {
				if len(c.Heatmap.AllDevices) != 0 {
					t.Error("profile applied")
				}
			}},
		{"estimate",
			func(f *fake, g *gate) {
				f.dailyFn = func(_ context.Context, a codexapi.Auth) (usage.Summary, error) {
					if a.AccountID != "acct-a" {
						return usage.Summary{}, errors.New("synthetic new account has no analytics")
					}
					g.wait()
					return usage.Summary{Confidence: usage.ConfidencePriced, Totals: usage.Totals{TotalTokens: 5}}, nil
				}
			},
			func(s *Service) <-chan struct{} { return s.RefreshQuotaEstimate() },
			func(t *testing.T, c CodexView) {
				if c.Estimate != nil {
					t.Error("estimate applied")
				}
			}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFake(t)
			authB := testAuth(t, "acct-b", "b@example.com", "refresh-b")
			g := newGate()
			g.off.Store(true)
			tc.hold(f, g)
			s, _ := newService(t, f, func(o *Options) { o.Sources.SwitchAccount = switchTo(f, authB) })
			// A quota reading has to exist for the estimate / cost flows.
			wait(t, s.RefreshQuota())
			settle(t, s)
			f.mu.Lock()
			f.calls = map[string]int{}
			f.mu.Unlock()
			g.off.Store(false)

			done := tc.start(s)
			g.waitStarted(t)
			if err := s.SwitchAccount(context.Background(), "b"); err != nil {
				t.Fatal(err)
			}
			g.open()
			wait(t, done)
			settle(t, s)
			tc.check(t, s.Snapshot().Codex)
		})
	}
}

func TestLateCostScanAfterAuthFailureKeepsError(t *testing.T) {
	f := newFake(t)
	g := newGate()
	f.scanFn = func(_ context.Context, qs []usage.Query) (map[string]usage.Summary, error) {
		g.wait()
		return map[string]usage.Summary{queryCurrent: {Source: usage.SourceLocalSessions, Confidence: usage.ConfidencePriced, Totals: usage.Totals{TotalTokens: 5}}}, nil
	}
	s, _ := newService(t, f, nil)
	done := s.RefreshCost(usage.RefreshForce)
	g.waitStarted(t)

	f.quotaFn = func(context.Context, codexapi.Auth) (codexapi.QuotaSnapshot, error) {
		return codexapi.QuotaSnapshot{}, codexapi.ErrAuthenticationRequired
	}
	wait(t, s.RefreshQuota())
	msg := s.Snapshot().Codex.LastError
	if msg == "" || s.Snapshot().Codex.Authenticated {
		t.Fatalf("auth failure not recorded: %q", msg)
	}

	g.open()
	wait(t, done)
	settle(t, s)
	c := s.Snapshot().Codex
	if c.Cost != nil || c.DisplayedCost != nil {
		t.Error("late cost scan restored the cost after sign-out")
	}
	if c.LastError != msg {
		t.Errorf("auth error hidden: %q", c.LastError)
	}
}

func TestProfileFailureAfterCreditsAuthFailurePreservesAuthError(t *testing.T) {
	f := newFake(t)
	g := newGate()
	f.profileFn = func(context.Context, codexapi.Auth) (usage.ProfileTokenUsage, error) {
		g.wait()
		return usage.ProfileTokenUsage{}, errors.New("profile boom")
	}
	s, _ := newService(t, f, nil)
	done := s.RefreshTokenHeatmap(usage.RefreshForce)
	g.waitStarted(t)

	f.creditsFn = func(context.Context, codexapi.Auth) (codexapi.ResetCreditsSnapshot, error) {
		return codexapi.ResetCreditsSnapshot{}, codexapi.ErrAuthenticationRequired
	}
	wait(t, s.RefreshResetCredits())
	msg := s.Snapshot().Codex.LastError
	if msg == "" {
		t.Fatal("auth failure not recorded")
	}
	g.open()
	wait(t, done)
	settle(t, s)
	c := s.Snapshot().Codex
	if c.LastError != msg {
		t.Errorf("auth error replaced by %q", c.LastError)
	}
	if !c.Heatmap.CalculatedAt.IsZero() {
		t.Error("heatmap of a signed-out account was marked calculated")
	}
}

// ---- authentication ----------------------------------------------------------

func TestAuthFailureClearsEverythingShownForTheAccount(t *testing.T) {
	f := newFake(t)
	s, _ := newService(t, f, nil)
	if c := refreshAll(t, s).Codex; c.Quota == nil {
		t.Fatal("setup")
	}
	f.quotaFn = func(context.Context, codexapi.Auth) (codexapi.QuotaSnapshot, error) {
		return codexapi.QuotaSnapshot{}, codexapi.ErrAuthenticationRequired
	}
	snap := refreshAll(t, s)
	c := snap.Codex
	if c.Quota != nil || len(c.Meters) != 0 || len(c.QuotaLines) != 0 || c.ResetCredits != nil || c.Cost != nil {
		t.Errorf("data survived the sign-out: %+v", c)
	}
	if c.Authenticated || snap.Accounts.Display.Authenticated {
		t.Error("still authenticated")
	}
	if c.StatusText != en.Text(l10n.KeyStatusLogin) {
		t.Errorf("status text %q", c.StatusText)
	}
	if c.LastError == "" {
		t.Error("no error shown")
	}
}

func TestLocalAuthFileFailureIsHumanized(t *testing.T) {
	f := newFake(t)
	f.setAuth(testAuth(t, "x", "x@example.com", "r"), &AuthLoadError{Kind: AuthExpired, Err: errors.New("detail")})
	s, _ := newService(t, f, nil)
	c := refreshAll(t, s).Codex
	if c.LastError != en.Text(l10n.KeyAuthExpired) {
		t.Errorf("error %q", c.LastError)
	}
	if f.count("quota") != 0 {
		t.Error("network used without a credential")
	}
	f.setAuth(testAuth(t, "x", "x@example.com", "r"), &AuthLoadError{Kind: AuthFileInvalid})
	if c := refreshAll(t, s).Codex; c.LastError != en.Text(l10n.KeyAuthFileInvalid) {
		t.Errorf("error %q", c.LastError)
	}
}

func TestTemporaryFailureKeepsLastQuota(t *testing.T) {
	f := newFake(t)
	s, _ := newService(t, f, nil)
	refreshAll(t, s)
	f.quotaFn = func(context.Context, codexapi.Auth) (codexapi.QuotaSnapshot, error) {
		return codexapi.QuotaSnapshot{}, errors.New("network down")
	}
	c := refreshAll(t, s).Codex
	if c.Quota == nil || !c.Authenticated {
		t.Fatal("temporary failure dropped the quota")
	}
	if !strings.Contains(c.LastError, "network down") {
		t.Errorf("error %q", c.LastError)
	}
	f.quotaFn = func(_ context.Context, a codexapi.Auth) (codexapi.QuotaSnapshot, error) {
		return f.quota("back", 5), nil
	}
	c = refreshAll(t, s).Codex
	if c.LastError != "" || c.Quota.Plan != "back" {
		t.Errorf("recovery: %q %+v", c.LastError, c.Quota)
	}
}

func TestSecondaryLegAuthFailureInvalidatesQuota(t *testing.T) {
	authErr := codexapi.ErrAuthenticationRequired
	legs := map[string]func(f *fake, p *Options){
		"credits": func(f *fake, _ *Options) {
			f.creditsFn = func(context.Context, codexapi.Auth) (codexapi.ResetCreditsSnapshot, error) {
				return codexapi.ResetCreditsSnapshot{}, authErr
			}
		},
		"estimate": func(f *fake, p *Options) {
			p.Preferences.ShowsCostSummary, p.Preferences.ShowsTokenUsageHeatmap = false, false
			f.dailyFn = func(context.Context, codexapi.Auth) (usage.Summary, error) { return usage.Summary{}, authErr }
		},
	}
	for name, setup := range legs {
		t.Run(name, func(t *testing.T) {
			f := newFake(t)
			s, _ := newService(t, f, func(o *Options) { setup(f, o) })
			showPanel(s)
			snap := refreshAll(t, s)
			c := snap.Codex
			if c.Quota != nil || len(c.Meters) != 0 || c.Authenticated {
				t.Errorf("quota survived an auth failure of %s: %+v", name, c.Quota)
			}
			if c.StatusText != en.Text(l10n.KeyStatusLogin) || c.LastError == "" {
				t.Errorf("status %q err %q", c.StatusText, c.LastError)
			}
		})
	}
}

func TestOnlineCostFailureIsNotASignOut(t *testing.T) {
	f := newFake(t)
	f.scanFn = func(context.Context, []usage.Query) (map[string]usage.Summary, error) {
		return map[string]usage.Summary{}, nil
	}
	f.dailyFn = func(context.Context, codexapi.Auth) (usage.Summary, error) {
		return usage.Summary{}, codexapi.ErrAuthenticationRequired
	}
	s, _ := newService(t, f, func(o *Options) { o.Preferences.ShowsQuotaEstimateSummary = false })
	showPanel(s)
	c := refreshAll(t, s).Codex
	if c.Quota == nil || !c.Authenticated {
		t.Fatal("cost failure signed the account out")
	}
	if c.CostText != en.Text(l10n.KeyUsageAnalyticsUnavailable) {
		t.Errorf("cost text %q", c.CostText)
	}
}

func TestIdentityChangeStartsNewGeneration(t *testing.T) {
	f := newFake(t)
	s, _ := newService(t, f, nil)
	refreshAll(t, s)
	e := s.Epoch()
	f.setAuth(testAuth(t, "acct-b", "b@example.com", "refresh-b"), nil)
	snap := refreshAll(t, s)
	if s.Epoch() == e {
		t.Error("another login must start a new generation")
	}
	if snap.Codex.Quota == nil || snap.Codex.Quota.Plan != "plan-acct-b" {
		t.Errorf("quota %+v", snap.Codex.Quota)
	}
}

// ---- cancellation ------------------------------------------------------------

func TestWatchdogCancelsHungRefresh(t *testing.T) {
	f := newFake(t)
	f.quotaFn = func(ctx context.Context, _ codexapi.Auth) (codexapi.QuotaSnapshot, error) {
		<-ctx.Done()
		return codexapi.QuotaSnapshot{}, ctx.Err()
	}
	s, _ := newService(t, f, func(o *Options) { o.FullRefreshTimeout = 50 * time.Millisecond })
	wait(t, s.Refresh(usage.RefreshForce))
	settle(t, s)
	if s.IsRefreshing(SectionQuota) || s.Snapshot().Refresh.Refreshing() {
		t.Error("watchdog did not release the pipeline")
	}
	f.quotaFn = func(context.Context, codexapi.Auth) (codexapi.QuotaSnapshot, error) { return f.quota("ok", 1), nil }
	if c := refreshAll(t, s).Codex; c.Quota == nil {
		t.Error("next refresh did not run")
	}
}

func TestSwitchCancelsRunningRefresh(t *testing.T) {
	f := newFake(t)
	authB := testAuth(t, "acct-b", "b@example.com", "refresh-b")
	started := make(chan struct{})
	f.quotaFn = func(ctx context.Context, a codexapi.Auth) (codexapi.QuotaSnapshot, error) {
		if a.AccountID == "acct-a" {
			close(started)
			<-ctx.Done()
			return codexapi.QuotaSnapshot{}, ctx.Err()
		}
		return f.quota("plan-"+a.AccountID, 1), nil
	}
	s, _ := newService(t, f, func(o *Options) { o.Sources.SwitchAccount = switchTo(f, authB) })
	full := s.Refresh(usage.RefreshForce)
	<-started
	if err := s.SwitchAccount(context.Background(), "b"); err != nil {
		t.Fatal(err)
	}
	wait(t, full)
	settle(t, s)
	c := s.Snapshot().Codex
	if c.Quota == nil || c.Quota.Plan != "plan-acct-b" {
		t.Errorf("quota after switch: %+v", c.Quota)
	}
	if c.LastError != "" {
		t.Errorf("a cancelled refresh left an error: %q", c.LastError)
	}
}

func TestOverlappingRefreshJoinsRunningOne(t *testing.T) {
	f := newFake(t)
	g := newGate()
	f.quotaFn = func(_ context.Context, a codexapi.Auth) (codexapi.QuotaSnapshot, error) {
		g.wait()
		return f.quota("p", 1), nil
	}
	s, _ := newService(t, f, nil)
	a := s.Refresh(usage.RefreshForce)
	g.waitStarted(t)
	b := s.Refresh(usage.RefreshForce)
	g.open()
	wait(t, a)
	wait(t, b)
	settle(t, s)
	if n := f.count("quota"); n != 1 {
		t.Errorf("quota fetched %d times", n)
	}
}
