//go:build darwin

package app

import (
	"fmt"

	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"
)

// These monitors live only as long as the main panel. Local events cover
// other windows in this app, global events cover the desktop and other apps.
func (a *App) watchPanelDismiss() error {
	panel := a.panel
	closeLater := func() {
		// Leave the current event callback before releasing its monitor.
		mygo.App.WhenReady(func() {
			if a.panel == panel {
				a.ClosePanel()
			}
		})
	}
	localBlock := objc.NewBlock(func(_ objc.Block, event objc.ID) objc.ID {
		if a.popover.shouldDismiss(event) {
			closeLater()
		}
		return event
	})
	globalBlock := objc.NewBlock(func(_ objc.Block, event objc.ID) {
		if a.popover.shouldDismiss(event) {
			closeLater()
		}
	})
	events := objc.ID(objc.GetClass("NSEvent"))
	const mouseDown = uint64(1<<1 | 1<<3 | 1<<25) // left, right, other
	local := events.Send(objc.RegisterName("addLocalMonitorForEventsMatchingMask:handler:"), mouseDown, localBlock)
	global := events.Send(objc.RegisterName("addGlobalMonitorForEventsMatchingMask:handler:"), mouseDown, globalBlock)
	stopActive := mygo.App.OnDidResignActive(closeLater)
	a.popover.stopDismiss = func() {
		stopActive()
		for _, monitor := range []objc.ID{local, global} {
			if monitor != 0 {
				events.Send(objc.RegisterName("removeMonitor:"), monitor)
			}
		}
		localBlock.Release()
		globalBlock.Release()
	}
	if local == 0 || global == 0 {
		return fmt.Errorf("main panel outside-click monitor could not be installed")
	}
	return nil
}

func (s panelPopoverState) shouldDismiss(event objc.ID) bool {
	if s.popover == 0 || !objc.Send[bool](s.popover, objc.RegisterName("isShown")) {
		return false
	}
	view := s.popover.Send(objc.RegisterName("contentViewController")).Send(objc.RegisterName("view"))
	window := view.Send(objc.RegisterName("window"))
	if window == 0 {
		return false
	}
	if eventWindowInPanel(event.Send(objc.RegisterName("window")), window) {
		return false
	}
	point := objc.Send[trayPoint](objc.ID(objc.GetClass("NSEvent")), objc.RegisterName("mouseLocation"))
	if containsNativePoint(objc.Send[trayRect](window, objc.RegisterName("frame")), point) {
		return false
	}
	// Its own status item remains owned by TogglePanel, avoiding a close
	// followed by an immediate reopen on the same click.
	if button := trayButton(); button != 0 {
		statusWindow := button.Send(objc.RegisterName("window"))
		if statusWindow != 0 && containsNativePoint(objc.Send[trayRect](statusWindow, objc.RegisterName("frame")), point) {
			return false
		}
	}
	return true
}

func eventWindowInPanel(window, panel objc.ID) bool {
	for window != 0 {
		if window == panel {
			return true
		}
		parent := window.Send(objc.RegisterName("sheetParent"))
		if parent == 0 {
			parent = window.Send(objc.RegisterName("parentWindow"))
		}
		window = parent
	}
	return false
}

func containsNativePoint(r trayRect, p trayPoint) bool {
	return p.X >= r.Origin.X && p.Y >= r.Origin.Y && p.X < r.Origin.X+r.Size.Width && p.Y < r.Origin.Y+r.Size.Height
}
