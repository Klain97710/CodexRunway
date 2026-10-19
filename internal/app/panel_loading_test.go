package app

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/runway"
	"github.com/licoy/quosparr/internal/store"
)

func TestPanelPendingModulesShowLoading(t *testing.T) {
	for _, provider := range []store.Provider{store.ProviderCodex, store.ProviderGrok, store.ProviderClaude} {
		modules := []store.Module{store.ModuleQuota, store.ModuleSpeedAnalysis, store.ModuleAPICost, store.ModuleTokenUsage, store.ModuleRecentSessions}
		if provider != store.ProviderGrok {
			modules = append(modules, store.ModuleResetCredits)
		}
		if provider == store.ProviderCodex {
			modules = append(modules, store.ModuleQuotaEstimate, store.ModuleSessionRepair)
		}
		for _, module := range modules {
			t.Run(string(provider)+"/"+string(module), func(t *testing.T) {
				snap := runway.Snapshot{Refresh: runway.RefreshView{All: true}}
				_, tt := newPanel(t, func(p *store.Preferences) {
					p.SelectedProvider = provider
					loadingTestModules(p, module)
				}, snap, PanelActions{})
				l := l10n.New(l10n.English)
				if !hasExact(tt, tx(l, xPanelLoading)) || hasExact(tt, l.Text(l10n.KeyNotLoaded)) {
					t.Fatalf("pending module should show loading, got %q", tt.Texts())
				}
				if box, ok := tt.Find(tx(l, xPanelLoading)); !ok || box.W != 14 || box.H != 14 {
					t.Fatalf("missing labeled loading spinner: %+v", box)
				}
			})
		}
	}
}

func loadingTestModules(p *store.Preferences, modules ...store.Module) {
	for _, m := range store.Modules() {
		if shown := moduleShown(p, m); shown != nil {
			*shown = false
			for _, wanted := range modules {
				*shown = *shown || m == wanted
			}
		}
	}
}

func TestPanelProviderLoadingPreservesResultsAndErrors(t *testing.T) {
	for _, provider := range []store.Provider{store.ProviderCodex, store.ProviderGrok, store.ProviderClaude} {
		t.Run(string(provider), func(t *testing.T) {
			snap := uiReviewDetails(uiReviewSnapshot())
			snap.Codex.Speed = speedFixture()
			snap.Grok = runway.GrokState{Refreshing: true}
			snap.Claude = runway.ClaudeState{Refreshing: true}
			snap.Refresh.Sections = []runway.Section{runway.SectionSpeed, runway.SectionAPICost}
			a, tt := newPanel(t, func(p *store.Preferences) {
				p.SelectedProvider = provider
				loadingTestModules(p, store.ModuleSpeedAnalysis, store.ModuleAPICost)
			}, snap, PanelActions{})
			// Local providers must show loading without a full Codex refresh.
			if provider != store.ProviderCodex && !hasExact(tt, tx(a.L(), xPanelLoading)) {
				t.Fatalf("local scan has no loading feedback: %q", tt.Texts())
			}
			snap.Grok.Speed, snap.Claude.Speed = snap.Codex.Speed, snap.Codex.Speed
			snap.Grok.Cost, snap.Claude.Cost = snap.Codex.DisplayedCost, snap.Codex.DisplayedCost
			a.snap = snap
			tt.Frame()
			if hasExact(tt, tx(a.L(), xPanelLoading)) || !tt.HasText("gpt-5.5 high") || !tt.HasText("18.42") {
				t.Fatalf("refresh hid existing data: %q", tt.Texts())
			}
			snap = loadingFailureState(snap)
			a.snap = snap
			tt.Frame()
			if hasExact(tt, tx(a.L(), xPanelLoading)) || !tt.HasText("sessions unreadable") {
				t.Fatalf("failed scan did not exit loading: %q", tt.Texts())
			}
		})
	}
}

func loadingFailureState(snap runway.Snapshot) runway.Snapshot {
	snap.Refresh = runway.RefreshView{}
	snap.Grok.Refreshing, snap.Claude.Refreshing = false, false
	snap.Codex.Speed, snap.Grok.Speed, snap.Claude.Speed = nil, nil, nil
	snap.Codex.SpeedError = "sessions unreadable"
	snap.Grok.SpeedError, snap.Claude.SpeedError = snap.Codex.SpeedError, snap.Codex.SpeedError
	return snap
}

func TestPanelCodexRefreshDoesNotAnimateOtherProviders(t *testing.T) {
	for _, provider := range []store.Provider{store.ProviderGrok, store.ProviderClaude} {
		snap := runway.Snapshot{Refresh: runway.RefreshView{Sections: []runway.Section{runway.SectionSpeed, runway.SectionAPICost}}}
		a, tt := newPanel(t, func(p *store.Preferences) {
			p.SelectedProvider = provider
			loadingTestModules(p, store.ModuleSpeedAnalysis, store.ModuleAPICost)
		}, snap, PanelActions{})
		if tt.HasText(tx(a.L(), xPanelLoading)) {
			t.Fatalf("%s inherited Codex loading: %q", provider, tt.Texts())
		}
	}
}

func TestExportPanelLoadingReview(t *testing.T) {
	dir := os.Getenv("QUOSPARR_LOADING_QA_DIR")
	if dir == "" {
		t.Skip("set QUOSPARR_LOADING_QA_DIR to export loading review PNGs")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, dark := range []bool{false, true} {
		_, tt := newPanel(t, func(p *store.Preferences) {
			p.Language = store.LanguageSimplifiedChinese
			p.SelectedProvider = store.ProviderClaude
			loadingTestModules(p, store.ModuleTokenUsage, store.ModuleSpeedAnalysis, store.ModuleResetCredits, store.ModuleAPICost)
		}, runway.Snapshot{Refresh: runway.RefreshView{All: true}}, PanelActions{})
		tt.SetSize(panelWidth, 650)
		tt.SetDark(dark)
		name := "loading-light.png"
		if dark {
			name = "loading-dark.png"
		}
		exportReviewPNG(t, tt, filepath.Join(dir, name))
	}
}

func TestLoadingIndicatorAnimates(t *testing.T) {
	// The native spinner paints on elapsed frame time without re-running the
	// view; the two rendered frames must differ while the loading text stays.
	l := l10n.New(l10n.English)
	tt := ui.NewTester(func(c *ui.Context) { sectionEmpty(c, l, true) }, 200, 40)
	if box, ok := tt.Find(tx(l, xPanelLoading)); !ok || box.W != 14 {
		t.Fatal("loading indicator is missing")
	}
	before := bytes.Clone(tt.Image().Pix)
	for range 4 {
		tt.Advance(90 * time.Millisecond)
		if !bytes.Equal(before, tt.Image().Pix) {
			return
		}
	}
	t.Fatal("loading spinner did not animate across rendered frames")
}
