package app

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/runway"
	"github.com/licoy/quosparr/internal/store"
	xdraw "golang.org/x/image/draw"
)

func TestTrayStylePreview(t *testing.T) {
	dir := os.Getenv("RUNWAY_TRAY_QA_DIR")
	if dir == "" {
		t.Skip("set RUNWAY_TRAY_QA_DIR to export tray previews")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, lang := range l10n.Languages() {
		writeTrayPreview(t, dir, lang)
	}
}

func previewTrayStatus(style store.StatusBarStyle, lang l10n.Language) TrayRender {
	lc := l10n.New(lang)
	now := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	base := trayTestStatus(style, nil)
	meters := base.Plan.Meters
	meters[0].Title = lc.Text(l10n.KeyFiveHourUsage)
	meters[1].Title = lc.Text(l10n.KeyWeeklyUsage)
	prefs := store.DefaultPreferences()
	prefs.StatusBarDisplayStyle = style
	prefs.StatusBarBatteryScope = store.BatteryScopeBoth
	return TrayFromStatus(runway.BuildStatusBar(lc, runway.StatusInputs{Prefs: prefs, Provider: store.ProviderCodex, CodexText: runway.DurationLocalized(10800, lang, false), CodexMeters: meters, Now: now}))
}

func writeTrayPreview(t *testing.T, dir string, lang l10n.Language) {
	t.Helper()
	styles := []store.StatusBarStyle{store.StatusBarStyleMeters, store.StatusBarStyleCountdown, store.StatusBarStyleBattery, store.StatusBarStyleRings, store.StatusBarStyleText}
	// Reserve the measured full content width plus a separate square icon
	columnWidth := 0
	for _, style := range styles {
		img := decodeTrayPreview(t, previewTrayStatus(style, lang), true, false)
		columnWidth = max(columnWidth, img.Bounds().Dx()+90)
	}
	canvas := image.NewNRGBA(image.Rect(0, 0, 150+2*columnWidth, 440))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{C: color.NRGBA{240, 240, 240, 255}}, image.Point{}, draw.Src)
	label := ui.Render(func(c *ui.Context) { ui.Text(c, lang.NativeTitle()).FontSize(15) }, 150, 30, 1)
	draw.Draw(canvas, image.Rect(0, 0, 150, 30), label, image.Point{}, draw.Over)
	for i, style := range styles {
		r := previewTrayStatus(style, lang)
		y := 44 + i*76
		label = ui.Render(func(c *ui.Context) { ui.Text(c, string(style)).FontSize(15) }, 120, 30, 1)
		draw.Draw(canvas, image.Rect(12, y+12, 132, y+42), label, image.Point{}, draw.Over)
		for j, light := range []bool{true, false} {
			x := 150 + j*columnWidth
			bg := color.NRGBA{255, 255, 255, 255}
			if !light {
				bg = color.NRGBA{32, 32, 35, 255}
			}
			draw.Draw(canvas, image.Rect(x-8, y-6, x+columnWidth-8, y+58), &image.Uniform{C: bg}, image.Point{}, draw.Src)
			full := decodeTrayPreview(t, r, light, false)
			draw.Draw(canvas, full.Bounds().Add(image.Pt(x, y)), full, image.Point{}, draw.Over)
			icon := decodeTrayPreview(t, r, light, true)
			xdraw.CatmullRom.Scale(canvas, image.Rect(x+columnWidth-64, y, x+columnWidth-20, y+44), icon, icon.Bounds(), draw.Over, nil)
		}
	}
	name := "tray-styles-" + string(lang) + ".png"
	if lang == l10n.English {
		name = "tray-styles.png"
	}
	file, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, canvas); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func decodeTrayPreview(t *testing.T, r TrayRender, light, compact bool) image.Image {
	t.Helper()
	data, err := TrayImage(r, light, compact)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func TestTrayLayerPreview(t *testing.T) {
	dir := os.Getenv("RUNWAY_TRAY_QA_DIR")
	if dir == "" {
		t.Skip("set RUNWAY_TRAY_QA_DIR to export native layer previews")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	styles := []store.StatusBarStyle{store.StatusBarStyleMeters, store.StatusBarStyleCountdown, store.StatusBarStyleBattery, store.StatusBarStyleRings, store.StatusBarStyleText}
	width := 0
	for _, style := range styles {
		width = max(width, decodeTrayPreview(t, previewTrayStatus(style, l10n.English), true, false).Bounds().Dx()+24)
	}
	canvas := image.NewNRGBA(image.Rect(0, 0, 150+3*width, 390))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{C: color.NRGBA{240, 240, 240, 255}}, image.Point{}, draw.Src)
	for i, style := range styles {
		r := previewTrayStatus(style, l10n.English)
		layers := make([]image.Image, 2)
		for layer, options := range []trayImageOptions{{light: true, foreground: true}, {colorsOnly: true}} {
			data, err := renderTrayImage(r, options)
			if err != nil {
				t.Fatal(err)
			}
			img, err := png.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			layers[layer] = img
			suffix := "foreground"
			if layer == 1 {
				suffix = "colors"
			}
			if err := os.WriteFile(filepath.Join(dir, string(style)+"-"+suffix+".png"), data, 0644); err != nil {
				t.Fatal(err)
			}
		}
		y := 30 + i*70
		label := ui.Render(func(c *ui.Context) { ui.Text(c, string(style)).FontSize(15) }, 140, 30, 1)
		draw.Draw(canvas, image.Rect(5, y, 145, y+30), label, image.Point{}, draw.Over)
		for col, bg := range []color.NRGBA{{255, 255, 255, 255}, {32, 32, 35, 255}, {79, 138, 178, 255}} {
			x := 150 + col*width
			rect := image.Rect(x, y, x+width-5, y+52)
			draw.Draw(canvas, rect, &image.Uniform{C: bg}, image.Point{}, draw.Src)
			ink := color.NRGBA{A: 255}
			if col != 0 {
				ink = color.NRGBA{255, 255, 255, 255}
			}
			target := layers[0].Bounds().Add(image.Pt(x+4, y+4))
			draw.DrawMask(canvas, target, &image.Uniform{C: ink}, image.Point{}, layers[0], image.Point{}, draw.Over)
			draw.Draw(canvas, target, layers[1], image.Point{}, draw.Over)
		}
	}
	file, err := os.Create(filepath.Join(dir, "tray-native-layer-matrix.png"))
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, canvas); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}
