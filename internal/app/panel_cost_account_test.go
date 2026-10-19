package app

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/store"
	"github.com/licoy/quosparr/internal/usage"
)

func TestPanelCostQueryCompletesForSelectedProvider(t *testing.T) {
	for _, tc := range []struct {
		name              string
		provider          store.Provider
		codexID, claudeID string
	}{
		{"codex_control", store.ProviderCodex, "codex-account", "claude-account"},
		{"grok_control", store.ProviderGrok, "codex-account", "claude-account"},
		{"claude_distinct_accounts", store.ProviderClaude, "codex-account", "claude-account"},
		{"claude_without_codex_login", store.ProviderClaude, "", "claude-account"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			queue := detailMainQueue(t)
			snap := panelFixture()
			snap.Accounts.ActiveID = tc.codexID
			snap.Grok.Accounts.ActiveID = "grok-account"
			snap.Claude.Accounts.ActiveID = tc.claudeID
			returned := make(chan store.Provider, 1)
			acts := PanelActions{QueryCost: func(_ context.Context, provider store.Provider, _ CostQuery) (usage.Summary, error) {
				sum := *uiReviewDetails(panelFixture()).Codex.DisplayedCost
				sum.Totals.TotalTokens = 1234
				returned <- provider
				return sum, nil
			}}
			a, tt := newPanel(t, func(p *store.Preferences) { p.SelectedProvider = tc.provider }, snap, acts)
			t.Cleanup(a.cancelPanelDetails)
			a.panelState.detail = store.ModuleAPICost
			tt.Frame()
			finishDetail(t, queue)
			if got := <-returned; got != tc.provider {
				t.Fatalf("query sent to %s, want %s", got, tc.provider)
			}
			tt.Frame()
			t.Logf("query finished successfully; captured_account=%q actual_account=%q running=%t summary_present=%t calculating_visible=%t", a.panelState.account, activePanelAccount(a.Snapshot(), tc.provider), a.panelState.cost.task.Running, a.panelState.cost.summary != nil, tt.HasText(a.L().Text(l10n.KeyCalculating)))
			if a.panelState.cost.task.Running || a.panelState.cost.summary == nil || !tt.HasText("1.23K") {
				t.Fatalf("completed cost query remains stuck: %q", tt.Texts())
			}
		})
	}
}

func TestPanelClaudeCostQueryErrorStopsLoading(t *testing.T) {
	queue := detailMainQueue(t)
	snap := panelFixture()
	snap.Claude.Accounts.ActiveID = "claude-account"
	a, tt := newPanel(t, func(p *store.Preferences) { p.SelectedProvider = store.ProviderClaude }, snap, PanelActions{
		QueryCost: func(context.Context, store.Provider, CostQuery) (usage.Summary, error) {
			return usage.Summary{}, errDetailUnavailable
		},
	})
	t.Cleanup(a.cancelPanelDetails)
	a.panelState.detail = store.ModuleAPICost
	tt.Frame()
	finishDetail(t, queue)
	tt.Frame()
	if a.panelState.cost.task.Running || !tt.HasText(tx(a.L(), xDetailUnavailable)) {
		t.Fatalf("query error remains hidden behind loading: %q", tt.Texts())
	}
}

func TestPanelClaudeCostQueryFollowsOnlyClaudeAccount(t *testing.T) {
	queue := detailMainQueue(t)
	started := make(chan context.Context, 2)
	releaseOld := make(chan struct{})
	var release sync.Once
	t.Cleanup(func() { release.Do(func() { close(releaseOld) }) })
	var calls atomic.Int32
	snap := panelFixture()
	snap.Claude.Accounts.ActiveID = "claude-old"
	a, tt := newPanel(t, func(p *store.Preferences) { p.SelectedProvider = store.ProviderClaude }, snap, PanelActions{
		QueryCost: func(ctx context.Context, _ store.Provider, _ CostQuery) (usage.Summary, error) {
			call := calls.Add(1)
			started <- ctx
			if call == 1 {
				<-releaseOld
			}
			sum := *uiReviewDetails(panelFixture()).Codex.DisplayedCost
			sum.Totals.TotalTokens = int64(call) * 111
			return sum, nil
		},
	})
	t.Cleanup(a.cancelPanelDetails)
	nextContext := func() context.Context {
		t.Helper()
		select {
		case ctx := <-started:
			return ctx
		case <-time.After(3 * time.Second):
			t.Fatal("cost query did not start")
			return nil
		}
	}
	a.panelState.detail = store.ModuleAPICost
	tt.Frame()
	oldContext := nextContext()
	a.snapMu.Lock()
	a.snap.Accounts.ActiveID = "codex-changed"
	a.snapMu.Unlock()
	tt.Frame()
	if oldContext.Err() != nil || calls.Load() != 1 {
		t.Fatal("unrelated Codex account change cancelled Claude cost query")
	}
	a.snapMu.Lock()
	a.snap.Claude.Accounts.ActiveID = "claude-new"
	a.snapMu.Unlock()
	tt.Frame()
	if oldContext.Err() == nil {
		t.Fatal("Claude account change did not cancel previous query")
	}
	newContext := nextContext()
	if newContext.Err() != nil {
		t.Fatal("new Claude query already cancelled")
	}
	finishDetail(t, queue)
	tt.Frame()
	if a.panelState.cost.task.Running || a.panelState.cost.summary == nil || a.panelState.cost.summary.Totals.TotalTokens != 222 {
		t.Fatalf("new account result not shown: %q", tt.Texts())
	}
	release.Do(func() { close(releaseOld) })
	finishDetail(t, queue)
	tt.Frame()
	if a.panelState.cost.task.Running || a.panelState.cost.summary == nil || a.panelState.cost.summary.Totals.TotalTokens != 222 {
		t.Fatal("late result from previous account replaced current result")
	}
}
