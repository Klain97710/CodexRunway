package app

import (
	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/store"
)

// Tint is one of the system colors that tell the parts of the settings
// window apart.
type Tint int

// Tints.
const (
	TintBlue Tint = iota
	TintGreen
	TintRed
	TintOrange
	TintTeal
	TintPurple
	TintIndigo
	TintPink
	TintGray
)

var tintHex = [...][2]string{
	TintBlue:   {"#007AFF", "#0A84FF"},
	TintGreen:  {"#28B14C", "#30D158"},
	TintRed:    {"#FF3B30", "#FF453A"},
	TintOrange: {"#F59100", "#FF9F0A"},
	TintTeal:   {"#14A3BE", "#40C8E0"},
	TintPurple: {"#AF52DE", "#BF5AF2"},
	TintIndigo: {"#5856D6", "#5E5CE6"},
	TintPink:   {"#FF2D55", "#FF375F"},
	TintGray:   {"#8E8E93", "#98989D"},
}

var tintColors [len(tintHex)][2]ui.Color

func init() {
	for i, h := range tintHex {
		tintColors[i] = [2]ui.Color{ui.Hex(h[0]), ui.Hex(h[1])}
	}
}

// C is the tint in the theme's appearance.
func (k Tint) C(t *ui.Theme) ui.Color {
	if t.Dark {
		return tintColors[k][1]
	}
	return tintColors[k][0]
}

// Text is the tint for text, darker on light backgrounds to stay legible.
func (k Tint) Text(t *ui.Theme) ui.Color {
	if t.Dark {
		return tintColors[k][1]
	}
	return tintColors[k][0].Mix(ui.RGB(0, 0, 0), 0.12)
}

// palette holds the surfaces the theme has no names for.
type palette struct {
	// Card is a card on the window's background, Inset what sits on a card,
	// Line a hairline, Hover a hovered row.
	Card, Inset, Line, Hover, Sidebar ui.Color
}

var palettes = [2]palette{
	{
		Card:    ui.Hex("#F4F4F6"),
		Sidebar: ui.Hex("#F5F5F7"),
		Inset:   ui.Hex("#FFFFFF"),
		Line:    ui.RGBA(0, 0, 0, 0.07),
		Hover:   ui.RGBA(0, 0, 0, 0.05),
	},
	{
		Card:    ui.Hex("#262628"),
		Sidebar: ui.Hex("#202022"),
		Inset:   ui.Hex("#303033"),
		Line:    ui.RGBA(255, 255, 255, 0.08),
		Hover:   ui.RGBA(255, 255, 255, 0.06),
	},
}

func pal(t *ui.Theme) palette {
	if t.Dark {
		return palettes[1]
	}
	return palettes[0]
}

// brandInk keeps the brown accent readable when it is used as foreground on
// dark cards. Filled controls continue to use the exact theme accent.
func brandInk(t *ui.Theme, color ui.Color) ui.Color {
	if t.Dark {
		return color.Mix(ui.RGB(255, 255, 255), 0.4)
	}
	return color
}

var themes [2]*ui.Theme

// applyTheme makes the frame use the app's theme in the system's (or the
// chosen) appearance. Views call it first.
func applyTheme(c *ui.Context) *ui.Theme {
	i := 0
	if c.Theme().Dark {
		i = 1
	}
	if themes[i] == nil {
		themes[i] = buildTheme(i == 1)
	}
	c.SetTheme(themes[i])
	return themes[i]
}

func buildTheme(dark bool) *ui.Theme {
	if dark {
		t := ui.DarkTheme()
		t.Background = ui.Hex("#18181A")
		t.Surface = ui.Hex("#333336")
		t.SurfaceHover = ui.Hex("#3C3C3F")
		t.SurfacePressed = ui.Hex("#47474A")
		t.Border = ui.Hex("#3A3A3D")
		t.Text = ui.Hex("#F5F5F7")
		t.TextMuted = ui.Hex("#98989D")
		t.Accent = ui.Hex("#B3582D")
		t.AccentHover = ui.Hex("#B95C31")
		t.AccentPressed = ui.Hex("#984925")
		t.Danger = ui.Hex("#FF453A")
		t.Warning = ui.Hex("#FF9F0A")
		t.Success = ui.Hex("#30D158")
		t.Selection = ui.RGBA(179, 88, 45, 0.4)
		t.Focus = ui.RGBA(179, 88, 45, 0.6)
		t.Inverse = ui.Hex("#F5F5F7")
		t.InverseText = ui.Hex("#1D1D1F")
		t.Radius = 8
		return t
	}
	t := ui.LightTheme()
	t.Background = ui.Hex("#FFFFFF")
	t.Surface = ui.Hex("#FFFFFF")
	t.SurfaceHover = ui.Hex("#F2F2F4")
	t.SurfacePressed = ui.Hex("#E8E8EB")
	t.Border = ui.Hex("#DCDCE0")
	t.Text = ui.Hex("#1D1D1F")
	t.TextMuted = ui.Hex("#6E6E73")
	t.Accent = ui.Hex("#B3582D")
	t.AccentHover = ui.Hex("#B95C31")
	t.AccentPressed = ui.Hex("#984925")
	t.Danger = ui.Hex("#E5342A")
	t.Warning = ui.Hex("#D97E00")
	t.Success = ui.Hex("#24A148")
	t.Selection = ui.RGBA(179, 88, 45, 0.25)
	t.Focus = ui.RGBA(179, 88, 45, 0.5)
	t.Inverse = ui.Hex("#1D1D1F")
	t.InverseText = ui.Hex("#FFFFFF")
	t.Radius = 8
	return t
}

// Tile is a rounded square holding an icon: tinted lightly with the icon in
// the tint, or, when solid, filled with the tint and a white icon.
func Tile(c *ui.Context, icon *ui.SVG, k Tint, size float32, solid bool) ui.Element {
	t := c.Theme()
	col := k.C(t)
	bg, fg := col.Alpha(0.15), k.Text(t)
	if solid {
		bg, fg = col, ui.RGB(255, 255, 255)
	}
	return ui.Row(c).Size(size, size).Radius(size * 0.28).Background(bg).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, icon).Size(size*0.58, size*0.58).TextColor(fg)
	})
}

// Panel is a card of the settings window.
func Panel(c *ui.Context) ui.Element {
	return ui.Column(c).Padding(16).Gap(0).Radius(10).Background(pal(c.Theme()).Card)
}

// PillButton is a capsule button with an optional icon. A disabled one
// reports no clicks.
func PillButton(c *ui.Context, icon *ui.SVG, label string, disabled bool) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Label(label).Disabled(disabled).Padding(6, 12).Gap(6).Radius(6).AlignItems(ui.Center)
	if label == "" {
		b.Padding(6, 8) // an icon alone: name it with Label
	}
	bg := t.Text.Alpha(0.06)
	switch {
	case disabled:
	case b.Pressed():
		bg = t.Text.Alpha(0.14)
	case b.Hovered():
		bg = t.Text.Alpha(0.10)
	}
	b.Background(bg)
	return b.Children(func() {
		col := t.Text
		if disabled {
			col = t.TextMuted
		}
		if icon != nil {
			ui.Icon(c, icon).FontSize(13).TextColor(col)
		}
		if label != "" {
			ui.Text(c, label).FontSize(13).FontWeight(500).TextColor(col).SingleLine()
		}
	})
}

// PillTabs is a capsule segmented control. It reports whether the choice
// changed. A disabled control reports no changes.
func PillTabs(c *ui.Context, selected *int, disabled bool, labels ...string) bool {
	t := c.Theme()
	seg := ui.SegmentedBase(c, selected, len(labels))
	seg.Track.Padding(3).Gap(2).Radius(7).Background(t.Text.Alpha(0.06)).Children(func() {
		for i, l := range labels {
			s := seg.Segment(i).Padding(5, 10).Radius(5).Disabled(disabled)
			col := t.TextMuted
			if i == *selected {
				col = t.Text
				if disabled {
					col = t.TextMuted
				}
				s.Background(pal(t).Inset).Shadow(0, 1, 3, 0, ui.RGBA(0, 0, 0, 0.12))
			}
			s.Children(func() { ui.Text(c, l).FontSize(13).FontWeight(500).TextColor(col).SingleLine() })
		}
	})
	// Querying Changed applies the click now, while selected still points at
	// the caller's variable; otherwise mygo applies it after the frame.
	return seg.Track.Changed()
}

// IconButton is compact toolbar chrome with a translated tooltip and name.
func IconButton(c *ui.Context, icon *ui.SVG, label string, disabled bool) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Label(label).Tooltip(label).Disabled(disabled).Size(26, 26).Radius(5).AlignItems(ui.Center).Justify(ui.Center)
	if b.Pressed() {
		b.Background(t.Text.Alpha(0.12))
	} else if b.Hovered() {
		b.Background(pal(t).Hover)
	}
	col := t.TextMuted
	if disabled {
		col = col.Alpha(0.5)
	}
	return b.Children(func() { ui.Icon(c, icon).Size(14, 14).TextColor(col) })
}

// inverseColors is the tooltip fill and ink of t, as mygo resolves them.
func inverseColors(t *ui.Theme) (fill, text ui.Color) {
	fill, text = t.Inverse, t.InverseText
	if fill == ui.Transparent {
		fill = t.Text
	}
	if text == ui.Transparent {
		text = t.Background
	}
	return fill, text
}

// applyPanelTheme shows native material only when the host actually provides it.
func applyPanelTheme(c *ui.Context, style store.PanelBackground) *ui.Theme {
	t := applyTheme(c)
	if style == store.PanelBackgroundTranslucent && c.Vibrancy() {
		material := *t
		// Tooltips draw the inverse colours, which fall back to Text on
		// Background; keep that ink opaque once the background is cleared.
		material.Inverse, material.InverseText = inverseColors(t)
		material.Background = ui.Transparent
		c.SetTheme(&material)
		c.Root().Background(ui.Transparent)
		return &material
	}
	return t
}
