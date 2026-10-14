package app

import (
	"math"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/store"
)

type panelResizeState struct {
	start, pointerY, delta, height float64
	dragging, moved                bool
	x, y, available                int
}

func panelResizeHandle(c *ui.Context, a *App) {
	s := &a.panelResize
	grip := ui.Box(c).Absolute().Left(0).Right(0).Bottom(0).Height(10).Cursor(ui.CursorResizeNS).Role(ui.RoleNone).Label("panel-resize-handle")
	_, _, held := grip.Dragged()
	_, pointerY, _ := c.Root().PointerPosition()
	if held {
		if !s.dragging {
			_, height := c.Size()
			*s = panelResizeState{start: float64(height), pointerY: float64(pointerY), dragging: true}
			if a.panel != nil {
				bounds := a.panel.Bounds()
				s.x, s.y = bounds.X, bounds.Y
				area := mygo.Screen.DisplayMatching(bounds).WorkArea
				chrome := max(0, bounds.Height-int(height))
				s.available = max(1, area.Y+area.Height-bounds.Y-chrome)
			}
		}
		delta := float64(pointerY) - s.pointerY
		if delta != s.delta {
			s.delta = delta
			s.moved = true
			a.resizePanelHeight(s.start+s.delta, false)
		}
	} else if s.dragging {
		s.dragging = false
		// A native frame can coalesce the last move with mouse-up, when
		// Dragged no longer reports a delta; keep that final pointer position.
		s.delta = float64(pointerY) - s.pointerY
		if s.moved || s.delta != 0 {
			a.resizePanelHeight(s.start+s.delta, true)
		}
	}
	color := ui.Transparent
	if grip.Hovered() || held {
		color = c.Theme().TextMuted.Alpha(0.55)
	}
	grip.AlignItems(ui.Center).Justify(ui.Center).Children(func() { ui.Box(c).Size(36, 3).Radius(2).Background(color) })
}

func (a *App) resizePanelHeight(proposed float64, persist bool) {
	s := &a.panelResize
	height := clampPanelHeight(proposed, s.available)
	s.height = height
	if panel := a.panel; panel != nil {
		x, y := s.x, s.y
		// Defer native geometry until this render pass ends; preserve the top
		// edge and the menu-bar anchor while only the bottom edge moves.
		mygo.App.WhenReady(func() {
			if a.panel != panel || panel.IsDestroyed() || a.panelResize.height != height {
				return
			}
			panel.SetContentSize(panelWidth, int(height))
			panel.SetPosition(x, y)
		})
	}
	if persist {
		a.settings.SetMainPanelHeight(height)
	}
}

func clampPanelHeight(proposed float64, available int) float64 {
	upper := float64(store.MaxMainPanelHeight)
	if available > 0 {
		upper = min(upper, max(float64(store.MinMainPanelHeight), float64(available-16)))
	}
	if math.IsNaN(proposed) || math.IsInf(proposed, 0) {
		proposed = store.DefaultMainPanelHeight
	}
	return math.Round(min(upper, max(float64(store.MinMainPanelHeight), proposed)))
}
