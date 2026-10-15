package app

import (
	"fmt"
	"math"
	"slices"

	"github.com/egoist/mygo/ui"
)

// themeSwatch is a theme color the settings offer: "#RRGGBB", or empty for
// the brand brown.
type themeSwatch struct {
	color string
	name  textKey
}

// themeSwatches are the brand color and the system's accent colors.
var themeSwatches = []themeSwatch{
	{"", xColorBrown},
	{"#007AFF", xColorBlue},
	{"#953D96", xColorPurple},
	{"#F74F9E", xColorPink},
	{"#E0383E", xColorRed},
	{"#F7821B", xColorOrange},
	{"#FFC600", xColorYellow},
	{"#62BA46", xColorGreen},
	{"#8C8C8C", xColorGraphite},
}

// themeColorName names the stored theme color.
func (u *settingsUI) themeColorName(color string) string {
	l := u.L()
	if i := slices.IndexFunc(themeSwatches, func(w themeSwatch) bool { return w.color == color }); i >= 0 {
		return tx(l, themeSwatches[i].name)
	}
	return tx(l, xColorCustom) + " " + color
}

// themeColorControl is a row of color swatches, as the accent colors of
// System Settings, ending in one that opens a picker for any color.
func (u *settingsUI) themeColorControl(c *ui.Context, ro bool) {
	s, l := u.s, u.L()
	cur := s.prefs.ThemeColor
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		for i, w := range themeSwatches {
			fill := buildTheme(false).Accent
			if w.color != "" {
				fill = ui.Hex(w.color)
			}
			if colorSwatch(c.Key(fmt.Sprint("theme-", i)), tx(l, w.name), fill, false, w.color == cur, ro).Clicked() {
				s.SetThemeColor(w.color)
			}
		}
		custom := !slices.ContainsFunc(themeSwatches, func(w themeSwatch) bool { return w.color == cur })
		anchor := colorSwatch(c.Key("theme-custom"), tx(l, xColorCustom), c.Theme().Accent, true, custom, ro)
		if anchor.Clicked() {
			u.pickingColor = !u.pickingColor
			u.customColor = c.Theme().Accent
		}
		ui.Popover(c, anchor, &u.pickingColor, func() {
			if ui.ColorPicker(c.Key("theme-picker"), &u.customColor).Changed() {
				col := u.customColor
				s.SetThemeColor(fmt.Sprintf("#%02X%02X%02X", col.R, col.G, col.B))
			}
		})
	})
}

// colorSwatch is a round button of a color, or of the hues when rainbow,
// ringed in the color when chosen.
func colorSwatch(c *ui.Context, name string, fill ui.Color, rainbow, chosen, disabled bool) ui.Element {
	b := ui.ButtonBase(c).Label(name).Tooltip(name).Disabled(disabled).Size(26, 26).Radius(13)
	b.Role(ui.RoleRadio).Selected(chosen)
	return b.Draw(func(p *ui.Painter, r ui.Rect) {
		inner := inset(r, 2)
		if chosen {
			p.Stroke(inset(r, 1), fill, r.W/2, 2)
			inner = inset(r, 4.5)
		}
		if rainbow {
			paintHues(p, inner)
		} else {
			p.Fill(inner, fill, inner.W/2)
		}
		p.Stroke(inner, ui.RGBA(0, 0, 0, 0.12), inner.W/2, 1)
	})
}

func inset(r ui.Rect, d float32) ui.Rect {
	return ui.Rect{X: r.X + d, Y: r.Y + d, W: r.W - 2*d, H: r.H - 2*d}
}

// paintHues fills a circle with wedges going round the hues.
func paintHues(p *ui.Painter, r ui.Rect) {
	const n = 24
	cx, cy, rad := r.X+r.W/2, r.Y+r.H/2, r.W
	p.Clip(r, r.W/2, func() {
		for i := range n {
			a0, a1 := 2*math.Pi*float64(i)/n, 2*math.Pi*float64(i+1)/n+0.02
			var path ui.Path
			path.MoveTo(cx, cy).
				LineTo(cx+rad*float32(math.Cos(a0)), cy+rad*float32(math.Sin(a0))).
				LineTo(cx+rad*float32(math.Cos(a1)), cy+rad*float32(math.Sin(a1))).Close()
			p.FillPath(&path, hue(float64(i)/n))
		}
	})
}

// hue is the fully saturated color at h of the color wheel, 0 to 1.
func hue(h float64) ui.Color {
	ch := func(n float64) uint8 {
		k := math.Mod(n+h*6, 6)
		return uint8(255 * (1 - max(0, min(k, 4-k, 1))))
	}
	return ui.RGB(ch(5), ch(3), ch(1))
}
