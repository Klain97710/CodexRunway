package app

import (
	"context"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/store"
	"github.com/licoy/quosparr/internal/usage"
)

func TestCostControlsRenderBrandAccent(t *testing.T) {
	l := l10n.New(l10n.English)
	acts := PanelActions{QueryCost: func(context.Context, store.Provider, CostQuery) (usage.Summary, error) {
		return usage.Summary{}, nil
	}}
	for _, dark := range []bool{false, true} {
		theme := buildTheme(dark)
		name := "light"
		if dark {
			name = "dark"
		}
		t.Run(name, func(t *testing.T) {
			a := &App{}
			a.panelState.cost.start = time.Now().AddDate(0, 0, -1)
			a.panelState.cost.end = time.Now()
			tabs := ui.NewTester(func(c *ui.Context) {
				applyTheme(c)
				a.costRangeTabs(c, l, acts)
			}, 400, 60)
			tabs.SetDark(dark)
			assertBrandBackground(t, tabs, l.Text(l10n.KeyCurrentCycle), theme)

			cta := ui.NewTester(func(c *ui.Context) {
				applyTheme(c)
				a.customCostControls(c, l, acts)
			}, 400, 70)
			cta.SetDark(dark)
			assertBrandBackground(t, cta, l.Text(l10n.KeyCalculate), theme)
		})
	}
}

func assertBrandBackground(t *testing.T, tt *ui.Tester, label string, theme *ui.Theme) {
	t.Helper()
	if !tt.HasText(label) {
		t.Fatalf("control %q not found", label)
	}
	brand, blue := 0, 0
	img := tt.Image()
	semantic := sys(theme, sysBlue)
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			px := img.RGBAAt(x, y)
			if px.R == theme.Accent.R && px.G == theme.Accent.G && px.B == theme.Accent.B {
				brand++
			}
			if px.R == semantic.R && px.G == semantic.G && px.B == semantic.B {
				blue++
			}
		}
	}
	if brand < 20 || blue != 0 {
		t.Fatalf("control %q brand pixels=%d, old blue pixels=%d", label, brand, blue)
	}
}
