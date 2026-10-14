package app

import (
	"slices"

	"github.com/egoist/mygo/ui"
)

// rowFunc adds one row to a settings group: a label with a muted
// description at the left, its control at the right.
type rowFunc func(label, detail string, control func())

// settingsGroup is a card of settings, one per row.
func settingsGroup(c *ui.Context, icon *ui.SVG, k Tint, title, subtitle string, header func(), rows func(row rowFunc)) {
	t := c.Theme()
	Panel(c).Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Padding(0, 0, 8, 0).Children(func() {
			Tile(c, icon, k, 30, false)
			ui.Column(c).Grow(1).Basis(0).Gap(1).Children(func() {
				ui.Text(c, title).FontSize(17).FontWeight(600).TextColor(k.Text(t))
				if subtitle != "" {
					ui.Text(c, subtitle).FontSize(11).TextColor(t.TextMuted)
				}
			})
			if header != nil {
				header()
			}
		})
		first := true
		rows(func(label, detail string, control func()) {
			if !first {
				ui.Box(c).Height(1).Background(pal(t).Line)
			}
			first = false
			ui.Row(c).Gap(16).Padding(9, 0).AlignItems(ui.Center).Children(func() {
				ui.Column(c).Grow(1).Basis(0).Gap(2).Children(func() {
					if label != "" {
						ui.Text(c, label).FontSize(13).FontWeight(500)
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

// toggle is a switch row control. It reports a new value.
func toggle(c *ui.Context, key, label string, on bool, disabled bool) (bool, bool) {
	v := on
	changed := ui.Switch(c.Key(key), &v).Label(label).Disabled(disabled).Changed()
	return v, changed
}

// choose is a drop-down choosing one of vals. The current value is offered
// even when it is not among vals (an in-range stored interval the options
// do not list), as the Swift app keeps it.
func choose[T comparable](c *ui.Context, key, label string, cur T, vals []T, name func(T) string, disabled bool) (T, bool) {
	if !slices.Contains(vals, cur) {
		vals = append(slices.Clone(vals), cur)
	}
	labels := make([]string, len(vals))
	sel := ""
	for i, v := range vals {
		labels[i] = name(v)
		if v == cur {
			sel = labels[i]
		}
	}
	if ui.Select(c.Key(key), &sel, labels).Label(label).Width(190).Disabled(disabled).Changed() {
		if i := slices.Index(labels, sel); i >= 0 {
			return vals[i], vals[i] != cur
		}
	}
	return cur, false
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
