package app

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/store"
)

var networkCardCases = []struct {
	name string
	mode store.ProxyMode
	auth bool
}{
	{"system", store.ProxyModeSystem, false},
	{"http", store.ProxyModeHTTP, false},
	{"socks5", store.ProxyModeSOCKS5, false},
	{"auth", store.ProxyModeHTTP, true},
}

func cardSettingsUI(t *testing.T, language store.Language) *settingsUI {
	t.Helper()
	e := newEnv(t, nil)
	if err := e.s.Load(); err != nil {
		t.Fatal(err)
	}
	if err := e.s.LoadProxy(); err != nil {
		t.Fatal(err)
	}
	e.s.prefs.Language = language
	e.s.applyLocale()
	e.s.SetProxyHooks(ProxyHooks{Test: func(context.Context, store.NetworkProxyConfiguration, *ProxyCredentials, string) error {
		panic("layout tests must never start a connection")
	}})
	u := newSettingsUI(e.s, SettingsOptions{Version: "1.0.0"})
	u.draftReady = true
	return u
}

func networkCardTester(u *settingsUI, width int, dark bool) *ui.Tester {
	tt := ui.NewTester(func(c *ui.Context) {
		applyTheme(c)
		ui.Column(c).Fill().Padding(16).Children(func() { u.proxyGroup(c) })
	}, width-settingsSidebarWidth, 640)
	tt.SetDark(dark)
	return tt
}

func TestSettingsNetworkActionsStayInsideCard(t *testing.T) {
	for _, language := range []store.Language{store.LanguageEnglish, store.LanguageSimplifiedChinese} {
		for _, dark := range []bool{false, true} {
			for _, width := range []int{settingsMinWidth, settingsWidth} {
				for _, tc := range networkCardCases {
					t.Run(fmt.Sprintf("%s/%t/%d/%s", language, dark, width, tc.name), func(t *testing.T) {
						u := cardSettingsUI(t, language)
						u.draft = ProxyDraft{Mode: tc.mode, UsesAuth: tc.auth, Host: "127.0.0.1", Port: "8080"}
						u.proxyMsg = u.text(l10n.KeyNetworkProxySaved)
						tt := networkCardTester(u, width, dark)
						for _, key := range []l10n.Key{l10n.KeyNetworkProxyTest, l10n.KeyNetworkProxySave, l10n.KeyNetworkProxySaved} {
							r, ok := tt.Find(u.text(key))
							if !ok || r.Y+r.H > 624 || r.X+r.W > float32(width-settingsSidebarWidth-28) {
								t.Fatalf("network action %s is missing or out of bounds: %+v", key, r)
							}
							// The left box padding must extend through the entire action row.
							assertCardPixel(t, tt.Image(), 22, int(r.Y+r.H/2), pal(buildTheme(dark)).Card)
						}
						assertProxyFields(t, tt, u, tc.mode != store.ProxyModeSystem, tc.auth)
						if tc.mode != store.ProxyModeSystem {
							host, _ := tt.Find(u.text(l10n.KeyNetworkProxyHost))
							save, _ := tt.Find(u.text(l10n.KeyNetworkProxySave))
							// Find returns the row label first, so use the fixed form inset for the right edge.
							if host.X != 28 || save.X+save.W != float32(width-settingsSidebarWidth-28) {
								t.Fatal("proxy form and actions do not share the box insets")
							}
						}
						assertSectionTitle(t, tt, u.text(l10n.KeyNetworkProxy), dark)
					})
				}
			}
		}
	}
}

func assertProxyFields(t *testing.T, tt *ui.Tester, u *settingsUI, manual, auth bool) {
	t.Helper()
	for _, key := range []l10n.Key{l10n.KeyNetworkProxyHost, l10n.KeyNetworkProxyPort, l10n.KeyNetworkProxyAuthentication} {
		if hasExact(tt, u.text(key)) != manual {
			t.Fatalf("manual field %s visibility differs from mode", key)
		}
	}
	for _, key := range []l10n.Key{l10n.KeyNetworkProxyUsername, l10n.KeyNetworkProxyPassword} {
		if hasExact(tt, u.text(key)) != (manual && auth) {
			t.Fatalf("authentication field %s visibility differs from mode", key)
		}
	}
}

func TestSettingsNetworkBusyActionsRemainDisabled(t *testing.T) {
	u := cardSettingsUI(t, store.LanguageEnglish)
	u.draft = ProxyDraft{Mode: store.ProxyModeSystem}
	for _, testing := range []bool{false, true} {
		u.proxyTask.Running, u.testTask.Running = !testing, testing
		u.proxyMsg = "busy feedback"
		tt := networkCardTester(u, settingsWidth, false)
		key := l10n.KeyNetworkProxyTest
		if testing {
			key = l10n.KeyNetworkProxyTesting
		}
		for _, action := range []l10n.Key{key, l10n.KeyNetworkProxySave} {
			if err := tt.Click(u.text(action)); err != nil {
				t.Fatal(err)
			}
		}
		if u.proxyTask.Running != !testing || u.testTask.Running != testing || u.proxyMsg != "busy feedback" {
			t.Fatal("busy action changed the running tasks or feedback")
		}
	}
}

func TestSettingsSectionTitlesSitAboveBoxes(t *testing.T) {
	for _, dark := range []bool{false, true} {
		u := cardSettingsUI(t, store.LanguageEnglish)
		u.draft.Mode = store.ProxyModeSystem
		for _, page := range []struct {
			view func(*ui.Context)
			keys []l10n.Key
		}{
			{u.generalPage, []l10n.Key{l10n.KeyNetworkProxy}},
			{u.appearanceGroup, []l10n.Key{l10n.KeyDisplayAppearanceSection}},
		} {
			tt := ui.NewTester(func(c *ui.Context) {
				applyTheme(c)
				ui.Column(c).Fill().Padding(16).Gap(14).Children(func() { page.view(c) })
			}, settingsMinWidth-settingsSidebarWidth, 1800)
			tt.SetDark(dark)
			for _, key := range page.keys {
				assertSectionTitle(t, tt, u.text(key), dark)
			}
		}
		// The first section of a page is untitled: the page header names it.
		tt := ui.NewTester(func(c *ui.Context) { applyTheme(c); u.advancedPage(c) }, 600, 600)
		if hasExact(tt, u.text(l10n.KeyAdvanced)) {
			t.Fatal("advanced box repeats the page title")
		}
		tt = ui.NewTester(func(c *ui.Context) { applyTheme(c); u.notificationsGroup(c) }, 600, 600)
		if hasExact(tt, u.text(l10n.KeyNotificationSettings)) {
			t.Fatal("notifications box repeats the page title")
		}
		tt = ui.NewTester(func(c *ui.Context) { applyTheme(c); u.modulesGroup(c) }, 600, 1800)
		if hasExact(tt, u.text(l10n.KeyPanelModules)) || hasExact(tt, u.text(l10n.KeyMainPanelModules)) {
			t.Fatal("panel modules box repeats the page title")
		}
		if !hasExact(tt, u.text(l10n.KeyMainPanelModulesDescription)) {
			t.Fatal("panel modules description missing")
		}
	}
}

func TestSettingsPageHeadingsKeepNormalTextColor(t *testing.T) {
	u := cardSettingsUI(t, store.LanguageEnglish)
	for _, dark := range []bool{false, true} {
		for _, tab := range settingsTabs {
			u.tab = tab
			tt := ui.NewTester(func(c *ui.Context) { applyTheme(c); u.pageHeader(c) }, 600, 64)
			tt.SetDark(dark)
			r, ok := tt.Find(tab.title(u.L()))
			if !ok || countHeaderColor(tt.Image(), r, buildTheme(dark).Text) < 3 {
				t.Fatalf("page heading %s does not use normal text color", tab.title(u.L()))
			}
		}
	}
}

// assertSectionTitle checks that a section title is plain text on the
// page, with its box below it.
func assertSectionTitle(t *testing.T, tt *ui.Tester, title string, dark bool) {
	t.Helper()
	r, ok := tt.Find(title)
	if !ok {
		t.Fatalf("missing section title %q", title)
	}
	theme := buildTheme(dark)
	img := tt.Image()
	if countHeaderColor(img, r, theme.Text) < 3 {
		t.Fatalf("section title %q does not use normal text color", title)
	}
	bg := theme.Background
	if img.RGBAAt(int(r.X)-6, int(r.Y+r.H/2)) != (color.RGBA{R: bg.R, G: bg.G, B: bg.B, A: 255}) {
		t.Fatalf("section title %q is not on the page background", title)
	}
	card := pal(theme).Card
	for y := int(r.Y + r.H); y < int(r.Y+r.H)+60; y++ {
		if img.RGBAAt(int(r.X)-6, y) == (color.RGBA{R: card.R, G: card.G, B: card.B, A: 255}) {
			return
		}
	}
	t.Fatalf("section title %q has no box below it", title)
}

func countHeaderColor(img *image.RGBA, r ui.Rect, want ui.Color) int {
	n := 0
	for y := int(r.Y); y < int(r.Y+r.H); y++ {
		for x := int(r.X); x < int(r.X+r.W); x++ {
			if img.RGBAAt(x, y) == (color.RGBA{R: want.R, G: want.G, B: want.B, A: 255}) {
				n++
			}
		}
	}
	return n
}

func assertCardPixel(t *testing.T, img *image.RGBA, x, y int, want ui.Color) {
	t.Helper()
	if got := img.RGBAAt(x, y); got != (color.RGBA{R: want.R, G: want.G, B: want.B, A: want.A}) {
		t.Fatalf("card background missing at (%d,%d): %v", x, y, got)
	}
}
