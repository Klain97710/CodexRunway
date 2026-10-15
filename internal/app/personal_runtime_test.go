package app

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/licoy/quosparr/internal/export"
	"github.com/licoy/quosparr/internal/personal"
	"github.com/licoy/quosparr/internal/store"
)

func TestPersonalRuntimeDoesNotConstructDisabledProviders(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GROK_HOME", filepath.Join(home, "forbidden-grok"))
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(home, "forbidden-claude"))
	t.Setenv("CODEX_HOME", filepath.Join(home, ".codex"))
	e := newEnv(t, nil)
	e.s.sys.Features = personal.Features{}
	rt := newRuntime(BackgroundOptions{Home: home, Settings: e.s})
	defer rt.stop()
	if rt.grok != nil || rt.grokMod != nil {
		t.Fatal("Grok resource constructors ran")
	}
	if rt.claudeB != nil || rt.claudeMod != nil {
		t.Fatal("Claude resource constructors ran")
	}
	if _, err := os.Stat(filepath.Join(home, "forbidden-claude")); !os.IsNotExist(err) {
		t.Fatal("Claude home touched")
	}
	if _, err := os.Stat(filepath.Join(home, "forbidden-grok")); !os.IsNotExist(err) {
		t.Fatal("Grok home touched")
	}
}

func TestPersonalDetailsKeepsLocalRefreshUntilBothWindowsClose(t *testing.T) {
	var transitions []bool
	actions := PanelActions{PanelShown: visiblePanelCounter(func(open bool) {
		transitions = append(transitions, open)
	})}
	popover, _ := newPanel(t, nil, panelFixture(), actions)
	details, _ := newPanel(t, nil, panelFixture(), actions)
	popover.panelOpened()
	details.panelOpened()
	popover.panelClosed()
	popover.panelClosed() // Duplicate close must not cancel the details window.
	if !slices.Equal(transitions, []bool{true}) {
		t.Fatalf("details still open, refresh transitions = %v", transitions)
	}
	popover.panelOpened()
	details.panelClosed()
	if !slices.Equal(transitions, []bool{true}) {
		t.Fatalf("popover still open, refresh transitions = %v", transitions)
	}
	popover.panelClosed()
	if !slices.Equal(transitions, []bool{true, false}) {
		t.Fatalf("both closed, refresh transitions = %v", transitions)
	}
}

func TestPersonalOldWidgetLinkNormalizesCodex(t *testing.T) {
	a, tt := newPanel(t, nil, panelFixture(), PanelActions{})
	a.settings.sys.Features = personal.Features{}
	for _, scope := range []export.ProviderScope{export.ScopeGrok, export.ScopeBoth, export.ScopeClaude, export.ScopeCodexClaude, export.ScopeGrokClaude} {
		if !a.OpenWidgetURL((export.DeepLink{Provider: scope, Section: export.SectionOverview}).URL()) {
			t.Fatal("old link rejected")
		}
		tt.Frame()
		if a.settings.Prefs().SelectedProvider != store.ProviderCodex || a.panelState.provider != store.ProviderCodex {
			t.Fatal("old link selected disabled provider")
		}
		if tt.HasText("Grok") || tt.HasText("Claude") {
			t.Fatal("disabled provider selector visible")
		}
	}
}
