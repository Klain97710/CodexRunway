package app

import (
	"slices"
	"testing"

	"github.com/licoy/quosparr/internal/export"
	"github.com/licoy/quosparr/internal/runway"
	"github.com/licoy/quosparr/internal/store"
)

func TestWidgetDeepLinksSelectProviderAndSection(t *testing.T) {
	for _, tc := range []struct {
		scope    export.ProviderScope
		section  export.Section
		provider store.Provider
		module   store.Module
		refresh  runway.Section
	}{
		{export.ScopeGrok, export.SectionCost, store.ProviderGrok, store.ModuleAPICost, runway.SectionAPICost},
		{export.ScopeBoth, export.SectionTokens, store.ProviderGrok, "", runway.SectionTokenHeatmap},
		{export.ScopeCodex, export.SectionQuota, store.ProviderCodex, "", runway.SectionQuota},
		{export.ScopeGrok, export.SectionResetToday, store.ProviderCodex, "", runway.SectionResetToday},
	} {
		t.Run(string(tc.scope)+string(tc.section), func(t *testing.T) {
			var got runway.Section
			a, panel := newPanel(t, func(p *store.Preferences) { p.SelectedProvider = store.ProviderGrok }, panelFixture(), PanelActions{RefreshSection: func(s runway.Section) { got = s }})
			if !a.OpenWidgetURL((export.DeepLink{Provider: tc.scope, Section: tc.section}).URL()) {
				t.Fatal("valid link rejected")
			}
			panel.Frame()
			if a.settings.Prefs().SelectedProvider != tc.provider || a.panelState.provider != tc.provider || a.panelState.detail != tc.module || got != tc.refresh {
				t.Fatalf("wrong widget target: provider=%s, detail=%s, refresh=%s", a.panelState.provider, a.panelState.detail, got)
			}
		})
	}
}

func TestWidgetDeepLinkRejectsInvalidAndOverviewRefreshes(t *testing.T) {
	count := 0
	a, _ := newPanel(t, nil, panelFixture(), PanelActions{Refresh: func() { count++ }})
	if a.OpenWidgetURL("codex-runway://widget?provider=grok&section=unknown") {
		t.Fatal("invalid link accepted")
	}
	if a.settings.Prefs().SelectedProvider != store.ProviderCodex || count != 0 {
		t.Fatal("invalid link changed state")
	}
	if !a.OpenWidgetURL((export.DeepLink{Provider: export.ScopeBoth, Section: export.SectionOverview}).URL()) || count != 1 || a.panelState.detail != "" {
		t.Fatal("overview did not open and refresh")
	}
}

// A widget may name a provider the panel hides. The panel stays on its own
// provider, opens the overview, and leaves the saved list alone.
func TestWidgetDeepLinkHiddenProviderOpensOverview(t *testing.T) {
	section := runway.Section("untouched")
	refreshed := 0
	shown := []store.Provider{store.ProviderClaude, store.ProviderGrok}
	a, panel := newPanel(t, func(p *store.Preferences) {
		p.SelectedProvider = store.ProviderGrok
		p.PanelProviders = shown
	}, panelFixture(), PanelActions{
		Refresh:        func() { refreshed++ },
		RefreshSection: func(s runway.Section) { section = s },
	})
	link := export.DeepLink{Provider: export.ScopeCodex, Section: export.SectionQuota}
	if !a.OpenWidgetURL(link.URL()) {
		t.Fatal("link rejected")
	}
	panel.Frame()
	if a.settings.Prefs().SelectedProvider != store.ProviderGrok || a.panelState.provider != store.ProviderGrok {
		t.Fatalf("provider became %s", a.panelState.provider)
	}
	if a.panelState.detail != "" || refreshed != 1 || section != "untouched" {
		t.Fatalf("detail=%s refreshed=%d section=%s", a.panelState.detail, refreshed, section)
	}
	if !slices.Equal(a.settings.Prefs().PanelProviders, shown) {
		t.Fatalf("saved list changed to %v", a.settings.Prefs().PanelProviders)
	}
}
