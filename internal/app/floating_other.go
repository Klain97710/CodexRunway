//go:build !darwin && !windows

package app

import "github.com/egoist/mygo"

// floatingTransparent: a compositing desktop shows through the corners.
func floatingTransparent() bool { return true }

func setupFloatingWindow(*mygo.Window, bool) {}

// watchFullScreen: Linux desktops raise full-screen windows above the
// others themselves.
func watchFullScreen(func(bool)) func() { return func() {} }
