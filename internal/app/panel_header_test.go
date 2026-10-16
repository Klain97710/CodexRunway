package app

import (
	"testing"

	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/store"
)

// The header switches the floating window, and the panel background where
// the system offers that choice.
func TestPanelHeaderBackgroundAndFloatingToggles(t *testing.T) {
	a, panel := newPanel(t, nil, panelFixture(), PanelActions{})
	l := a.L()
	floating := tx(l, xFloatingEnabled)
	if err := panel.Click(floating); err != nil {
		t.Fatal(err)
	}
	if !a.settings.Prefs().FloatingWindow.Enabled {
		t.Fatal("floating toggle did not turn the window on")
	}
	if err := panel.Click(floating); err != nil {
		t.Fatal(err)
	}
	if a.settings.Prefs().FloatingWindow.Enabled {
		t.Fatal("floating toggle did not turn the window off")
	}

	bgTitle := func(v store.PanelBackground) string {
		return l.Text(l10n.KeyMainPanelBackground) + " · " + l.Text(panelBackgroundKey(v))
	}
	if _, ok := panel.Find(bgTitle(store.PanelBackgroundTranslucent)); ok {
		t.Fatal("background toggle shown where the system has no choice")
	}
	a.settings.sys.PanelBackground = true
	panel.Frame()
	if _, ok := panel.Find(bgTitle(store.PanelBackgroundTranslucent)); !ok {
		t.Fatal("background toggle missing")
	}
	if err := panel.Click(bgTitle(store.PanelBackgroundTranslucent)); err != nil {
		t.Fatal(err)
	}
	if got := a.settings.Prefs().MainPanelBackgroundStyle; got != store.PanelBackgroundOpaque {
		t.Fatalf("background = %q, want opaque", got)
	}
	if err := panel.Click(bgTitle(store.PanelBackgroundOpaque)); err != nil {
		t.Fatal(err)
	}
	if got := a.settings.Prefs().MainPanelBackgroundStyle; got != store.PanelBackgroundTranslucent {
		t.Fatalf("background = %q, want translucent", got)
	}
	a.settings.Flush()
}
