package app

import (
	"bytes"
	"fmt"
	"image/png"
	"math"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/runway"
	"github.com/licoy/quosparr/internal/store"
)

const trayContentHeight = 22
const trayHorizontalPadding float32 = 4

type trayLayout struct {
	widths        []float32
	gap, fontSize float32
	rows          int
}

// TrayImage renders menu bar content or returns the desktop application logo.
// No window, account access or display server is needed.
func TrayImage(r TrayRender, light, compact bool) ([]byte, error) {
	if compact {
		return desktopTrayIconPNG, nil
	}
	return renderTrayImage(r, trayImageOptions{light: light})
}

type trayImageOptions struct {
	light                  bool
	foreground, colorsOnly bool
}

type trayPaint struct {
	ink          ui.Color
	colorOpacity float32
}

func renderTrayImage(r TrayRender, options trayImageOptions) ([]byte, error) {
	if r.Style != store.StatusBarStyleMeters && r.Style != store.StatusBarStyleBattery && r.Style != store.StatusBarStyleRings && r.Style != store.StatusBarStyleCountdown && r.Style != store.StatusBarStyleText {
		return nil, fmt.Errorf("unsupported tray style %q", r.Style)
	}
	layout := makeTrayLayout(r)
	width := float32(0)
	for _, w := range layout.widths {
		width += w
	}
	width += float32(max(0, len(layout.widths)-1))*layout.gap + 2*trayHorizontalPadding
	img := ui.Render(func(c *ui.Context) {
		theme := ui.LightTheme()
		theme.Background = ui.Transparent
		c.SetTheme(theme)
		ui.Box(c).Fill().Draw(func(p *ui.Painter, rect ui.Rect) {
			ink := ui.RGB(0, 0, 0)
			if !options.light {
				ink = ui.RGB(255, 255, 255)
			}
			paint := trayPaint{ink: ink, colorOpacity: 1}
			if options.foreground {
				paint.colorOpacity = 0
			}
			if options.colorsOnly {
				paint.ink = ui.Transparent
			}
			drawTrayContent(p, r, layout, paint)
		})
	}, int(math.Ceil(float64(width))), trayContentHeight, 2)
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return nil, fmt.Errorf("encode tray: %w", err)
	}
	return out.Bytes(), nil
}

func makeTrayLayout(r TrayRender) trayLayout {
	l := trayLayout{rows: max(1, r.RowsPerColumn), fontSize: 8.5, gap: 6}
	switch r.Style {
	case store.StatusBarStyleText:
		l.rows, l.fontSize, l.gap = 1, 14, 4
		for _, text := range r.Lines {
			l.widths = append(l.widths, max(24, trayTextWidth(text, 14)+4))
		}
	case store.StatusBarStyleCountdown:
		l.rows, l.fontSize, l.gap = 1, 10, 2
		if len(r.Items) <= 1 {
			l.fontSize = 14
			l.widths = []float32{min(180, max(42, trayTextWidth(r.Text, 14)+14))}
		} else {
			for _, item := range r.Items {
				l.widths = append(l.widths, min(112, max(42, trayTextWidth(item.Caption, 10)+10)))
			}
		}
	case store.StatusBarStyleRings:
		l.rows, l.fontSize, l.gap = 1, 6, 4
		for range max(1, len(r.Items)) {
			l.widths = append(l.widths, 20)
		}
	case store.StatusBarStyleBattery:
		l.fontSize, l.gap = 7, 5
		if len(r.Items) <= 1 {
			l.fontSize = 10.5
			text := "--"
			if len(r.Items) == 1 {
				text = r.Items[0].Caption
			}
			l.widths = []float32{min(150, max(72, trayTextWidth(text, l.fontSize)+32))}
		}
	}
	if len(l.widths) == 0 {
		for i := 0; i < len(r.Items); i += l.rows {
			w := float32(0)
			for _, item := range r.Items[i:min(i+l.rows, len(r.Items))] {
				w = max(w, trayTextWidth(item.Caption, l.fontSize))
			}
			if r.Style == store.StatusBarStyleBattery {
				w = min(156, max(92, w+18))
			} else {
				w = min(166, max(64, w+44))
			}
			l.widths = append(l.widths, w)
		}
	}
	if len(l.widths) == 0 {
		l.widths = []float32{70}
	}
	return l
}

func trayTextWidth(s string, size float32) float32 {
	var width float32
	for _, g := range ui.Shape(s, trayFont(size)) {
		width = max(width, g.X+g.Advance)
	}
	return width
}

func trayFont(size float32) ui.Font {
	font := ui.Font{Size: size, Weight: 600, Features: "tnum"}
	if size <= 7 {
		// Native fonts have different ascenders; compact labels must fit the
		// 9pt battery body and the smaller ring without clipping their glyphs.
		m := font.Metrics()
		font.Size *= min(1, (size+2)/(m.Ascent+m.Descent))
	}
	return font
}

func drawTrayContent(p *ui.Painter, r TrayRender, l trayLayout, paint trayPaint) {
	ink := paint.ink
	x := trayHorizontalPadding
	for col, width := range l.widths {
		start := col * l.rows
		count := min(l.rows, len(r.Items)-start)
		if r.Style == store.StatusBarStyleText {
			text := "--"
			if col < len(r.Lines) {
				text = r.Lines[col]
			}
			drawTrayText(p, text, ui.Rect{X: x, W: width, H: 22}, 14, ink, true)
		} else if r.Style == store.StatusBarStyleCountdown && len(r.Items) <= 1 {
			drawTrayText(p, r.Text, ui.Rect{X: x, W: width, H: 22}, 14, ink, true)
		} else {
			for row := 0; row < max(1, count); row++ {
				item := TrayItem{Caption: "--"}
				if start+row < len(r.Items) {
					item = r.Items[start+row]
				}
				height := float32(22)
				if count > 1 {
					height = 10
				}
				box := ui.Rect{X: x, Y: (22-float32(max(1, count))*height)/2 + float32(row)*height, W: width, H: height}
				drawTrayItem(p, item, r.Style, box, l.fontSize, paint)
			}
		}
		x += width + l.gap
	}
}

func drawTrayItem(p *ui.Painter, item TrayItem, style store.StatusBarStyle, box ui.Rect, size float32, paint trayPaint) {
	ink := paint.ink
	level := min(1, max(0, item.Remaining))
	fill := trayLevelColor(item.Health, true).Alpha(paint.colorOpacity)
	switch style {
	case store.StatusBarStyleMeters:
		bar := ui.Rect{X: box.X, Y: box.Y + box.H/2 - 2.5, W: 40, H: 5}
		if ink.A > 0 {
			p.Fill(bar, ink.Alpha(0.25), 2.5)
		}
		bar.W *= float32(level)
		if level > 0 {
			p.Fill(bar, fill, 2.5)
		}
		textBox := box
		textBox.X, textBox.W = box.X+44, box.W-44
		drawTrayText(p, item.Caption, textBox, size, ink, false)
	case store.StatusBarStyleBattery:
		drawTrayBattery(p, item, box, size, ink, fill)
	case store.StatusBarStyleRings:
		cx, cy := box.X+box.W/2, box.Y+box.H/2
		if ink.A > 0 {
			p.StrokePath((&ui.Path{}).Circle(cx, cy, 8), 2.5, ink.Alpha(0.25))
		}
		if level > 0 {
			path := &ui.Path{}
			steps := max(1, int(math.Ceil(level*64)))
			for i := 0; i <= steps; i++ {
				a := -math.Pi/2 + level*2*math.Pi*float64(i)/float64(steps)
				x, y := cx+float32(8*math.Cos(a)), cy+float32(8*math.Sin(a))
				if i == 0 {
					path.MoveTo(x, y)
				} else {
					path.LineTo(x, y)
				}
			}
			p.StrokePath(path, 2.5, fill)
		}
		drawTrayText(p, item.Caption, box, 6, ink, true)
	case store.StatusBarStyleCountdown:
		drawTrayText(p, item.Caption, box, size, ink, true)
	}
}

func drawTrayBattery(p *ui.Painter, item TrayItem, box ui.Rect, size float32, ink, fill ui.Color) {
	height, inset, stroke, radius := float32(12), float32(2.15), float32(1.35), float32(3.25)
	if box.H <= 10 {
		height, inset, stroke, radius = 9, 1.6, 1.1, 2.1
	}
	body := ui.Rect{X: box.X + 2, Y: box.Y + (box.H-height)/2, W: box.W - 7, H: height}
	if ink.A > 0 {
		p.Stroke(body, ink.Alpha(0.92), radius, stroke)
		p.Fill(ui.Rect{X: body.X + body.W + 0.85, Y: body.Y + height*0.31, W: 1.75, H: height * 0.38}, ink.Alpha(0.92), 0.7)
	}
	track := ui.Rect{X: body.X + inset, Y: body.Y + inset, W: body.W - 2*inset, H: body.H - 2*inset}
	level := float32(min(1, max(0, item.Remaining)))
	if level > 0 {
		part := track
		part.W = min(track.W, max(2, track.W*level))
		p.Fill(part, fill, 1)
	}
	drawTrayText(p, item.Caption, body, size, ink, true)
	if level > 0 && fill.A > 0 {
		filled := track
		filled.W = min(track.W, max(2, track.W*level))
		p.Clip(filled, 1, func() { drawTrayText(p, item.Caption, body, size, ui.RGB(255, 255, 255), true) })
	}
}

func drawTrayText(p *ui.Painter, text string, box ui.Rect, size float32, ink ui.Color, center bool) {
	if ink.A == 0 {
		return
	}
	font := trayFont(size)
	w := trayTextWidth(text, size)
	// Layout adds and subtracts bar widths in float32; a subpixel rounding
	// difference must not replace a fitting last glyph with an ellipsis
	if w > box.W+0.01 {
		runes := []rune(text)
		for len(runes) > 0 && w > box.W+0.01 {
			runes = runes[:len(runes)-1]
			text = strings.TrimSpace(string(runes)) + "…"
			w = trayTextWidth(text, size)
		}
	}
	x := box.X
	if center {
		x += max(0, (box.W-w)/2)
	}
	m := font.Metrics()
	baseline := box.Y + (box.H-m.Ascent-m.Descent)/2 + m.Ascent
	p.Clip(box, 0, func() { p.Glyphs(ui.Shape(text, font), x, baseline, ink) })
}

// Bright status accents stay consistent while AppKit tints template foregrounds
func trayLevelColor(health runway.Health, _ bool) ui.Color {
	return [...]ui.Color{ui.RGB(48, 209, 88), ui.RGB(255, 159, 10), ui.RGB(255, 69, 58)}[health]
}
