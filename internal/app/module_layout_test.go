package app

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/store"
)

func moduleLayoutView(t *testing.T, width int, language store.Language, module store.Module) (*env, *settingsUI, *ui.Tester) {
	t.Helper()
	e := newEnv(t, nil)
	if err := e.s.Load(); err != nil {
		t.Fatal(err)
	}
	e.s.SetLanguage(language)
	e.s.Apply()
	u := newSettingsUI(e.s, SettingsOptions{Version: "1.0.0"})
	u.tab = tabModules
	viewWidth := width
	if module != "" {
		viewWidth -= settingsSidebarWidth
	}
	tt := ui.NewTester(func(c *ui.Context) {
		applyTheme(c)
		if module != "" {
			ui.Column(c).Fill().Padding(16).Children(func() {
				Panel(c).Children(func() { u.moduleCard(c, e.s.Prefs(), module, 0, 1) })
			})
			return
		}
		u.shell(c, func(c *ui.Context) {
			ui.Scroll(c).Grow(1).Padding(16).Children(func() {
				u.modulesGroup(c)
			})
		})
	}, viewWidth, settingsHeight)
	e.s.OnChange(tt.Frame)
	return e, u, tt
}

func moduleLayoutRect(t *testing.T, tt *ui.Tester, text string) ui.Rect {
	t.Helper()
	r, ok := tt.Find(text)
	if !ok {
		t.Fatalf("missing %q", text)
	}
	return r
}

func TestModuleLayoutColumnsAndConfiguration(t *testing.T) {
	configKeys := map[store.Module]l10n.Key{
		store.ModuleQuota:               l10n.KeyShowModelSpecificQuotaUsage,
		store.ModuleTokenUsage:          l10n.KeyTokenUsageChartStyle,
		store.ModuleRateLimitResetToday: l10n.KeyRateLimitResetTodayRefreshInterval,
		store.ModuleQuotaEstimate:       l10n.KeyQuotaEstimateWindowMode,
		store.ModuleAPICost:             l10n.KeyApiCostSummaryRange,
	}
	for _, width := range []int{settingsMinWidth, settingsWidth} {
		for _, language := range []store.Language{store.LanguageSimplifiedChinese, store.LanguageEnglish, store.LanguageRussian} {
			for _, module := range store.Modules() {
				t.Run(fmt.Sprintf("%d/%s/%s", width, language, module), func(t *testing.T) {
					e, u, tt := moduleLayoutView(t, width, language, module)
					defer e.settle()
					for _, dark := range []bool{false, true} {
						tt.SetDark(dark)
						l := u.L()
						titleText, detailText := moduleTexts(l, module)
						title := moduleLayoutRect(t, tt, titleText)
						detail := moduleLayoutRect(t, tt, detailText)
						stateKey := l10n.KeyModuleHidden
						if ModuleShown(e.s.Prefs(), module) {
							stateKey = l10n.KeyModuleVisible
						}
						control := moduleLayoutRect(t, tt, l.Text(stateKey))
						if detail.X != title.X || detail.Y < title.Y+title.H || title.X+title.W > control.X || detail.X+detail.W > control.X {
							t.Fatalf("module text columns overlap controls: title=%+v detail=%+v switch=%+v", title, detail, control)
						}
						up := moduleLayoutRect(t, tt, l.Text(l10n.KeyAccountsMoveUp))
						for _, action := range []ui.Rect{control, up} {
							if math.Abs(float64(title.Y+title.H/2-action.Y-action.H/2)) > 0.5 {
								t.Fatalf("module title and controls have different center lines: title=%+v action=%+v", title, action)
							}
						}
						if key, ok := configKeys[module]; ok {
							config := moduleLayoutRect(t, tt, l.Text(key))
							if config.X != title.X || config.Y <= detail.Y+detail.H {
								t.Fatalf("configuration is outside its module text column: title=%+v detail=%+v config=%+v", title, detail, config)
							}
						}
						for _, text := range tt.Texts() {
							r := moduleLayoutRect(t, tt, text)
							if r.X < 0 || r.X+r.W > float32(width-settingsSidebarWidth) {
								t.Fatalf("text %q exceeds window: %+v", text, r)
							}
						}
					}
				})
			}
		}
	}
}

func TestModuleLayoutSwitchesKeepHiddenConfigurationRules(t *testing.T) {
	for _, test := range []struct {
		module store.Module
		config l10n.Key
		keep   bool
	}{
		{store.ModuleQuota, l10n.KeyShowModelSpecificQuotaUsage, true},
		{store.ModuleTokenUsage, l10n.KeyTokenUsageChartStyle, false},
		{store.ModuleRateLimitResetToday, l10n.KeyRateLimitResetTodayRefreshInterval, false},
		{store.ModuleQuotaEstimate, l10n.KeyQuotaEstimateWindowMode, false},
		{store.ModuleAPICost, l10n.KeyApiCostSummaryRange, false},
	} {
		t.Run(string(test.module), func(t *testing.T) {
			e, u, tt := moduleLayoutView(t, settingsMinWidth, store.LanguageEnglish, test.module)
			if err := tt.Click(u.text(l10n.KeyModuleVisible)); err != nil {
				t.Fatal(err)
			}
			e.settle()
			if ModuleShown(e.stored(), test.module) || tt.HasText(u.text(test.config)) != test.keep {
				t.Fatal("hiding a module changed the configuration visibility contract")
			}
			if err := tt.Click(u.text(l10n.KeyModuleHidden)); err != nil {
				t.Fatal(err)
			}
			e.settle()
			if !ModuleShown(e.stored(), test.module) || !tt.HasText(u.text(test.config)) {
				t.Fatal("showing a module did not restore its setting")
			}
		})
	}
}

func TestModuleLayoutOrderAndRestoreButtons(t *testing.T) {
	e, u, tt := moduleLayoutView(t, settingsMinWidth, store.LanguageEnglish, "")
	before := e.s.Prefs().MainPanelModuleOrder
	if err := tt.Click(u.text(l10n.KeyAccountsMoveDown)); err != nil {
		t.Fatal(err)
	}
	e.settle()
	after := e.stored().MainPanelModuleOrder
	if after[0] != before[1] || after[1] != before[0] {
		t.Fatal("move down button did not swap the first two modules")
	}
	up := moduleLayoutRect(t, tt, u.text(l10n.KeyAccountsMoveUp))
	quota := moduleLayoutRect(t, tt, u.text(l10n.KeyQuota))
	tt.ClickAt(up.X+up.W/2, quota.Y+quota.H/2)
	e.settle()
	if !slices.Equal(e.stored().MainPanelModuleOrder, before) {
		t.Fatal("move up button did not return quota to its original position")
	}
	if err := tt.Click(u.text(l10n.KeyAccountsMoveDown)); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(u.text(l10n.KeyRestoreDefaultOrder)); err != nil {
		t.Fatal(err)
	}
	e.settle()
	if !slices.Equal(e.stored().MainPanelModuleOrder, store.Modules()) {
		t.Fatal("restore order button did not restore defaults")
	}
}

func TestModuleLayoutReadOnlySwitchDoesNotWrite(t *testing.T) {
	e, u, tt := moduleLayoutView(t, settingsMinWidth, store.LanguageEnglish, store.ModuleQuota)
	e.settle()
	e.mem.Put(store.KeyPreferences, []byte("{broken"))
	if err := e.s.Load(); err == nil {
		t.Fatal("broken preferences should be read-only")
	}
	tt.Frame()
	before := e.mem.Writes()
	if err := tt.Click(u.text(l10n.KeyModuleVisible)); err != nil {
		t.Fatal(err)
	}
	e.settle()
	if !ModuleShown(e.s.Prefs(), store.ModuleQuota) || e.mem.Writes() != before {
		t.Fatal("read-only module switch changed preferences")
	}
}

func TestExportModuleLayoutReview(t *testing.T) {
	if os.Getenv("RUNWAY_MODULE_LAYOUT_QA") == "" {
		t.Skip("set RUNWAY_MODULE_LAYOUT_QA=1 to export offscreen PNGs")
	}
	dir := filepath.Join("..", "..", ".build", "v1-settings-layout-qa")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, width := range []int{settingsMinWidth, settingsWidth} {
		for _, language := range []store.Language{store.LanguageSimplifiedChinese, store.LanguageEnglish, store.LanguageRussian} {
			for _, dark := range []bool{false, true} {
				e, u, tt := moduleLayoutView(t, width, language, "")
				tt.SetDark(dark)
				tt.SetTitleBar(ui.TitleBar{Height: 28, Left: 80})
				theme := "light"
				if dark {
					theme = "dark"
				}
				name := fmt.Sprintf("modules-%s-%d-%s", theme, width, language)
				exportReviewPNG(t, tt, filepath.Join(dir, name+"-top.png"))
				tt.Scroll(float32(width-100), 500, 0, 1000)
				exportReviewPNG(t, tt, filepath.Join(dir, name+"-bottom.png"))
				page := ui.NewTester(u.view, width, settingsHeight)
				page.SetDark(dark)
				page.SetTitleBar(ui.TitleBar{Height: 28, Left: 80})
				exportReviewPNG(t, page, filepath.Join(dir, name+"-page-top.png"))
				page.Scroll(float32(width-100), 500, 0, 500)
				exportReviewPNG(t, page, filepath.Join(dir, name+"-page-bottom.png"))
				e.settle()
			}
		}
	}
}
