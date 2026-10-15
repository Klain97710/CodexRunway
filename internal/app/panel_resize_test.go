package app

import (
	"math"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/store"
)

func TestPanelResizeDragPersistsOnRelease(t *testing.T) {
	a, tt := newPanel(t, nil, panelFixture(), PanelActions{})
	tt.SetSize(panelWidth, int(store.DefaultMainPanelHeight))
	tt.SetPreferences(ui.Preferences{ReduceMotion: true, TextScale: 1})
	changes := 0
	a.settings.OnChange(func() { changes++ })
	r, ok := tt.Find("panel-resize-handle")
	if !ok {
		t.Fatal("main panel has no bottom resize handle")
	}
	x, y := r.X+r.W/2, r.Y+r.H/2
	tt.Press(x, y)
	tt.Move(x, y+80)
	tt.Move(x, y+120)
	if changes != 0 || a.settings.Prefs().MainPanelHeight != store.DefaultMainPanelHeight {
		t.Fatal("drag persisted before mouse-up")
	}
	tt.Release(x, y+120)
	if changes != 1 || a.settings.Prefs().MainPanelHeight != store.DefaultMainPanelHeight+120 {
		t.Fatalf("release changes=%d height=%v", changes, a.settings.Prefs().MainPanelHeight)
	}
	tt.Frame()
	tt.Frame()
	if changes != 1 {
		t.Fatal("release persisted more than once")
	}
	a.settings.Flush()
}

func TestPanelResizeClickDoesNotPersist(t *testing.T) {
	a, tt := newPanel(t, nil, panelFixture(), PanelActions{})
	tt.SetSize(panelWidth, int(store.DefaultMainPanelHeight))
	changes := 0
	a.settings.OnChange(func() { changes++ })
	if err := tt.Click("panel-resize-handle"); err != nil {
		t.Fatal(err)
	}
	if changes != 0 {
		t.Fatal("click without movement persisted height")
	}
}

func TestPanelResizeDragClampsHeight(t *testing.T) {
	for _, target := range []struct {
		delta  float32
		height float64
	}{{1000, store.MaxMainPanelHeight}, {-1000, store.MinMainPanelHeight}} {
		a, tt := newPanel(t, nil, panelFixture(), PanelActions{})
		tt.SetSize(panelWidth, int(store.DefaultMainPanelHeight))
		r, ok := tt.Find("panel-resize-handle")
		if !ok {
			t.Fatal("resize handle missing")
		}
		x, y := r.X+r.W/2, r.Y+r.H/2
		tt.Press(x, y)
		tt.Move(x, y+target.delta)
		tt.Release(x, y+target.delta)
		if got := a.settings.Prefs().MainPanelHeight; got != target.height {
			t.Fatalf("height %v, want %v", got, target.height)
		}
		a.settings.Flush()
	}
}

func TestPanelHeightClampsToUsableScreen(t *testing.T) {
	for _, tc := range []struct {
		proposed  float64
		available int
		want      float64
	}{{900, 700, 684}, {100, 700, 360}, {900, 300, 360}, {900, 2000, 900}, {math.NaN(), 500, 484}, {math.Inf(1), 0, store.DefaultMainPanelHeight}, {584.6, 0, 585}} {
		if got := clampPanelHeight(tc.proposed, tc.available); got != tc.want {
			t.Fatalf("clamp(%v,%d)=%v, want %v", tc.proposed, tc.available, got, tc.want)
		}
	}
}

func TestPanelOpeningSizeFitsScreenWithoutChangingPreference(t *testing.T) {
	a, _ := newPanel(t, func(p *store.Preferences) { p.MainPanelHeight = 900 }, panelFixture(), PanelActions{})
	for _, tc := range []struct{ screen, want int }{{700, 684}, {1200, 900}, {300, 360}, {0, 900}} {
		width, height := a.panelSizeWithin(tc.screen)
		if width != panelWidth || height != tc.want {
			t.Fatalf("screen %d: panel %dx%d, want %dx%d", tc.screen, width, height, panelWidth, tc.want)
		}
	}
	if a.settings.Prefs().MainPanelHeight != 900 {
		t.Fatal("fitting the panel overwrote the preferred height")
	}
}

func TestPanelResizeCoalescedReleaseIncludesFinalMovement(t *testing.T) {
	testPanelResizeFinalMovement(t, false)
}

func TestPanelResizeReleaseIncludesMovementAfterLastFrame(t *testing.T) {
	testPanelResizeFinalMovement(t, true)
}

func testPanelResizeFinalMovement(t *testing.T, moveBeforeRelease bool) {
	t.Helper()
	a, tt := newPanel(t, nil, panelFixture(), PanelActions{})
	tt.SetSize(panelWidth, int(store.DefaultMainPanelHeight))
	changes := 0
	a.settings.OnChange(func() { changes++ })
	r, ok := tt.Find("panel-resize-handle")
	if !ok {
		t.Fatal("resize handle missing")
	}
	x, y := r.X+r.W/2, r.Y+r.H/2
	tt.Press(x, y)
	if moveBeforeRelease {
		tt.Move(x, y-40)
	}
	tt.Release(x, y-80) // A real native frame can see release before any held move frame.
	if changes != 1 || a.settings.Prefs().MainPanelHeight != store.DefaultMainPanelHeight-80 {
		t.Fatalf("coalesced release changes=%d height=%v, want one save at %v", changes, a.settings.Prefs().MainPanelHeight, store.DefaultMainPanelHeight-80)
	}
	a.settings.Flush()
}

func TestPanelResizeTopEdgeGrowsUpward(t *testing.T) {
	a, tt := newPanel(t, nil, panelFixture(), PanelActions{})
	tt.SetSize(panelWidth, int(store.DefaultMainPanelHeight))
	a.panelResize.top = true
	tt.Frame()
	r, ok := tt.Find("panel-resize-handle")
	if !ok {
		t.Fatal("main panel has no top resize handle")
	}
	if r.Y != 0 {
		t.Fatalf("top handle at y=%v, want 0", r.Y)
	}
	x, y := r.X+r.W/2, r.Y+r.H/2
	tt.Press(x, y)
	tt.Move(x, y-60)
	tt.Release(x, y-100)
	if got := a.settings.Prefs().MainPanelHeight; got != store.DefaultMainPanelHeight+100 {
		t.Fatalf("height %v, want %v", got, store.DefaultMainPanelHeight+100)
	}
	if !a.panelResize.top {
		t.Fatal("drag dropped the top edge")
	}
	a.settings.Flush()
}
