package app

import (
	"slices"

	"github.com/egoist/mygo/ui"
)

// rowFunc adds one row to a settings group: a label with a muted
// description at the left, its control at the right.
type rowFunc func(label, detail string, control func())

// settingsGroup is a section of settings, one per row of its box.
func settingsGroup(c *ui.Context, title, subtitle string, header func(), rows func(row rowFunc)) {
	t := c.Theme()
	settingsSection(c, title, subtitle, header, func() {
		first := true
		rows(func(label, detail string, control func()) {
			if !first {
				ui.Box(c).Height(1).Background(pal(t).Line)
			}
			first = false
			ui.Row(c).Gap(16).Padding(8, 0).MinHeight(40).AlignItems(ui.Center).Children(func() {
				ui.Column(c).Grow(1).Basis(0).Gap(2).Children(func() {
					if label != "" {
						ui.Text(c, label).FontSize(13)
					}
					if detail != "" {
						ui.Text(c, detail).FontSize(11).TextColor(t.TextMuted)
					}
				})
				if control != nil {
					control()
				}
			})
		})
	})
}

// settingsSection is a box of settings under a plain title, as System
// Settings sets its sections apart; the first one of a page has none.
func settingsSection(c *ui.Context, title, subtitle string, header func(), body func()) {
	t := c.Theme()
	ui.Column(c).Gap(8).Children(func() {
		if title != "" || header != nil {
			ui.Row(c).Gap(10).Padding(0, 12).AlignItems(ui.End).Children(func() {
				ui.Column(c).Grow(1).Basis(0).Gap(2).Children(func() {
					if title != "" {
						ui.Text(c, title).FontSize(13).FontWeight(600)
					}
					if subtitle != "" {
						ui.Text(c, subtitle).FontSize(11).TextColor(t.TextMuted)
					}
				})
				if header != nil {
					header()
				}
			})
		}
		Panel(c).Children(body)
	})
}

// toggle is a switch row control. It reports a new value.
func toggle(c *ui.Context, key, label string, on bool, disabled bool) (bool, bool) {
	v := on
	changed := ui.Switch(c.Key(key), &v).Label(label).Disabled(disabled).Changed()
	return v, changed
}

// choose is a pop-up button choosing one of vals, borderless with up and
// down arrows as in System Settings' forms. The current value is offered
// even when it is not among vals (an in-range stored interval the options
// do not list), as the Swift app keeps it.
func choose[T comparable](c *ui.Context, key, label string, cur T, vals []T, name func(T) string, disabled bool) (T, bool) {
	if !slices.Contains(vals, cur) {
		vals = append(slices.Clone(vals), cur)
	}
	t := c.Theme()
	v := cur
	sel := ui.SelectBase(c.Key(key), &v)
	b := sel.Trigger.Label(label).Disabled(disabled).Gap(5).Padding(3, 5, 3, 9).Radius(6).AlignItems(ui.Center)
	if !disabled && (b.Hovered() || sel.Open()) {
		b.Background(pal(t).Hover)
	}
	col := t.Text
	if disabled {
		col = t.TextMuted
	}
	b.Children(func() {
		ui.Text(c, name(cur)).FontSize(13).TextColor(col).SingleLine()
		ui.Icon(c, icUpDown).Size(13, 13).TextColor(t.TextMuted)
	})
	sel.Popup(func(panel ui.Element) {
		popupMenu(c, panel)
		for _, o := range vals {
			item := sel.Item(o).Padding(4, 12, 4, 6).Gap(4).Radius(5).AlignItems(ui.Center)
			fg := t.Text
			if item.Highlighted() {
				item.Background(t.Accent)
				fg = t.AccentText
			}
			item.Children(func() {
				checkmark(c, o == cur, fg)
				ui.Text(c, name(o)).FontSize(13).TextColor(fg).SingleLine()
			})
		}
	})
	if b.Changed() && v != cur {
		return v, true
	}
	return cur, false
}

// popupMenu styles a popup as a menu: a raised, rounded panel.
func popupMenu(c *ui.Context, panel ui.Element) {
	t := c.Theme()
	panel.Margin(4, 0, 0, 0).Padding(5).Gap(1).MinWidth(140).Radius(8).
		Background(pal(t).Inset).BorderWidth(1).BorderColor(t.Border).
		Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, 0.18))
}

// checkmark marks the chosen item of a menu, or holds its room.
func checkmark(c *ui.Context, on bool, col ui.Color) {
	box := ui.Box(c).Size(14, 14).Shrink(0)
	if !on {
		return
	}
	box.Draw(func(p *ui.Painter, r ui.Rect) {
		var path ui.Path
		path.MoveTo(r.X+r.W*0.18, r.Y+r.H*0.52).LineTo(r.X+r.W*0.42, r.Y+r.H*0.76).LineTo(r.X+r.W*0.84, r.Y+r.H*0.26)
		p.StrokePath(&path, 1.6, col)
	})
}

// pills is a capsule control choosing one of vals.
func pills[T comparable](c *ui.Context, key string, cur T, vals []T, name func(T) string, disabled bool) (T, bool) {
	labels := make([]string, len(vals))
	for i, v := range vals {
		labels[i] = name(v)
	}
	i := max(0, slices.Index(vals, cur))
	if PillTabs(c.Key(key), &i, disabled, labels...) && vals[i] != cur {
		return vals[i], true
	}
	return cur, false
}
