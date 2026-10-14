package app

import (
	"github.com/licoy/quosparr/internal/personal"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/store"
)

// openSettingsView returns a window-less settings view on tab, in English.
func openSettingsView(t *testing.T, e *env, tab settingsTab) (*settingsUI, *ui.Tester) {
	t.Helper()
	if err := e.s.Load(); err != nil {
		t.Log("load:", err)
	}
	_ = e.s.LoadProxy()
	e.s.Apply()
	u := newSettingsUI(e.s, SettingsOptions{Version: "9.9.9"})
	u.tab = tab
	tt := ui.NewTester(u.view, settingsWidth, settingsHeight)
	e.s.OnChange(tt.Frame)
	return u, tt
}

func TestSettingsViewPillsChangePreferences(t *testing.T) {
	e := newEnv(t, nil)
	_, tt := openSettingsView(t, e, tabDisplay)
	l := e.s.L()
	if err := tt.Click(l.Text(appearanceKey(store.AppearanceDark))); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(l.Text(panelBackgroundKey(store.PanelBackgroundOpaque))); err != nil {
		t.Fatal(err)
	}
	e.settle()
	got := e.stored()
	if got.Appearance != store.AppearanceDark || got.MainPanelBackgroundStyle != store.PanelBackgroundOpaque {
		t.Errorf("stored appearance %v, background %v", got.Appearance, got.MainPanelBackgroundStyle)
	}
	if last := e.sys.appearance[len(e.sys.appearance)-1]; last != store.AppearanceDark {
		t.Errorf("appearance not applied live: %v", e.sys.appearance)
	}
}

func TestSettingsViewLanguageChoiceRelabelsThePage(t *testing.T) {
	e := newEnv(t, nil)
	_, tt := openSettingsView(t, e, tabGeneral)
	en := e.s.L()
	if !tt.HasText(en.Text(l10n.KeyLanguage)) {
		t.Fatalf("no language row; texts %q", tt.Texts())
	}
	// Open the dropdown, which shows the current choice, and pick French.
	if err := tt.Click(l10n.MenuTitle("system", en.Language())); err != nil {
		t.Fatalf("open dropdown: %v; texts %q", err, tt.Texts())
	}
	if err := tt.Click(l10n.MenuTitle("french", en.Language())); err != nil {
		t.Fatalf("choose: %v; texts %q", err, tt.Texts())
	}
	tt.Frame()
	e.settle()
	if got := e.stored().Language; got != store.LanguageFrench {
		t.Fatalf("stored language = %v", got)
	}
	fr := e.s.L()
	if fr.Language() != l10n.French || !tt.HasText(fr.Text(l10n.KeyLanguage)) {
		t.Errorf("the page did not switch to French; texts %q", tt.Texts())
	}
}

func TestSettingsViewReadOnlyExplainsAndRefuses(t *testing.T) {
	e := newEnv(t, func(d *failDomain) { d.Put(store.KeyPreferences, []byte("{broken")) })
	_, tt := openSettingsView(t, e, tabDisplay)
	l := e.s.L()
	if !tt.HasText(tx(l, xSettingsUnreadable)) {
		t.Errorf("the problem is not shown; texts %q", tt.Texts())
	}
	// Whatever is clicked, nothing is saved.
	_ = tt.Click(l.Text(appearanceKey(store.AppearanceDark)))
	e.settle()
	if e.mem.Writes() != 0 {
		t.Error("a damaged config was written over")
	}
	if e.s.Prefs().Appearance != store.AppearanceSystem {
		t.Error("the read-only settings changed")
	}
}

func TestSettingsViewTabs(t *testing.T) {
	e := newEnv(t, nil)
	u, tt := openSettingsView(t, e, tabGeneral)
	l := e.s.L()
	for _, tab := range settingsTabs {
		if err := tt.Click(l.Text(tab.key())); err != nil {
			t.Fatalf("tab %v: %v", tab, err)
		}
		tt.Frame()
		if u.tab != tab {
			t.Errorf("clicked %v, on %v", tab, u.tab)
		}
	}
	if !tt.HasText("9.9.9") {
		t.Errorf("the About tab does not show the version; texts %q", tt.Texts())
	}
}

func TestSettingsViewExplainsUnsupportedPlatformSettings(t *testing.T) {
	e := newEnv(t, nil)
	e.s = NewSettings(e.st, System{Features: personal.Features{Grok: true}}, e.creds)
	_, tt := openSettingsView(t, e, tabDisplay)
	if !tt.HasText(tx(e.s.L(), xMacOnly)) {
		t.Errorf("no note on the status bar styles; texts %q", tt.Texts())
	}
}
