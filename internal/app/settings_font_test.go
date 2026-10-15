package app

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/store"
)

func TestDisplayFontChoosesAnInstalledFamilyAndBackToDefault(t *testing.T) {
	u := cardSettingsUI(t, store.LanguageEnglish)
	listed := 0
	u.s.sys.FontFamilies = func() []string { listed++; return []string{"Avenir", "Menlo", "PingFang SC"} }
	u.tab = tabDisplay
	tt := ui.NewTester(u.view, settingsWidth, settingsHeight)
	def := tx(u.L(), xDisplayFontDefault)
	clickFontControl(t, u, tt)
	tt.Type("men")
	tt.Key(0, ui.KeyEnter)
	if got := u.s.Prefs().FontFamily; got != "Menlo" {
		t.Fatalf("font family = %q, want Menlo", got)
	}
	var font string
	ui.NewTester(func(c *ui.Context) { font = applyThemeColor(c, "", u.s.Prefs().FontFamily).Font }, 10, 10)
	if font != "Menlo" {
		t.Fatalf("theme font = %q", font)
	}
	tt.Key(0, ui.KeyEscape)
	tt.ClickAt(10, float32(settingsHeight-10))
	clickFontControl(t, u, tt)
	tt.Type(def)
	tt.Key(0, ui.KeyEnter)
	if got := u.s.Prefs().FontFamily; got != "" {
		t.Fatalf("font family = %q after choosing Default", got)
	}
	if listed != 1 {
		t.Errorf("font families listed %d times, want once per window", listed)
	}
}

// clickFontControl clicks the combobox at the right of the display font row;
// its text is no label the tester finds.
func clickFontControl(t *testing.T, u *settingsUI, tt *ui.Tester) {
	t.Helper()
	title, ok := tt.Find(tx(u.L(), xDisplayFont))
	detail, detailOK := tt.Find(tx(u.L(), xDisplayFontHint))
	if !ok || !detailOK {
		t.Fatal("display font row missing")
	}
	tt.ClickAt(float32(settingsWidth-120), (title.Y+detail.Y+detail.H)/2)
}
