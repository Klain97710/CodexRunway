package app

import (
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
	if a.OpenWidgetURL("quosparr://widget?provider=grok&section=unknown") {
		t.Fatal("invalid link accepted")
	}
	if a.settings.Prefs().SelectedProvider != store.ProviderCodex || count != 0 {
		t.Fatal("invalid link changed state")
	}
	if !a.OpenWidgetURL((export.DeepLink{Provider: export.ScopeBoth, Section: export.SectionOverview}).URL()) || count != 1 || a.panelState.detail != "" {
		t.Fatal("overview did not open and refresh")
	}
}
