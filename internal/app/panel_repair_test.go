package app

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/sessions"
)

func TestPanelRepairRequiresConfirmationAndRunsOnce(t *testing.T) {
	snap := panelFixture()
	snap.Codex.SessionReport = &sessions.Report{MissingIndexIDs: []string{"session-1"}, PlannedEntries: 1}
	snap.Codex.SessionText = "1 missing, 0 orphan, 0 duplicate"
	calls := 0
	l, st := l10n.New(l10n.English), &panelState{}
	tt := ui.NewTester(func(c *ui.Context) {
		applyTheme(c)
		ui.Column(c).Fill().Padding(16).Children(func() {
			first := true
			panelRepair(c, l, snap, PanelActions{RepairSessions: func() { calls++ }}, &first, st)
		})
	}, 400, 360)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true, TextScale: 1})
	click := func(key l10n.Key) {
		t.Helper()
		if err := tt.Click(l.Text(key)); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
	}
	click(l10n.KeyRepairIndex)
	if calls != 0 || !hasExact(tt, l.Text(l10n.KeyRepairConfirmTitle)) {
		t.Fatalf("repair must wait for confirmation: calls=%d popover=%q", calls, st.info)
	}
	click(l10n.KeyCancel)
	if calls != 0 || st.info != "" {
		t.Fatal("cancelling must close the confirmation without repairing")
	}
	click(l10n.KeyRepairIndex)
	click(l10n.KeyRepair)
	if calls != 1 || st.info != "" {
		t.Fatalf("confirmed repair calls = %d, want 1; popover = %q", calls, st.info)
	}
	tt.Frame()
	if calls != 1 || hasExact(tt, l.Text(l10n.KeyRepairConfirmTitle)) {
		t.Fatal("accepted repair must close the confirmation and run only once")
	}
}

func TestPanelRepairWithoutActionStaysDisabled(t *testing.T) {
	l, st := l10n.New(l10n.English), &panelState{}
	tt := ui.NewTester(func(c *ui.Context) {
		applyTheme(c)
		ui.Column(c).Fill().Padding(16).Children(func() {
			repairRow(c, l, PanelActions{}, st)
		})
	}, 400, 360)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true, TextScale: 1})
	if err := tt.Click(l.Text(l10n.KeyRepairIndex)); err != nil {
		t.Fatal(err)
	}
	if st.info != "" {
		t.Fatal("unavailable repair must not open the confirmation")
	}
	// An already open popover must also stay safe if its action disappears.
	st.info = "repair-confirm"
	tt.Frame()
	if err := tt.Click(l.Text(l10n.KeyRepair)); err != nil {
		t.Fatal(err)
	}
	if st.info != "repair-confirm" {
		t.Fatal("unavailable repair must not accept confirmation")
	}
}
