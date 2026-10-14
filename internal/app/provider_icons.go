package app

import (
	_ "embed"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/store"
)

//go:embed assets/providers/codex-color.svg
var codexProviderSVG []byte

//go:embed assets/providers/grok.svg
var grokProviderSVG []byte

var (
	codexProviderIcon = ui.MustParseSVG(codexProviderSVG)
	grokProviderIcon  = ui.MustParseSVG(grokProviderSVG)
)

// ProviderIcon preserves the official logo's colors; monochrome SVGs inherit
// the text color. Unknown providers use the interface's neutral plug icon.
func ProviderIcon(c *ui.Context, provider store.Provider, size float32) ui.Element {
	switch provider {
	case store.ProviderCodex:
		return ui.Image(c, codexProviderIcon).Size(size, size)
	case store.ProviderGrok:
		return ui.Image(c, grokProviderIcon).Size(size, size)
	default:
		return ui.Icon(c, icPlug).Size(size, size).TextColor(c.Theme().TextMuted)
	}
}

// ProviderTabs follows PillTabs' selection and accessibility semantics, with
// the provider logo preceding each caller-localized label. With no labels it
// shows only icons, retaining the localized brand names for accessibility.
func ProviderTabs(c *ui.Context, selected *int, disabled bool, labels ...string) bool {
	t := c.Theme()
	providers := store.Providers()
	if len(labels) == 0 {
		labels = []string{"", ""}
	}
	l := l10n.New(l10n.English)
	names := []string{l.Text(l10n.KeyProviderCodex), l.Text(l10n.KeyProviderGrok)}
	seg := ui.SegmentedBase(c, selected, len(labels))
	seg.Track.Padding(3).Gap(2).Radius(7).Background(t.Text.Alpha(0.06)).Children(func() {
		for i, label := range labels {
			name := label
			if name == "" && i < len(names) {
				name = names[i]
			}
			s := seg.Segment(i).Padding(5, 7).Gap(5).Radius(5).Disabled(disabled).Label(name).Tooltip(name)
			col := t.TextMuted
			if i == *selected {
				if !disabled {
					col = t.Text
				}
				s.Background(pal(t).Inset).Shadow(0, 1, 3, 0, ui.RGBA(0, 0, 0, 0.12))
			}
			s.Children(func() {
				provider := store.Provider("")
				if i < len(providers) {
					provider = providers[i]
				}
				ProviderIcon(c, provider, 16).TextColor(col)
				if label != "" {
					ui.Text(c, label).FontSize(13).FontWeight(500).TextColor(col).SingleLine()
				}
			})
		}
	})
	return seg.Track.Changed()
}
