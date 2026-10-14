//go:build darwin

package app

import "testing"

func TestPanelCloseDisposesOutsideClickMonitors(t *testing.T) {
	a, _ := newPanel(t, nil, panelFixture(), PanelActions{})
	stops := 0
	a.popover.stopDismiss = func() { stops++ }
	a.panelClosed()
	a.panelClosed()
	if stops != 1 || a.popover.stopDismiss != nil {
		t.Fatalf("panel close did not dispose event monitors exactly once: %d", stops)
	}
}
