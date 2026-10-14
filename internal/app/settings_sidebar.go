package app

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

func (u *settingsUI) shell(c *ui.Context, content func(*ui.Context)) {
	t := c.Theme()
	ui.Row(c).Fill().AlignItems(ui.Stretch).Background(t.Background).Children(func() {
		ui.Column(c).Width(settingsSidebarWidth).Background(pal(t).Sidebar).BorderWidth(0, 1, 0, 0).BorderColor(pal(t).Line).Children(func() {
			u.sidebar(c)
		})
		ui.Column(c).Grow(1).Basis(0).Children(func() {
			u.pageHeader(c)
			content(c)
		})
	})
}

func (u *settingsUI) sidebar(c *ui.Context) {
	t := c.Theme()
	bar := c.TitleBar()
	top := float32(14)
	if bar.Left > 0 && bar.Height > 0 {
		top = max(bar.Height, 52)
	}
	ui.Row(c).Height(top).DragWindow()
	ui.Scroll(c.Key("settings-sidebar")).Grow(1).Padding(0, 10, 10, 10).Gap(2).Children(func() {
		for _, tab := range settingsTabs {
			if u.sidebarItem(c, tab) {
				u.tab = tab
			}
		}
	})
	ui.Row(c).Padding(14, 18).Gap(10).AlignItems(ui.Center).BorderWidth(1, 0, 0, 0).BorderColor(pal(t).Line).Children(func() {
		ui.Image(c, appIconBitmap()).Size(32, 32)
		ui.Column(c).Gap(2).Children(func() {
			ui.Text(c, "CodexRunway").FontSize(13).FontWeight(600).SingleLine()
			ui.Text(c, u.opts.Version).FontSize(11).TextColor(t.TextMuted).SingleLine()
		})
	})
}

func (u *settingsUI) sidebarItem(c *ui.Context, tab settingsTab) bool {
	t := c.Theme()
	title, selected := u.text(tab.key()), u.tab == tab
	b := ui.ButtonBase(c.Key(fmt.Sprintf("tab-%d", tab))).Label(title).Padding(5, 8).Gap(10).Radius(8).AlignItems(ui.Center)
	b.Role(ui.RoleTab).Selected(selected)
	col := t.Text
	if selected {
		b.Background(t.Accent)
		col = t.AccentText
	} else if b.Hovered() {
		b.Background(pal(t).Hover)
	}
	b.Children(func() {
		if selected {
			ui.Row(c).Size(24, 24).Radius(7).Background(ui.RGBA(255, 255, 255, 0.22)).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, tab.icon()).Size(14, 14).TextColor(t.AccentText)
			})
		} else {
			Tile(c, tab.icon(), tab.tint(), 24, true)
		}
		ui.Text(c, title).FontSize(14).TextColor(col).SingleLine().Grow(1)
	})
	return b.Clicked()
}

func (u *settingsUI) pageHeader(c *ui.Context) {
	bar := c.TitleBar()
	ui.Row(c).Height(max(bar.Height, 52)).Padding(0, bar.Right+16, 0, 24).AlignItems(ui.Center).DragWindow().Children(func() {
		ui.Text(c, u.text(u.tab.key())).FontSize(15).FontWeight(700).SingleLine().Grow(1).Basis(0)
	})
}
