package runway

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/licoy/quosparr/internal/codex"
	"github.com/licoy/quosparr/internal/estimate"
	"github.com/licoy/quosparr/internal/store"
	"github.com/licoy/quosparr/internal/usage"
)

func TestUnrelatedPreferencesRetainEstimateAndModeUsesLoadedHistory(t *testing.T) {
	f := newFake(t)
	path := filepath.Join(t.TempDir(), "history.json")
	s, _ := newService(t, f, func(o *Options) { o.EstimateHistoryPath = path })
	sample := estimate.HistorySample{CycleStartDate: "2026-08-31", EstimatedCredits: 1000, UsedPercent: 30, RecordedAt: t0}
	if _, err := (estimate.HistoryStore{Path: path}).Upsert(codex.MatchKey(f.auth), sample); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	q := f.quota("plus", 20)
	daily := usage.Summary{CalculatedAt: t0, DailyRows: []usage.DailyRow{{Date: "2026-09-07", RawCredits: 100, Totals: usage.Totals{TotalTokens: 1000}}}}
	s.auth = &f.auth
	s.quota = &q
	s.estDaily = &daily
	s.applyEstimateLocked(q, daily, true)
	old := s.est
	s.mu.Unlock()
	p := s.Preferences()
	p.AccountPrivacyModeEnabled = !p.AccountPrivacyModeEnabled
	s.SetPreferences(p)
	if s.Snapshot().Codex.Estimate != old {
		t.Error("privacy setting recomputed unchanged quota estimate")
	}
	// A UI-only window selection must use the history of the already loaded
	// estimate; refreshing the underlying provider is the disk reload boundary.
	if err := os.WriteFile(path, []byte("{damaged"), 0600); err != nil {
		t.Fatal(err)
	}
	p.QuotaEstimateWindowMode = store.WindowModeRollingWeek
	s.SetPreferences(p)
	got := s.Snapshot().Codex.Estimate
	if got == old || got.WindowMode != estimate.ModeRollingWeek {
		t.Fatal("window mode was not recomputed")
	}
	if len(got.History) != len(old.History) {
		t.Fatalf("mode change reread history: got %d, want %d", len(got.History), len(old.History))
	}
}

func TestChangingCostRangeDoesNotLoadHiddenModule(t *testing.T) {
	for _, provider := range []store.Provider{store.ProviderCodex, store.ProviderGrok} {
		t.Run(string(provider), func(t *testing.T) {
			f := newFake(t)
			g := &fakeGrok{}
			s, _ := newService(t, f, func(o *Options) {
				o.Grok = g
				o.Preferences.SelectedProvider = provider
				o.Preferences.ShowsCostSummary = false
				o.Preferences.ShowsQuotaEstimateSummary = false
			})
			p := s.Preferences()
			p.APICostSummaryRange = store.CostRangeThisMonth
			s.SetPreferences(p)
			settle(t, s)
			if f.count("scan") != 0 || g.count() != 0 {
				t.Fatalf("hidden cost range fetched data: codex=%d grok=%d", f.count("scan"), g.count())
			}
		})
	}
}
