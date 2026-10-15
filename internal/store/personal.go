package store

import "github.com/licoy/quosparr/internal/personal"

// ApplyFeatures preserves legacy decoding and changes only provider choices.
func (p *Preferences) ApplyFeatures(features personal.Features) {
	if !features.Enabled(string(p.SelectedProvider)) {
		p.SelectedProvider = ProviderCodex
	}
	if !features.Grok && !features.Claude {
		p.StatusBarProviderScope = ProviderScopeSelected
	}
	if features.Grok && features.Claude {
		return
	}
	filter := func(providers []Provider) []Provider {
		var result []Provider
		for _, provider := range providers {
			if features.Enabled(string(provider)) {
				result = append(result, provider)
			}
		}
		if len(result) == 0 {
			return []Provider{ProviderCodex}
		}
		return result
	}
	p.PanelProviders = filter(p.PanelSelection())
	if len(p.StatusBarProviders) > 0 {
		p.StatusBarProviders = filter(p.StatusBarProviders)
	}
	if len(p.FloatingWindow.Providers) > 0 {
		p.FloatingWindow.Providers = filter(p.FloatingWindow.Providers)
	}
}
