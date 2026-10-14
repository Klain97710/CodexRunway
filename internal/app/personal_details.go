package app

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/personal"
)

// OpenDetailsWindow has its own view state and remains open when the tray
// popover is dismissed. It shares the current snapshot and existing actions.
func (a *App) OpenDetailsWindow() {
	if !mygo.App.IsReady() {
		return
	}
	if a.details != nil && a.details.panel != nil && !a.details.panel.IsDestroyed() {
		a.details.panel.Focus()
		return
	}
	d := &App{settings: a.settings, opts: a.opts, snap: a.Snapshot(), panelActs: a.PanelActions()}
	_, height := a.panelSize()
	d.panel = mygo.NewWindow(mygo.WindowOptions{
		Title: personal.Name + " · " + a.L().Text(l10n.KeyOpenDetailsWindow),
		Width: panelWidth, Height: height, MinWidth: panelWidth, MaxWidth: panelWidth,
		UseContentSize: true, Content: ui.View(d.panelView),
	})
	a.details = d
	d.panel.OnClosed(func() {
		d.cancelPanelDetails()
		d.panel = nil
		if a.details == d {
			a.details = nil
		}
	})
	d.panelOpened()
}

func (a *App) confirmTrayRepair() {
	if a.opts.Actions.RepairIndex == nil {
		return
	}
	l := a.L()
	confirmed, err := confirmDialog(l, l.Text(l10n.KeyRepairConfirmTitle), l.Text(l10n.KeyRepairConfirmMessage), "", l.Text(l10n.KeyRepair))
	if err == nil && confirmed {
		a.opts.Actions.RepairIndex()
	}
}
