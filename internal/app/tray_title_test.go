package app

import (
	"bytes"
	"image/color"
	"image/png"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/runway"
	"github.com/licoy/quosparr/internal/store"
)

func TestTrayFromStatusPreservesStyleCaptions(t *testing.T) {
	now := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	meters := []runway.Meter{{Title: "5h", RemainingPercent: 80, ResetsAt: now.Add(3 * time.Hour)}, {Title: "Weekly", RemainingPercent: 35, ResetsAt: now.Add(48 * time.Hour)}}
	prefs := store.DefaultPreferences()
	prefs.StatusBarDisplayStyle = store.StatusBarStyleText
	v := runway.BuildStatusBar(l10n.New(l10n.English), runway.StatusInputs{Prefs: prefs, Provider: store.ProviderCodex, CodexText: "3h", CodexMeters: meters, Now: now})
	got := TrayFromStatus(v)
	if !reflect.DeepEqual(got.Lines, []string{"80%", "35%"}) {
		t.Fatalf("text style discarded percentage captions: got %q, want [80%% 35%%]", got.Lines)
	}
}

func trayTestStatus(style store.StatusBarStyle, customize func(*store.Preferences)) runway.StatusBarView {
	now := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	prefs := store.DefaultPreferences()
	prefs.StatusBarDisplayStyle = style
	prefs.StatusBarBatteryScope = store.BatteryScopeBoth
	if customize != nil {
		customize(&prefs)
	}
	meters := []runway.Meter{
		{Title: "5h", WindowMinutes: 300, RemainingPercent: 80, ResetsAt: now.Add(3 * time.Hour), Health: runway.HealthGreen},
		{Title: "Weekly", WindowMinutes: 10080, RemainingPercent: 35, ResetsAt: now.Add(48 * time.Hour), Health: runway.HealthYellow},
	}
	return runway.BuildStatusBar(l10n.New(l10n.English), runway.StatusInputs{Prefs: prefs, Provider: store.ProviderCodex, CodexText: "3h", CodexMeters: meters, Now: now})
}

func TestTrayStatusRendersFiveDifferentStyles(t *testing.T) {
	styles := []store.StatusBarStyle{store.StatusBarStyleMeters, store.StatusBarStyleCountdown, store.StatusBarStyleBattery, store.StatusBarStyleRings, store.StatusBarStyleText}
	seen := map[string]store.StatusBarStyle{}
	for _, style := range styles {
		v := trayTestStatus(style, nil)
		r := TrayFromStatus(v)
		if r.Style != style || r.RowsPerColumn != v.Plan.RowsPerColumn || len(r.Items) != 2 {
			t.Fatalf("style/layout lost: %+v", r)
		}
		data, err := TrayImage(r, true, false)
		if err != nil {
			t.Fatal(err)
		}
		if prior, ok := seen[string(data)]; ok {
			t.Fatalf("%s and %s rendered identically", prior, style)
		}
		seen[string(data)] = style
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		if img.Bounds().Dy() != 44 {
			t.Fatalf("unexpected tray image bounds %v", img.Bounds())
		}
		if _, _, _, alpha := img.At(0, 0).RGBA(); alpha != 0 {
			t.Fatalf("style %s image has an opaque background", style)
		}
	}
}

func TestDesktopTrayKeepsApplicationLogoForEveryStatus(t *testing.T) {
	states := []TrayRender{{}}
	for _, style := range store.StatusBarStyles() {
		states = append(states, TrayRender{Style: style}, TrayFromStatus(trayTestStatus(style, nil)))
	}
	for _, state := range states {
		for _, light := range []bool{false, true} {
			data, err := TrayImage(state, light, true)
			if err != nil || !bytes.Equal(data, desktopTrayIconPNG) {
				t.Fatalf("desktop tray replaced logo for style %q, light=%v: %v", state.Style, light, err)
			}
		}
	}
}

func TestTrayDetailsReachRenderer(t *testing.T) {
	seen := map[string]bool{}
	for _, detail := range []store.MetersDetail{store.MetersDetailRemainingPercent, store.MetersDetailResetTime, store.MetersDetailBoth} {
		v := trayTestStatus(store.StatusBarStyleMeters, func(p *store.Preferences) { p.StatusBarMetersDetailStyle = detail })
		r := TrayFromStatus(v)
		if r.Items[0].Caption != v.Items[0].MeterText {
			t.Fatal("meter detail discarded")
		}
		image, err := TrayImage(r, true, false)
		if err != nil {
			t.Fatal(err)
		}
		if seen[string(image)] {
			t.Fatal("different meter details rendered identically")
		}
		seen[string(image)] = true
	}
	for _, detail := range []store.BatteryDetail{store.BatteryDetailRemainingPercent, store.BatteryDetailCountdown} {
		v := trayTestStatus(store.StatusBarStyleBattery, func(p *store.Preferences) {
			p.StatusBarBatteryScope = store.BatteryScopeWeekly
			p.StatusBarBatteryDetailStyle = detail
		})
		r := TrayFromStatus(v)
		if len(r.Items) != 1 || r.Items[0].Caption != v.Items[0].BatteryDetail || strings.Contains(r.Items[0].Caption, "Weekly") {
			t.Fatalf("single battery scope/detail discarded: %+v", r)
		}
	}
}

func TestTrayDualProvidersKeepTwoRows(t *testing.T) {
	v := trayTestStatus(store.StatusBarStyleMeters, nil)
	prefs := store.DefaultPreferences()
	prefs.StatusBarProviderScope = store.ProviderScopeBoth
	v = runway.BuildStatusBar(l10n.New(l10n.English), runway.StatusInputs{Prefs: prefs, Provider: store.ProviderCodex, CodexText: "3h", CodexMeters: v.Plan.Meters, Grok: runway.GrokState{Meters: []runway.Meter{{WindowMinutes: 10080, RemainingPercent: 75}}}, Now: time.Now()})
	r := TrayFromStatus(v)
	if r.RowsPerColumn != 2 || len(r.Items) != 2 || !strings.Contains(r.Items[0].Caption, "Codex") || !strings.Contains(r.Items[1].Caption, "Grok") {
		t.Fatalf("dual provider rows lost: %+v", r)
	}
	if got := makeTrayLayout(r); len(got.widths) != 1 || got.rows != 2 {
		t.Fatalf("dual providers must share a two-row column: %+v", got)
	}
}

func TestTrayRendererRejectsUnsupportedStyle(t *testing.T) {
	if _, err := TrayImage(TrayRender{Style: "unknown"}, true, false); err == nil {
		t.Fatal("unsupported style must fail visibly")
	}
}

func TestTrayContentCacheDetectsChanges(t *testing.T) {
	a := TrayFromStatus(trayTestStatus(store.StatusBarStyleMeters, nil))
	b := TrayFromStatus(trayTestStatus(store.StatusBarStyleMeters, nil))
	if !sameTrayRender(a, b) {
		t.Fatal("unchanged snapshot must reuse its tray content")
	}
	b.Items[0].Caption = "changed detail"
	if sameTrayRender(a, b) {
		t.Fatal("detail changes must redraw")
	}
	b = TrayFromStatus(trayTestStatus(store.StatusBarStyleText, nil))
	if sameTrayRender(a, b) {
		t.Fatal("style changes must redraw")
	}
}

func TestTraySmallLabelsUseMeasuredFontAndBaseline(t *testing.T) {
	for _, label := range []string{"Weekly 35%", "每周 35%", "Недельная квота 35%"} {
		font := trayFont(7)
		metrics := font.Metrics()
		if metrics.Ascent+metrics.Descent > 9 {
			t.Fatalf("7pt label cannot fit 9pt battery body: %+v", metrics)
		}
		width := trayTextWidth(label, 7)
		var fullWidth float32
		for _, g := range ui.Shape(label, font) {
			fullWidth = max(fullWidth, g.X+g.Advance)
		}
		if width != fullWidth {
			t.Fatalf("measurement does not use the drawn font: %v / %v", width, fullWidth)
		}
		drawLabel := func(w float32) []byte {
			img := ui.Render(func(c *ui.Context) {
				theme := ui.LightTheme()
				theme.Background = ui.Transparent
				c.SetTheme(theme)
				ui.Box(c).Fill().Draw(func(p *ui.Painter, _ ui.Rect) {
					drawTrayText(p, label, ui.Rect{X: 0, Y: 2, W: w, H: 9}, 7, ui.RGB(0, 0, 0), false)
				})
			}, 300, 14, 2)
			return img.Pix
		}
		// A float32 add/subtract from the bar layout may shrink the width
		// by a fraction of a device pixel; it must still keep the last glyph
		if !bytes.Equal(drawLabel(width), drawLabel((width+44)-44)) {
			t.Fatalf("fitting label was truncated by float rounding: %s", label)
		}
	}
}

func TestMeterIcon(t *testing.T) {
	if MeterIcon(nil, color.NRGBA{A: 255}) != nil {
		t.Error("no meters, no icon")
	}
	ink := color.NRGBA{A: 255}
	one := MeterIcon([]TrayMeter{{Remaining: 0.5}}, ink)
	two := MeterIcon([]TrayMeter{{Remaining: 1}, {Remaining: 0}}, ink)
	a, err := png.Decode(bytes.NewReader(one))
	if err != nil {
		t.Fatal(err)
	}
	b, err := png.Decode(bytes.NewReader(two))
	if err != nil {
		t.Fatal(err)
	}
	if b.Bounds().Dx() <= a.Bounds().Dx() || a.Bounds().Dy() != meterIconH {
		t.Errorf("sizes %v %v", a.Bounds(), b.Bounds())
	}
}

func TestMeterIconFillFollowsWhatIsLeft(t *testing.T) {
	count := func(r float64) int {
		img, err := png.Decode(bytes.NewReader(MeterIcon([]TrayMeter{{Remaining: r}}, color.NRGBA{A: 255})))
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for y := 0; y < img.Bounds().Dy(); y++ {
			for x := 0; x < img.Bounds().Dx(); x++ {
				if _, _, _, a := img.At(x, y).RGBA(); a != 0 {
					n++
				}
			}
		}
		return n
	}
	empty, half, full := count(0), count(0.5), count(1)
	if !(empty < half && half < full) {
		t.Errorf("filled pixels %d %d %d", empty, half, full)
	}
	if count(-3) != empty || count(9) != full {
		t.Error("out-of-range fractions must clamp")
	}
}

func TestTrayInk(t *testing.T) {
	if trayInk(true) != (color.NRGBA{A: 255}) || trayInk(false) != (color.NRGBA{255, 255, 255, 255}) {
		t.Error("ink must be dark on a light bar and light on a dark one")
	}
}
