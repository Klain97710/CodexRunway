package app

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/codex"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/runway"
	"github.com/licoy/quosparr/internal/store"
)

func TestSettingsSidebarSelectionAndTitlebarInsets(t *testing.T) {
	e := newEnv(t, nil)
	u, tt := openSettingsView(t, e, tabGeneral)
	for _, bar := range []ui.TitleBar{{Height: 28, Left: 80}, {Height: 32, Right: 130}, {}} {
		tt.SetTitleBar(bar)
		top := float32(14)
		if bar.Left > 0 {
			top = 52
		}
		first, ok := tt.Find(tabGeneral.title(u.L()))
		if !ok || first.Y < top || first.X < 0 || first.X+first.W > settingsSidebarWidth {
			t.Fatalf("sidebar window controls %+v overlap first item %+v", bar, first)
		}
		for _, tab := range settingsTabs {
			if err := tt.Click(tab.title(u.L())); err != nil {
				t.Fatal(err)
			}
			if u.tab != tab {
				t.Fatalf("selected %v, want %v", u.tab, tab)
			}
			r, _ := tt.Find(tab.title(u.L()))
			pixel := tt.Image().RGBAAt(int(r.X+r.W-3), int(r.Y+r.H/2))
			expected := buildTheme(false).Accent
			if pixel.R != expected.R || pixel.G != expected.G || pixel.B != expected.B {
				t.Fatalf("selected sidebar item has no accent background: %v", pixel)
			}
			before := r
			// Content scrolling must keep the navigation fixed.
			tt.Scroll(settingsSidebarWidth+200, 300, 0, 500)
			r, ok = tt.Find(tab.title(u.L()))
			if !ok || r != before {
				t.Fatal("navigation moved with page content")
			}
		}
	}
}

func TestSettingsSidebarMinimumSizeTranslatedPages(t *testing.T) {
	for _, language := range []store.Language{store.LanguageEnglish, store.LanguageSimplifiedChinese, store.LanguageRussian} {
		for _, dark := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%t", language, dark), func(t *testing.T) {
				e := newEnv(t, nil)
				u, tt := openSettingsView(t, e, tabGeneral)
				e.s.SetLanguage(language)
				tt.SetSize(settingsMinWidth, settingsMinHeight)
				tt.SetDark(dark)
				tt.SetTitleBar(ui.TitleBar{Height: 28, Left: 80})
				for _, tab := range settingsTabs {
					u.tab = tab
					tt.Frame()
					for frame := 0; frame < 5; frame++ {
						for _, text := range tt.Texts() {
							r, ok := tt.Find(text)
							if !ok || r.X < -0.5 || r.X+r.W > settingsMinWidth+0.5 {
								t.Fatalf("page %v text %q extends beyond minimum window: %+v", tab, text, r)
							}
						}
						tt.Scroll(650, 450, 0, 350)
					}
				}
				e.settle()
			})
		}
	}
}

func TestSettingsSidebarWritesPreferencesAcrossPages(t *testing.T) {
	e := newEnv(t, nil)
	u, tt := openSettingsView(t, e, tabGeneral)
	l := u.L()
	for _, change := range []struct {
		tab   settingsTab
		label string
	}{
		{tabDisplay, l.Text(l10n.KeyAccountPrivacyMode)},
		{tabAdvanced, l.Text(l10n.KeyExportStatusJSON)},
		{tabAbout, l.Text(l10n.KeyAutomaticallyCheckForUpdates)},
	} {
		if err := tt.Click(change.tab.title(u.L())); err != nil {
			t.Fatal(err)
		}
		// Some row labels are static text, so click the corresponding switch at the right.
		var control ui.Rect
		if r, ok := tt.Find(change.label); ok {
			control = r
		} else {
			t.Fatal(change.label)
		}
		if change.tab == tabDisplay {
			clickAccountPrivacySwitch(t, u, tt)
		} else {
			tt.ClickAt(settingsWidth-48, control.Y+control.H/2)
		}
	}
	e.settle()
	p := e.stored()
	if !p.AccountPrivacyModeEnabled || !p.ExportsStatusJSON || p.AutomaticallyChecksForUpdates {
		t.Fatalf("sidebar page changes were not stored: privacy=%t export=%t updates=%t", p.AccountPrivacyModeEnabled, p.ExportsStatusJSON, p.AutomaticallyChecksForUpdates)
	}
}

func TestSettingsPageHeaderReservesRightWindowControls(t *testing.T) {
	e := newEnv(t, nil)
	u, _ := openSettingsView(t, e, tabGeneral)
	width := settingsMinWidth - settingsSidebarWidth
	for _, bar := range []ui.TitleBar{{Height: 32, Right: 130}, {Height: 64, Left: 80}, {}} {
		tt := ui.NewTester(func(c *ui.Context) { applyTheme(c); ui.Column(c).Fill().Children(func() { u.pageHeader(c) }) }, width, 100)
		tt.SetTitleBar(bar)
		r, ok := tt.Find(tabGeneral.title(u.L()))
		height := max(bar.Height, 52)
		if !ok || r.X != 24 || r.X+r.W > float32(width)-bar.Right-16 || r.Y < 0 || r.Y+r.H > height {
			t.Fatalf("header overlaps window controls %+v: %+v", bar, r)
		}
	}
}

func TestProviderPanelToolbarFitsWhileRefreshing(t *testing.T) {
	for _, language := range l10n.Languages() {
		snap := panelFixture()
		snap.Refresh.All = true
		a, tt := newPanel(t, func(p *store.Preferences) { p.Language = store.Language(language) }, snap, PanelActions{})
		for _, key := range []l10n.Key{l10n.KeyRefresh, l10n.KeySettings, l10n.KeyQuit} {
			r, ok := tt.Find(a.L().Text(key))
			if !ok || r.X < 0 || r.X+r.W > panelWidth {
				t.Fatalf("busy toolbar %s language %s outside panel: %+v", key, language, r)
			}
		}
	}
}

func TestSettingsSidebarProviderSelectionRespectsReadOnly(t *testing.T) {
	for _, readonly := range []bool{false, true} {
		t.Run(fmt.Sprint(readonly), func(t *testing.T) {
			e := newEnv(t, func(d *failDomain) {
				if readonly {
					d.Put(store.KeyPreferences, []byte("{broken"))
				}
			})
			_, tt := openSettingsView(t, e, tabDisplay)
			if err := tt.Click(e.s.L().Text(l10n.KeyProviderGrok)); err != nil {
				t.Fatal(err)
			}
			e.settle()
			expected := store.ProviderGrok
			if readonly {
				expected = store.ProviderCodex
			}
			if e.s.Prefs().SelectedProvider != expected {
				t.Fatal("provider selection did not honor readonly state")
			}
			if readonly && e.mem.Writes() != 0 {
				t.Fatal("readonly provider selection wrote settings")
			}
			if !readonly && e.stored().SelectedProvider != expected {
				t.Fatal("provider selection not persisted")
			}
		})
	}
}

func TestSettingsSystemProxyFormVisibility(t *testing.T) {
	e := newEnv(t, nil)
	u, tt := openSettingsView(t, e, tabGeneral)
	l := u.L()
	if u.draft.Mode != store.ProxyModeSystem {
		t.Fatalf("initial proxy mode = %q", u.draft.Mode)
	}
	for _, key := range []l10n.Key{l10n.KeyNetworkProxyHost, l10n.KeyNetworkProxyPort, l10n.KeyNetworkProxyAuthentication} {
		if hasExact(tt, l.Text(key)) {
			t.Fatalf("System mode shows %s", key)
		}
	}
	if err := tt.Click(l.Text(l10n.KeyNetworkProxyHTTP)); err != nil {
		t.Fatal(err)
	}
	if u.draft.Mode != store.ProxyModeHTTP {
		t.Fatalf("proxy selection did not change draft: %q", u.draft.Mode)
	}
	for _, key := range []l10n.Key{l10n.KeyNetworkProxyHost, l10n.KeyNetworkProxyPort, l10n.KeyNetworkProxyAuthentication} {
		if !hasExact(tt, l.Text(key)) {
			t.Fatalf("HTTP mode missing %s", key)
		}
	}
	if err := tt.Click(l.Text(l10n.KeyNetworkProxySystem)); err != nil {
		t.Fatal(err)
	}
	if hasExact(tt, l.Text(l10n.KeyNetworkProxyHost)) || hasExact(tt, l.Text(l10n.KeyNetworkProxyPort)) {
		t.Fatal("System mode retained HTTP fields")
	}
}

func TestManagedAccountCountSumsProviders(t *testing.T) {
	var snap runway.Snapshot
	if got := managedAccountCount(snap); got != 0 {
		t.Fatalf("empty snapshot counted %d accounts", got)
	}
	snap.Accounts.Sidebar = []codex.ManagedAccount{{ID: "a"}, {ID: "b"}}
	snap.Grok.Accounts.Sidebar = []codex.ManagedAccount{{ID: "grok-c"}}
	if got := managedAccountCount(snap); got != 3 {
		t.Fatalf("counted %d accounts, want 3", got)
	}
}

func TestSettingsTabBadgeOnlyOnAccounts(t *testing.T) {
	u := &settingsUI{}
	for _, tab := range settingsTabs {
		if tab != tabAccounts && u.tabBadge(tab) != "" {
			t.Fatalf("tab %d shows a badge", tab)
		}
	}
}

func TestPanelModulesHaveTheirOwnSidebarPage(t *testing.T) {
	e := newEnv(t, nil)
	u, tt := openSettingsView(t, e, tabDisplay)
	description := u.text(l10n.KeyMainPanelModulesDescription)
	if tt.HasText(description) {
		t.Fatal("display page still lists the panel modules")
	}
	if err := tt.Click(u.text(l10n.KeyPanelModules)); err != nil {
		t.Fatal(err)
	}
	if u.tab != tabModules || !tt.HasText(description) || !tt.HasText(u.text(moduleInfoOf(store.ModuleQuota).title)) {
		t.Fatalf("panel modules page missing; texts %q", tt.Texts())
	}
}

func TestNotificationsHaveTheirOwnSidebarPage(t *testing.T) {
	e := newEnv(t, nil)
	u, tt := openSettingsView(t, e, tabDisplay)
	alerts := u.text(l10n.KeyQuotaAlerts)
	if tt.HasText(alerts) {
		t.Fatal("appearance and menu page still lists the notifications")
	}
	if err := tt.Click(u.text(l10n.KeyNotificationSettings)); err != nil {
		t.Fatal(err)
	}
	if u.tab != tabNotifications || !tt.HasText(alerts) || !tt.HasText(u.text(l10n.KeyTestNotificationSubtitle)) {
		t.Fatalf("notifications page missing; texts %q", tt.Texts())
	}
}
