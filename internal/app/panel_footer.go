package app

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/l10n"
)

// panelFooter is the fixed footer: Settings on the left, Quit on the right.
func panelFooter(c *ui.Context, a *App, l l10n.Localizer) {
	ui.Row(c).Padding(8, 16).AlignItems(ui.Center).Children(func() {
		if footerButton(c, icFilterSliders, l.Text(l10n.KeySettings), l.Text(l10n.KeyOpenControlPanel), false).Clicked() {
			a.OpenSettings()
		}
		ui.Spacer(c).Grow(1)
		if footerButton(c, icQuit, l.Text(l10n.KeyQuit), l.Text(l10n.KeyQuit), true).Clicked() {
			mygo.App.Quit()
		}
	})
}

// footerButton is a quiet borderless button; the destructive role turns red
// on hover only.
func footerButton(c *ui.Context, icon *ui.SVG, title, help string, destructive bool) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Label(title).Tooltip(help).Padding(5, 9).Gap(5).Radius(radiusRow).AlignItems(ui.Center).Cursor(ui.CursorPointer)
	ink := inkSecondary(t)
	if b.Hovered() {
		ink = t.Text
		b.Background(hoverNeutral(t))
		if destructive {
			ink = pick(!t.Dark, ui.RGBA(191, 31, 38, 1), sys(t, sysRed))
			b.Background(sys(t, sysRed).Alpha(0.10))
		}
	}
	return b.Children(func() {
		ui.Icon(c, icon).Size(11, 11).TextColor(ink)
		ui.Text(c, title).FontSize(fsCallout).FontWeight(500).TextColor(ink).SingleLine()
	})
}
