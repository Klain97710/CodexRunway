package app

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"slices"
	"strings"

	"github.com/egoist/mygo"
	"github.com/licoy/quosparr/internal/runway"
	"github.com/licoy/quosparr/internal/store"
)

// TrayRender is what the status item shows. The coordinator builds it from
// the status bar settings (one plan for every style: it decides the text and
// the meters), and ApplyTray draws it.
type TrayRender struct {
	Style         store.StatusBarStyle
	Text          string
	Items         []TrayItem
	RowsPerColumn int
	// Lines preserves the visible captions for the text style and tooltip
	Lines []string
	// Meters is retained for the startup/legacy tray plan without a style
	Meters []TrayMeter
	// ToolTip is the hover text. Empty keeps the title text.
	ToolTip string
}

type TrayItem struct {
	Caption   string
	Remaining float64
	Health    runway.Health
}

// The application has one status item; keep only its last successful draw
// This state is accessed on MyGo's main thread, never from refresh workers
var lastAppliedTray struct {
	tray   *mygo.Tray
	render TrayRender
	light  bool
}

func sameTrayRender(a, b TrayRender) bool {
	return a.Style == b.Style && a.Text == b.Text && a.RowsPerColumn == b.RowsPerColumn && a.ToolTip == b.ToolTip && slices.Equal(a.Lines, b.Lines) && slices.Equal(a.Items, b.Items) && slices.Equal(a.Meters, b.Meters)
}

func rememberTray(tray *mygo.Tray, r TrayRender, light bool) {
	stored := r
	stored.Lines = slices.Clone(r.Lines)
	stored.Items = slices.Clone(r.Items)
	stored.Meters = slices.Clone(r.Meters)
	lastAppliedTray.tray, lastAppliedTray.render, lastAppliedTray.light = tray, stored, light
}

// TrayMeter is one bar of the icon.
type TrayMeter struct {
	// Remaining is the fraction left, 0...1 (values outside are clamped).
	Remaining float64
}

// Size of the meter icon, in pixels (2x of a 18 point bar).
const (
	meterIconH = 36
	meterBarW  = 8
	meterGap   = 4
)

// MeterIcon draws meters as a PNG for the tray: each bar is an outline with
// a fill that grows with what is left. ink is the drawing color; on macOS the
// image is a template (black, tinted by the system), elsewhere it follows
// the taskbar (see taskbarLight). Returns nil without meters.
func MeterIcon(meters []TrayMeter, ink color.NRGBA) []byte {
	if len(meters) == 0 {
		return nil
	}
	w := len(meters)*meterBarW + (len(meters)-1)*meterGap + 4
	img := image.NewNRGBA(image.Rect(0, 0, w, meterIconH))
	for i, m := range meters {
		x0 := 2 + i*(meterBarW+meterGap)
		drawMeter(img, x0, m.Remaining, ink)
	}
	var buf bytes.Buffer
	if png.Encode(&buf, img) != nil {
		log.Println("tray: could not encode meter image")
		return nil
	}
	return buf.Bytes()
}

// drawMeter draws one bar: a 1 px outline and the fill from the bottom.
func drawMeter(img *image.NRGBA, x0 int, remaining float64, ink color.NRGBA) {
	top, bottom := 4, meterIconH-4
	h := bottom - top
	fill := int(math.Round(math.Max(0, math.Min(1, remaining)) * float64(h-4)))
	for y := top; y < bottom; y++ {
		for x := x0; x < x0+meterBarW; x++ {
			edge := y == top || y == bottom-1 || x == x0 || x == x0+meterBarW-1
			filled := y >= bottom-2-fill && y < bottom-2 && x > x0+1 && x < x0+meterBarW-2
			if edge || filled {
				img.SetNRGBA(x, y, ink)
			}
		}
	}
}

// trayInk is the drawing color for non-template icons.
func trayInk(light bool) color.NRGBA {
	if light {
		return color.NRGBA{A: 255}
	}
	return color.NRGBA{R: 255, G: 255, B: 255, A: 255}
}

// ApplyTray renders the complete menu bar content, or the desktop application
// icon with tooltip; identical content reuses the last successful draw
// It may be called from any goroutine
func ApplyTray(tray *mygo.Tray, r TrayRender) {
	if tray == nil {
		return
	}
	mygo.RunOnMain(func() {
		if r.Style == "" && trayIsTemplate() {
			applyLegacyTray(tray, r)
			return
		}
		// macOS tints every foreground itself, independently of app appearance
		light := trayIsTemplate() || trayLight()
		if lastAppliedTray.tray == tray && lastAppliedTray.light == light && sameTrayRender(lastAppliedTray.render, r) {
			return
		}
		if err := setTrayContent(tray, r, light); err != nil {
			log.Printf("tray: layout failed: %v", err)
			return
		}
		tip := strings.TrimSpace(r.ToolTip + "\n" + strings.Join(r.Lines, " · "))
		tray.SetToolTip(trayTooltip(tip))
		rememberTray(tray, r, light)
	})
}

func applyLegacyTray(tray *mygo.Tray, r TrayRender) {
	icon := MeterIcon(r.Meters, meterInk(trayIsTemplate()))
	template := trayIsTemplate()
	if icon == nil {
		icon, template = trayIcon()
	}
	if err := tray.SetIcon(icon, template); err != nil {
		log.Printf("tray: set icon failed: %v", err)
	}
	setTrayLines(tray, r.Lines)
	tip := r.ToolTip
	if tip == "" {
		tip = strings.Join(r.Lines, " · ")
	}
	tray.SetToolTip(trayTooltip(tip))
}

func trayTooltip(tip string) string {
	if strings.TrimSpace(tip) == "" {
		return "Quosparr"
	}
	return tip
}

func meterInk(template bool) color.NRGBA {
	if template {
		return color.NRGBA{A: 255}
	}
	return trayInk(taskbarLight())
}
