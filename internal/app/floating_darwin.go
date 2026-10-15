package app

import (
	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"
)

// NSWindowCollectionBehavior bits.
const (
	nsCanJoinAllSpaces    = 1 << 0
	nsStationary          = 1 << 4
	nsIgnoresCycle        = 1 << 6
	nsFullScreenAuxiliary = 1 << 8
)

// floatingTransparent: the window shows the desktop around its rounded card.
func floatingTransparent() bool { return true }

// setupFloatingWindow keeps the window on every Space, out of Mission
// Control and the window cycle, and, when asked, over full-screen apps,
// whose Spaces it otherwise stays out of.
func setupFloatingWindow(win *mygo.Window, overFullScreen bool) {
	w := objc.ID(win.NativeHandle())
	if w == 0 {
		return
	}
	behavior := uint64(nsCanJoinAllSpaces | nsStationary | nsIgnoresCycle)
	if overFullScreen {
		behavior |= nsFullScreenAuxiliary
	}
	w.Send(objc.RegisterName("setCollectionBehavior:"), behavior)
}

// watchFullScreen: full-screen apps have Spaces of their own on macOS, which
// the window does not join unless set up for it.
func watchFullScreen(func(bool)) func() { return func() {} }
