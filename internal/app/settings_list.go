package app

import "github.com/egoist/mygo/ui"

// The managed lists of the settings window (sessions, skills and MCP
// servers) share one layout: fixed controls over a list that scrolls on its
// own, rows that light up whole under the pointer, and a selection bar
// whose box lines up with the rows' boxes.

// listInset and listRowPadX place the rows inside the list card, so the
// selection bar's box lines up with theirs.
const (
	listInset   = 4
	listRowPadX = 10
	// listControlHeight is the height every control of a filter bar shares.
	listControlHeight = 28
)

// splitPage lays out a page whose header stays put while its body scrolls,
// its scroll bar at the window's edge. key names the scroll position.
func splitPage(c *ui.Context, key string, header, body func()) {
	ui.Column(c).Padding(0, 24).Gap(14).Children(header)
	ui.Scroll(c.Key(key)).Grow(1).Basis(0).Padding(0, 24, 24, 24).Gap(14).Children(body)
}

// listPanel is the card holding a list's rows, with no rules between them.
func listPanel(c *ui.Context) ui.Element {
	return Panel(c).Padding(listInset).Gap(2)
}

// listRow is one row of a listPanel, tinted while checked and lit under the
// pointer. The caller adds its children.
func listRow(c *ui.Context, key string, checked bool) ui.Element {
	t := c.Theme()
	row := ui.Row(c.Key(key)).Gap(10).Padding(7, listRowPadX).Radius(8).AlignItems(ui.Center)
	switch {
	case checked:
		row.Background(t.Accent.Alpha(0.10))
	case row.Hovered():
		row.Background(pal(t).Hover)
	}
	return row
}

// listBar is the row over a listPanel, inset so its leading box lines up
// with the rows' boxes.
func listBar(c *ui.Context) ui.Element {
	return ui.Row(c).Gap(10).Padding(0, listInset+1+listRowPadX).AlignItems(ui.Center)
}

// mixedCheckbox draws a check box like ui.Checkbox's that can also be
// mixed, as one over a list whose boxes differ. It reports a click.
func mixedCheckbox(c *ui.Context, key string, all, some bool, label string, disabled bool) bool {
	t := c.Theme()
	on := all
	row := ui.CheckboxBase(c.Key(key), &on).Label(label).Tooltip(label).Disabled(disabled).Cursor(ui.CursorPointer)
	if some && !all {
		row.Mixed()
	}
	row.Children(func() {
		box := ui.Box(c).Size(t.Space(4), t.Space(4)).Radius(t.Space(1)).Shrink(0).AlignItems(ui.Center).Justify(ui.Center)
		switch {
		case all || some:
			fill := t.Accent
			if disabled {
				fill = fill.Alpha(0.5)
			}
			box.Background(fill)
		case row.Hovered() && !disabled:
			box.Background(t.Background).Border(1, t.Accent)
		default:
			box.Background(t.Background).Border(1, t.Border.Mix(t.Text, 0.25))
		}
		box.Children(func() {
			if all {
				ui.Icon(c, icTick).Size(12, 12).TextColor(t.AccentText)
			} else if some {
				ui.Box(c).Size(8, 2).Radius(1).Background(t.AccentText)
			}
		})
	})
	return row.Changed()
}

// listMenuButton is a filter bar button that opens a menu: its label, or an
// icon alone, then a down chevron.
func listMenuButton(c *ui.Context, key string, icon *ui.SVG, label, name string, disabled bool, build func(m *ui.Menu)) {
	t := c.Theme()
	b := ui.ButtonBase(c.Key(key)).Label(name).Tooltip(name).Disabled(disabled).Height(listControlHeight).Padding(0, 8).Gap(4).
		Radius(6).AlignItems(ui.Center).Cursor(ui.CursorPointer)
	bg, col := t.Text.Alpha(0.06), t.Text
	switch {
	case disabled:
		col = t.TextMuted
	case b.Pressed():
		bg = t.Text.Alpha(0.14)
	case b.Hovered():
		bg = t.Text.Alpha(0.10)
	}
	b.Background(bg).Children(func() {
		if icon != nil {
			ui.Icon(c, icon).Size(13, 13).TextColor(col)
		}
		if label != "" {
			ui.Text(c, label).FontSize(12).FontWeight(500).TextColor(col).SingleLine()
		}
		ui.Icon(c, icChevDown).Size(11, 11).TextColor(t.TextMuted)
	})
	b.Menu(build)
}

// listNotice is the result of the last action over a list, green or red.
// A close button clears it when onClose is set.
func listNotice(c *ui.Context, key, text string, failed bool, closeLabel string, onClose func()) {
	if text == "" {
		return
	}
	t := c.Theme()
	col, icon := t.Success, icCheckCircle
	if failed {
		col, icon = t.Danger, icWarnCircle
	}
	ui.Row(c).Gap(8).Padding(8, 10).Radius(8).Background(col.Alpha(0.10)).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, icon).Size(13, 13).TextColor(col)
		ui.Text(c, text).FontSize(12).TextColor(col).Grow(1).Basis(0)
		if onClose == nil {
			return
		}
		b := ui.ButtonBase(c.Key(key)).Label(closeLabel).Size(18, 18).Radius(4).AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
		b.Children(func() { ui.Icon(c, icClose).Size(10, 10).TextColor(col) })
		if b.Clicked() {
			onClose()
		}
	})
}
