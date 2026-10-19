package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/l10n"
)

func TestAccountEditorIsOverlayAndEscapeClearsDraft(t *testing.T) {
	for _, mode := range []string{"paste", modeCodexAPIAdd, modeClaudeAPIAdd, "alias", "note", "switch"} {
		t.Run(mode, func(t *testing.T) {
			e := newEnv(t, nil)
			if err := e.s.Load(); err != nil {
				t.Fatal(err)
			}
			u := newSettingsUI(e.s, SettingsOptions{})
			f := accountFormOf(u)
			tt := ui.NewTester(func(c *ui.Context) {
				applyTheme(c)
				ui.Column(c).Fill().Padding(20).Children(func() {
					accountEditor(c, u, f)
					ui.Text(c, "Account row")
				})
			}, 700, 550)
			before, _ := tt.Find("Account row")
			f.mode, f.paste, f.api.key, f.alias, f.note = mode, "draft", "draft-key", "draft alias", "draft note"
			tt.Frame()
			after, _ := tt.Find("Account row")
			if after != before {
				t.Fatalf("editor shifted account row instead of opening over it: %v -> %v", before, after)
			}
			if !tt.HasText(u.L().Text(l10n.KeyCancel)) {
				t.Fatal("modal controls missing")
			}
			tt.Key(0, ui.KeyEscape)
			if f.mode != "" || f.paste != "" || f.api.key != "" || f.alias != "" || f.note != "" {
				t.Fatal("closing modal retained form draft")
			}
		})
	}
}

func TestAccountModalKeepsPendingAndFailedDraft(t *testing.T) {
	e := newEnv(t, nil)
	if err := e.s.Load(); err != nil {
		t.Fatal(err)
	}
	u := newSettingsUI(e.s, SettingsOptions{})
	f := accountFormOf(u)
	f.mode, f.alias = "alias", "Personal"
	tt := ui.NewTester(func(c *ui.Context) { accountEditor(c, u, f) }, 700, 550)
	gate := make(chan struct{})
	u.accountEditAct(f, func() (string, bool) { <-gate; return "Could not save", false })
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	if f.mode != "alias" || f.alias != "Personal" || !f.busy {
		t.Fatal("pending modal lost its operation/draft")
	}
	close(gate)
	deadline := time.Now().Add(time.Second)
	for f.busy && time.Now().Before(deadline) {
		e.q.drain()
		time.Sleep(time.Millisecond)
	}
	tt.Frame()
	if f.busy || f.mode != "alias" || f.alias != "Personal" || !tt.HasText("Could not save") {
		t.Fatal("failure did not keep the editable draft and visible error")
	}
}

func TestAccountSelectionIsOverlay(t *testing.T) {
	e := newEnv(t, nil)
	if err := e.s.Load(); err != nil {
		t.Fatal(err)
	}
	u := newSettingsUI(e.s, SettingsOptions{})
	f := accountFormOf(u)
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Padding(20).Children(func() { accountSelectionEditor(c, u, f, nil); ui.Text(c, "Account list") })
	}, 700, 550)
	before, _ := tt.Find("Account list")
	f.selection = &accountSelection{export: true, rows: []accountChoice{{id: "a", name: "Personal", selected: true}}}
	tt.Frame()
	after, _ := tt.Find("Account list")
	if after != before {
		t.Fatalf("selection shifted account list: %v -> %v", before, after)
	}
	tt.Key(0, ui.KeyEscape)
	if f.selection != nil {
		t.Fatal("Escape retained import/export selection")
	}
}

func TestGrokAliasEditorDoesNotExpandCard(t *testing.T) {
	a := &App{}
	l := l10n.New(l10n.English)
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Padding(16).Children(func() {
			a.grokAliasEditor(c, l, "account", AccountActions{})
			ui.Text(c, "Account card")
		})
	}, 400, 500)
	before, _ := tt.Find("Account card")
	a.panelState.accounts.aliasID, a.panelState.accounts.alias = "account", "Personal"
	tt.Frame()
	after, _ := tt.Find("Account card")
	if before != after {
		t.Fatal("alias editing expanded the account card")
	}
	tt.Key(0, ui.KeyEscape)
	if a.panelState.accounts.aliasID != "" || a.panelState.accounts.alias != "" {
		t.Fatal("dismissed alias dialog retained draft")
	}
}
