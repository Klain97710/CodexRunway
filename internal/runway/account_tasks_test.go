package runway

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/codexapi"
	"github.com/licoy/quosparr/internal/store"
	"github.com/licoy/quosparr/internal/usage"
)

func TestAccountSwitchStartsNewRefreshWhileOldQuotaIgnoresCancellation(t *testing.T) {
	f := newFake(t)
	gate := newGate()
	authB := testAuth(t, "acct-b", "b@example.com", "refresh-b")
	f.quotaFn = func(_ context.Context, a codexapi.Auth) (codexapi.QuotaSnapshot, error) {
		if a.AccountID == "acct-a" {
			gate.wait()
			return f.quota("old", 99), nil
		}
		return f.quota("new", 25), nil
	}
	s, _ := newService(t, f, func(o *Options) { o.Sources.SwitchAccount = switchTo(f, authB) })
	done := s.RefreshQuota()
	gate.waitStarted(t)
	if err := s.SwitchAccount(context.Background(), "b"); err != nil {
		t.Fatal(err)
	}
	// Joining the new full refresh must not join the old standalone request.
	wait(t, s.Refresh(usage.RefreshForce))
	if q := s.Snapshot().Codex.Quota; q == nil || q.Plan != "new" {
		t.Fatal("new account did not refresh")
	}
	gate.open()
	wait(t, done)
	settle(t, s)
	if q := s.Snapshot().Codex.Quota; q == nil || q.Plan != "new" {
		t.Fatal("old result overwrote new account")
	}
}

func TestCostDetailFlightDoesNotCrossAccounts(t *testing.T) {
	f := newFake(t)
	gate := newGate()
	authB := testAuth(t, "acct-b", "b@example.com", "refresh-b")
	f.scanFn = func(_ context.Context, queries []usage.Query) (map[string]usage.Summary, error) {
		f.mu.Lock()
		id := f.auth.Tokens.AccountID
		f.mu.Unlock()
		if id == "acct-a" {
			gate.wait()
		}
		tokens := int64(22)
		if id == "acct-a" {
			tokens = 99
		}
		out := map[string]usage.Summary{}
		for _, q := range queries {
			out[q.ID] = usage.Summary{Confidence: usage.ConfidencePriced, Totals: usage.Totals{TotalTokens: tokens}}
		}
		return out, nil
	}
	s, _ := newService(t, f, func(o *Options) { o.Sources.SwitchAccount = switchTo(f, authB) })
	rng := usage.Today(f.clk.Now(), time.UTC)
	oldDone := make(chan error, 1)
	go func() { _, err := s.QueryCost(context.Background(), rng); oldDone <- err }()
	gate.waitStarted(t)
	if err := s.SwitchAccount(context.Background(), "b"); err != nil {
		t.Fatal(err)
	}
	sum, err := s.QueryCost(context.Background(), rng)
	if err != nil || sum.Totals.TotalTokens != 22 {
		t.Fatalf("new account joined old cost: %d, %v", sum.Totals.TotalTokens, err)
	}
	gate.open()
	if err := <-oldDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("old cost returned success: %v", err)
	}
	settle(t, s)
}

func TestProviderChangeCancelsStandaloneCostBeforeNewRefresh(t *testing.T) {
	f := newFake(t)
	gate := newGate()
	g := &fakeGrok{}
	var calls atomic.Int32
	f.quotaFn = func(ctx context.Context, _ codexapi.Auth) (codexapi.QuotaSnapshot, error) {
		if calls.Add(1) == 1 {
			gate.waitCtx(ctx)
			return f.quota("late", 99), nil
		}
		return f.quota("new", 25), nil
	}
	s, _ := newService(t, f, func(o *Options) { o.Grok = g })
	done := s.RefreshCost(usage.RefreshForce)
	gate.waitStarted(t)
	p := s.Preferences()
	p.SelectedProvider = store.ProviderGrok
	s.SetPreferences(p)
	wait(t, done)
	settle(t, s)
	if g.count() != 1 {
		t.Fatalf("new provider refreshes = %d", g.count())
	}
	if s.IsRefreshing(SectionAPICost) {
		t.Fatal("old cost kept section busy")
	}
	if q := s.Snapshot().Codex.Quota; q != nil && q.Plan == "late" {
		t.Fatal("cancelled standalone result overwrote state")
	}
}

func TestCloseCancelsStandaloneRefreshAndRejectsNewWork(t *testing.T) {
	f := newFake(t)
	gate := newGate()
	f.quotaFn = func(ctx context.Context, _ codexapi.Auth) (codexapi.QuotaSnapshot, error) {
		gate.waitCtx(ctx)
		return f.quota("late", 99), ctx.Err()
	}
	s, _ := newService(t, f, nil)
	done := s.RefreshCost(usage.RefreshForce)
	gate.waitStarted(t)
	s.Close()
	wait(t, done)
	wait(t, s.RefreshCost(usage.RefreshForce))
	if s.IsRefreshing(SectionAPICost) {
		t.Fatal("closed service left cost busy")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.standalone) != 0 {
		t.Fatal("closed service retained standalone tasks")
	}
}
