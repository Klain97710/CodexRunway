package app

import (
	"context"
	"errors"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/codex"
	"github.com/licoy/quosparr/internal/codexapi"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/resettoday"
	"github.com/licoy/quosparr/internal/runway"
	"github.com/licoy/quosparr/internal/sessions"
	"github.com/licoy/quosparr/internal/store"
	"github.com/licoy/quosparr/internal/usage"
)

func quietLivePreferences() store.Preferences {
	p := store.DefaultPreferences()
	p.ShowsCostSummary, p.ShowsTokenUsageHeatmap, p.ShowsRecentSessions = false, false, false
	p.ShowsQuotaEstimateSummary, p.ShowsRateLimitResetToday, p.ShowsSessionRepairSummary = false, false, false
	return p
}

func TestLivePreferenceBurstClosesWithLatestValue(t *testing.T) {
	p := quietLivePreferences()
	g := newGrokBridge(nil, p, nil)
	svc := runway.New(runway.Options{Preferences: p, Grok: g})
	defer svc.Close()
	rt := &liveApp{svc: svc, grok: g, reset: newResetBridge(nil, p, nil)}
	g.mu.Lock()
	first := p.Clone()
	first.AccountPrivacyModeEnabled = true
	rt.enqueuePreferences(first)
	deadline := time.Now().Add(time.Second)
	for {
		rt.preferenceWork.mu.Lock()
		taken := rt.preferenceWork.pending == nil
		rt.preferenceWork.mu.Unlock()
		if taken {
			break
		}
		if time.Now().After(deadline) {
			g.mu.Unlock()
			t.Fatal("worker did not take preferences")
		}
		time.Sleep(time.Millisecond)
	}
	latest := p.Clone()
	for i := 0; i < 100; i++ {
		latest.AccountPrivacyModeEnabled = i%2 == 0
		latest.MainPanelHeight = float64(400 + i)
		rt.enqueuePreferences(latest)
	}
	latest.Language = store.LanguageSimplifiedChinese
	latest.APICostSummaryRange = store.CostRangeToday
	rt.enqueuePreferences(latest)
	done := make(chan struct{})
	go func() { rt.closePreferences(); close(done) }()
	select {
	case <-done:
		g.mu.Unlock()
		t.Fatal("close skipped in-flight preferences")
	case <-time.After(20 * time.Millisecond):
	}
	g.mu.Unlock()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("close did not finish")
	}
	if got := svc.Preferences(); !reflect.DeepEqual(got, latest) {
		t.Fatalf("last preferences lost: got %s/%v/%v, want %s/%v/%v", got.Language, got.MainPanelHeight, got.AccountPrivacyModeEnabled, latest.Language, latest.MainPanelHeight, latest.AccountPrivacyModeEnabled)
	}
	if svc.Snapshot().Language != l10n.SimplifiedChinese {
		t.Fatal("last language not published")
	}
	rt.enqueuePreferences(p)
	if !reflect.DeepEqual(svc.Preferences(), latest) {
		t.Fatal("closed runtime accepted later change")
	}
}

func TestLiveRefreshUsesLatestUIProvider(t *testing.T) {
	e := newEnv(t, nil)
	p := quietLivePreferences()
	e.s.prefs = p
	var codexLoads atomic.Int32
	g := newGrokBridge(nil, p, nil)
	svc := runway.New(runway.Options{Preferences: p, Grok: g, Sources: runway.Sources{LoadValidAuth: func(context.Context, bool, *codex.Auth) (codex.Auth, error) {
		codexLoads.Add(1)
		return codex.Auth{}, errors.New("unexpected Codex refresh")
	}}})
	defer svc.Close()
	rt := &liveApp{svc: svc, grok: g, reset: newResetBridge(e.s, p, nil)}
	e.s.OnChange(func() { rt.pushPreferences(e.s) })
	// Preferences are still pending when the button is pressed; dispatch must
	// follow the UI's Grok selection rather than the service's old Codex value.
	g.mu.Lock()
	e.s.SetSelectedProvider(store.ProviderGrok)
	rt.panelActions(e.s).RefreshSection(runway.SectionQuota)
	g.mu.Unlock()
	rt.preferenceWork.idle.Wait()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := svc.WaitIdle(ctx); err != nil {
		t.Fatal(err)
	}
	if codexLoads.Load() != 0 {
		t.Fatalf("refreshed old provider %d times", codexLoads.Load())
	}
	if svc.Preferences().SelectedProvider != store.ProviderGrok || svc.Snapshot().Provider != store.ProviderGrok {
		t.Fatal("latest provider was not published")
	}
	rt.closePreferences()
	e.settle()
}

func TestLiveRefreshUsesLatestUICostRange(t *testing.T) {
	e := newEnv(t, nil)
	p := quietLivePreferences()
	p.ShowsCostSummary = true
	p.APICostSummaryRange = store.CostRangeCurrent
	e.s.prefs = p
	requested := make(chan usage.Window, 2)
	g := newGrokBridge(nil, p, nil)
	svc := runway.New(runway.Options{Preferences: p, Grok: g, Sources: runway.Sources{
		LoadValidAuth: func(context.Context, bool, *codex.Auth) (codex.Auth, error) {
			return codex.APIKeyAuth("synthetic-test"), nil
		},
		FetchQuota: func(context.Context, codexapi.Auth) (codexapi.QuotaSnapshot, error) {
			return codexapi.QuotaSnapshot{}, nil
		},
		ScanCost: func(_ context.Context, queries []usage.Query, _ time.Time, _ usage.RefreshPolicy, _ *usage.ProgressReporter) (map[string]usage.Summary, error) {
			for _, q := range queries {
				requested <- q.Window
			}
			out := map[string]usage.Summary{}
			for _, q := range queries {
				out[q.ID] = usage.Summary{Window: q.Window, Totals: usage.Totals{TotalTokens: 1}}
			}
			return out, nil
		},
	}})
	defer svc.Close()
	rt := &liveApp{svc: svc, grok: g, reset: newResetBridge(e.s, p, nil)}
	e.s.OnChange(func() { rt.pushPreferences(e.s) })
	g.mu.Lock()
	e.s.SetAPICostRange(store.CostRangeToday)
	rt.panelActions(e.s).RefreshSection(runway.SectionAPICost)
	g.mu.Unlock()
	rt.preferenceWork.idle.Wait()
	select {
	case got := <-requested:
		want := usage.Today(time.Now(), time.Local).Window
		if !got.Start.Equal(want.Start) {
			t.Fatalf("old cost range requested: %+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("selected cost range was not refreshed")
	}
	rt.closePreferences()
	e.settle()
}

func TestPendingCodexSectionsAfterProviderSwitch(t *testing.T) {
	for _, section := range []runway.Section{runway.SectionQuotaEstimate, runway.SectionSessionRepair, runway.SectionResetToday} {
		t.Run(string(section), func(t *testing.T) {
			e := newEnv(t, nil)
			p := quietLivePreferences()
			p.SelectedProvider = store.ProviderGrok
			p.ShowsQuotaEstimateSummary, p.ShowsSessionRepairSummary, p.ShowsRateLimitResetToday = true, true, true
			e.s.prefs = p
			var codexReads, sessionScans, resetReads atomic.Int32
			g := newGrokBridge(nil, p, nil)
			reset := newResetBridge(e.s, p, nil)
			reset.ref.Fetch = func(context.Context, time.Time, *time.Location) (*resettoday.Snapshot, error) {
				resetReads.Add(1)
				return nil, errors.New("synthetic feed error")
			}
			svc := runway.New(runway.Options{Preferences: p, Grok: g, ResetToday: reset, Sources: runway.Sources{
				LoadValidAuth: func(context.Context, bool, *codex.Auth) (codex.Auth, error) {
					codexReads.Add(1)
					return codex.Auth{}, errors.New("Codex refresh is not supported by selected provider")
				},
				DryRunSessions: func(context.Context) (sessions.Report, error) { sessionScans.Add(1); return sessions.Report{}, nil },
			}})
			defer svc.Close()
			rt := &liveApp{svc: svc, grok: g, reset: reset}
			e.s.OnChange(func() { rt.pushPreferences(e.s) })
			// Hold consumption at the real "worker scheduled, not yet run" boundary:
			// a UI burst switches to Codex, asks for a section, then switches back.
			// Only the final Grok preference survives, but the earlier section remains.
			rt.preferenceWork.running = true
			rt.preferenceWork.idle.Add(1)
			e.s.SetSelectedProvider(store.ProviderCodex)
			rt.panelActions(e.s).RefreshSection(section)
			e.s.SetSelectedProvider(store.ProviderGrok)
			go rt.drainPreferences()
			rt.preferenceWork.idle.Wait()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if err := svc.WaitIdle(ctx); err != nil {
				t.Fatal(err)
			}
			if codexReads.Load() != 0 || sessionScans.Load() != 0 {
				t.Fatalf("old Codex-only section ran after Grok selection: auth=%d sessions=%d", codexReads.Load(), sessionScans.Load())
			}
			if section == runway.SectionResetToday && resetReads.Load() != 1 {
				t.Fatalf("shared reset feed was dropped: fetches=%d", resetReads.Load())
			}
			rt.closePreferences()
			e.settle()
		})
	}
}
