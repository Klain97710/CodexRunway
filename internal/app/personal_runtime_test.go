package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/licoy/quosparr/internal/export"
	"github.com/licoy/quosparr/internal/personal"
	"github.com/licoy/quosparr/internal/store"
)

func TestPersonalRuntimeDoesNotConstructGrok(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GROK_HOME", filepath.Join(home, "forbidden-grok"))
	t.Setenv("CODEX_HOME", filepath.Join(home, ".codex"))
	e := newEnv(t, nil)
	e.s.sys.Features = personal.Features{}
	rt := newRuntime(BackgroundOptions{Home: home, Settings: e.s})
	defer rt.stop()
	if rt.grok != nil || rt.grokMod != nil {
		t.Fatal("Grok resource constructors ran")
	}
	if _, err := os.Stat(filepath.Join(home, "forbidden-grok")); !os.IsNotExist(err) {
		t.Fatal("Grok home touched")
	}
}

func TestPersonalOldWidgetLinkNormalizesCodex(t *testing.T) {
	a, tt := newPanel(t, nil, panelFixture(), PanelActions{})
	a.settings.sys.Features = personal.Features{}
	for _, scope := range []export.ProviderScope{export.ScopeGrok, export.ScopeBoth} {
		if !a.OpenWidgetURL((export.DeepLink{Provider: scope, Section: export.SectionOverview}).URL()) {
			t.Fatal("old link rejected")
		}
		tt.Frame()
		if a.settings.Prefs().SelectedProvider != store.ProviderCodex || a.panelState.provider != store.ProviderCodex {
			t.Fatal("old link selected disabled provider")
		}
		if tt.HasText("Grok") {
			t.Fatal("disabled provider selector visible")
		}
	}
}
