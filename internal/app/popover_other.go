//go:build !darwin

package app

type panelPopoverState struct{}

func observePanelPopover(*App) func()   { return func() {} }
func (a *App) matchPanelAppearance()    {}
func releasePanelPopover(*App)          {}
func (a *App) watchPanelDismiss() error { return nil }
