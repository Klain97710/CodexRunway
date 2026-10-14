package store

import "github.com/licoy/quosparr/internal/personal"

// ApplyFeatures preserves legacy decoding and changes only provider choices.
func (p *Preferences) ApplyFeatures(features personal.Features) {
	if !features.Grok {
		p.SelectedProvider = ProviderCodex
		p.StatusBarProviderScope = ProviderScopeSelected
	}
}
