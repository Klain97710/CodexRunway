package app

import (
	"github.com/licoy/quosparr/internal/export"
	"github.com/licoy/quosparr/internal/runway"
	"github.com/licoy/quosparr/internal/store"
)

// OpenWidgetURL handles only the existing widget contract. Both keeps the
// current provider; resetToday always opens Codex, matching the Swift host.
// Call on the UI thread after App.Ready.
func (a *App) OpenWidgetURL(raw string) bool {
	link, ok := export.ParseDeepLink(raw)
	if !ok || a.settings == nil {
		return false
	}
	a.cancelPanelDetails()
	provider := a.settings.Prefs().SelectedProvider
	switch link.Provider {
	case export.ScopeCodex:
		provider = store.ProviderCodex
	case export.ScopeGrok:
		provider = store.ProviderGrok
	}
	if link.Section == export.SectionResetToday {
		provider = store.ProviderCodex
	}
	a.settings.SetSelectedProvider(provider)
	a.panelState.provider = provider
	a.panelState.detail = widgetSectionModule(link.Section)
	acts := a.PanelActions()
	if link.Section == export.SectionOverview {
		if acts.Refresh != nil {
			acts.Refresh()
		}
	} else if acts.RefreshSection != nil {
		acts.RefreshSection(widgetSectionRefresh(link.Section))
	}
	a.OpenPanel()
	a.redrawPanel()
	RefreshWidgetConfigurations()
	return true
}

func widgetSectionModule(s export.Section) store.Module {
	switch s {
	case export.SectionQuota:
		return store.ModuleQuota
	case export.SectionTokens:
		return store.ModuleTokenUsage
	case export.SectionCost:
		return store.ModuleAPICost
	case export.SectionResetToday:
		return store.ModuleRateLimitResetToday
	default:
		return ""
	}
}

func widgetSectionRefresh(s export.Section) runway.Section {
	switch s {
	case export.SectionQuota:
		return runway.SectionQuota
	case export.SectionTokens:
		return runway.SectionTokenHeatmap
	case export.SectionCost:
		return runway.SectionAPICost
	case export.SectionResetToday:
		return runway.SectionResetToday
	default:
		return ""
	}
}
