package app

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/sessions"
	"github.com/licoy/quosparr/internal/store"
)

func TestSessionFilterIncludesEndDateAcrossDST(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	f, err := (sessionFilter{from: "2026-03-08", to: "2026-03-08"}).apply(loc)
	if err != nil {
		t.Fatal(err)
	}
	if got := f.end.Sub(f.start); got != 23*time.Hour {
		t.Fatalf("DST day duration = %v, want 23h", got)
	}
	inside := managedSession("inside", "inside", time.Date(2026, 3, 8, 23, 59, 0, 0, loc))
	after := managedSession("after", "after", time.Date(2026, 3, 9, 0, 0, 0, 0, loc))
	if !f.matches(inside) || f.matches(after) {
		t.Fatalf("end-date inclusion: inside=%t after=%t", f.matches(inside), f.matches(after))
	}
}

func TestSessionFilterValidationAndChangesClearSelection(t *testing.T) {
	s := settingsSessions{provider: store.ProviderCodex, selected: map[string]bool{"one": true}}
	s.draft = sessionFilter{from: "2026-10-18", to: "2026-10-17"}
	if s.applyFilter(time.Local) || s.filterErr == nil || len(s.selected) != 1 {
		t.Fatalf("invalid range applied or selection changed: err=%v selected=%v", s.filterErr, s.selected)
	}
	s.draft = sessionFilter{search: "release", from: "2026-10-17", to: "2026-10-18"}
	if !s.applyFilter(time.Local) || len(s.selected) != 0 || s.applied.search != "release" {
		t.Fatalf("valid filter not applied: %+v", s)
	}
	s.selected["two"] = true
	s.switchProvider(store.ProviderClaude)
	if len(s.selected) != 0 || s.provider != store.ProviderClaude {
		t.Fatalf("provider switch kept selection: %+v", s)
	}
}

func TestSessionQuickRangeKeepsDraftSearchAndProject(t *testing.T) {
	s := settingsSessions{selected: map[string]bool{"one": true}}
	s.draft = sessionFilter{search: "unsaved search", project: "cwd:/work/a", projectLabel: "Quosparr · /work/a"}
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.Local)
	s.setQuickRange(now, 7)
	if s.draft.search != "unsaved search" || s.draft.project != "cwd:/work/a" || s.applied.project != "cwd:/work/a" {
		t.Fatalf("quick range lost draft filters: draft=%+v applied=%+v", s.draft, s.applied)
	}
	if len(s.selected) != 0 || s.applied.from != "2026-10-12" || s.applied.to != "2026-10-18" {
		t.Fatalf("quick range state = selected=%v applied=%+v", s.selected, s.applied)
	}
}

func TestSessionProjectsDistinguishDuplicateNamesAndCombineFilters(t *testing.T) {
	loc := time.Local
	a := withProject(managedSession("a", "Release alpha", time.Date(2026, 10, 18, 12, 0, 0, 0, loc)), "Quosparr", "/work/a")
	b := withProject(managedSession("b", "Release beta", time.Date(2026, 10, 18, 13, 0, 0, 0, loc)), "Quosparr", "/work/b")
	projects := sessionProjects([]sessions.ManagedSession{b, a, a})
	if len(projects) != 2 || projects[0].label != "Quosparr · /work/a" || projects[1].label != "Quosparr · /work/b" {
		t.Fatalf("projects = %+v", projects)
	}
	f, err := (sessionFilter{search: "alpha", from: "2026-10-18", to: "2026-10-18", project: sessionProjectKey(a)}).apply(loc)
	if err != nil || !f.matches(a) || f.matches(b) {
		t.Fatalf("combined filter err=%v a=%t b=%t", err, f.matches(a), f.matches(b))
	}
}

func TestSessionCalendarOpensFromEmptyAndChoosesToday(t *testing.T) {
	u, tt, _ := settingsSessionsView(t, SessionActions{})
	if err := tt.Click(tx(u.L(), xSessionsStartDate)); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(tx(u.L(), xSessionsUseDate)) {
		t.Fatalf("calendar did not open; texts %q", tt.Texts())
	}
	if err := tt.Click(tx(u.L(), xSessionsUseDate)); err != nil {
		t.Fatal(err)
	}
	if want := time.Now().In(time.Local).Format(dateLayout); u.sessions.draft.from != want {
		t.Fatalf("empty calendar chose %q, want today %q", u.sessions.draft.from, want)
	}
	if err := tt.Click(tx(u.L(), xSessionsStartDate)); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyRight)
	if err := tt.Click(tx(u.L(), xSessionsUseDate)); err != nil {
		t.Fatal(err)
	}
	if want := time.Now().In(time.Local).AddDate(0, 0, 1).Format(dateLayout); u.sessions.draft.from != want {
		t.Fatalf("calendar arrow chose %q, want %q", u.sessions.draft.from, want)
	}
}

func TestSessionProjectSelectAndSelectAllUseCurrentFilter(t *testing.T) {
	u, tt, _ := settingsSessionsView(t, SessionActions{})
	u.sessions.items[0] = withProject(u.sessions.items[0], "Quosparr", "/work/a")
	u.sessions.items[1] = withProject(u.sessions.items[1], "Quosparr", "/work/b")
	tt.Frame()
	if err := tt.Click(tx(u.L(), xSessionsProjectFilter)); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("Quosparr · /work/a") || !tt.HasText("Quosparr · /work/b") {
		t.Fatalf("duplicate project choices missing; texts %q", tt.Texts())
	}
	if err := tt.Click("Quosparr · /work/a"); err != nil {
		t.Fatal(err)
	}
	if u.sessions.draft.project != "cwd:/work/a" || u.sessions.applied.project != "" {
		t.Fatalf("project applied too early: draft=%q applied=%q", u.sessions.draft.project, u.sessions.applied.project)
	}
	if err := tt.Click(tx(u.L(), xSessionsApply)); err != nil {
		t.Fatal(err)
	}
	if got := len(u.sessions.filtered()); got != 1 {
		t.Fatalf("project filter count = %d", got)
	}
	if err := tt.Click(tx(u.L(), xSessionsSelectFiltered, 1)); err != nil {
		t.Fatal(err)
	}
	if got := u.sessions.selectedItems(); len(got) != 1 || got[0].ID != "new" {
		t.Fatalf("selected outside current filter: %+v", got)
	}
	u.sessions.switchProvider(store.ProviderClaude)
	if u.sessions.draft.project != "" || u.sessions.applied.project != "" || u.sessions.draft.projectLabel != "" {
		t.Fatalf("provider switch kept project: draft=%+v applied=%+v", u.sessions.draft, u.sessions.applied)
	}
}

func TestSessionProjectPickerKeepsMissingSelectionLabel(t *testing.T) {
	u, tt, _ := settingsSessionsView(t, SessionActions{})
	u.sessions.draft.project = "cwd:/missing/project"
	u.sessions.draft.projectLabel = "Missing project · /missing/project"
	tt.Frame()
	if err := tt.Click(tx(u.L(), xSessionsProjectFilter)); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(u.sessions.draft.projectLabel) {
		t.Fatalf("missing selected project looks like all projects; texts %q", tt.Texts())
	}
	if err := tt.Click(tx(u.L(), xSessionsAllProjects)); err != nil {
		t.Fatal(err)
	}
	if u.sessions.draft.project != "" || u.sessions.draft.projectLabel != "" {
		t.Fatalf("all projects kept %q (%q)", u.sessions.draft.project, u.sessions.draft.projectLabel)
	}
}

func TestSessionRefreshPreservesDraftAndAppliedFilter(t *testing.T) {
	e := newEnv(t, nil)
	queue := detailMainQueue(t)
	u := newSettingsUI(e.s, SettingsOptions{})
	s := u.sessionsState()
	s.draft = sessionFilter{search: "draft", from: "2026-10-01", project: "cwd:/draft", projectLabel: "Draft"}
	s.applied = sessionFilter{search: "applied", to: "2026-10-18", project: "cwd:/applied", projectLabel: "Applied"}
	u.startSessionsScan(SessionActions{Query: func(context.Context, store.Provider) ([]sessions.ManagedSession, error) { return nil, nil }})
	finishDetail(t, queue)
	if s.draft.search != "draft" || s.draft.project != "cwd:/draft" || s.applied.search != "applied" || s.applied.project != "cwd:/applied" {
		t.Fatalf("refresh changed filters: draft=%+v applied=%+v", s.draft, s.applied)
	}
}

func TestSessionListScrollsUnderFixedFilters(t *testing.T) {
	u, tt, _ := settingsSessionsView(t, SessionActions{})
	for i := range 40 {
		u.sessions.items = append(u.sessions.items, managedSession(fmt.Sprint("extra", i), fmt.Sprint("Extra session ", i), time.Date(2026, 10, 1, 9, 0, 0, 0, time.Local)))
	}
	tt.Frame()
	apply, ok := tt.Find(tx(u.L(), xSessionsApply))
	row, rowOK := tt.Find("Review session management")
	if !ok || !rowOK {
		t.Fatalf("filters or list missing; texts %q", tt.Texts())
	}
	tt.Scroll(row.X+row.W/2, row.Y+row.H/2, 0, 400)
	tt.Frame()
	after, _ := tt.Find(tx(u.L(), xSessionsApply))
	if after.Y != apply.Y {
		t.Fatalf("filters scrolled from %v to %v", apply.Y, after.Y)
	}
	if moved, ok := tt.Find("Review session management"); ok && moved.Y == row.Y {
		t.Fatal("session list did not scroll")
	}
}

func TestSessionFilterPendingAndQuickRangeMatch(t *testing.T) {
	s := settingsSessions{selected: map[string]bool{}}
	if s.filterPending() {
		t.Fatal("empty filters pending")
	}
	s.draft.search = "  release "
	if !s.filterPending() || !s.applyFilter(time.Local) || s.filterPending() {
		t.Fatalf("pending around apply: draft=%+v applied=%+v", s.draft, s.applied)
	}
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.Local)
	for _, days := range []int{0, 1, 7, 30} {
		s.setQuickRange(now, days)
		if got, ok := s.applied.quickRange(now); !ok || got != days {
			t.Fatalf("quick range %d matched %d %t", days, got, ok)
		}
	}
	s.draft.from = "2026-10-01"
	s.applyFilter(time.Local)
	if _, ok := s.applied.quickRange(now); ok {
		t.Fatalf("custom range matched a quick range: %+v", s.applied)
	}
}

func TestSessionSelectAllBoxTogglesFilteredSessions(t *testing.T) {
	u, tt, _ := settingsSessionsView(t, SessionActions{})
	s := u.sessions
	s.selected[sessionKey(s.items[0])] = true
	tt.Frame()
	if err := tt.Click(tx(u.L(), xSessionsSelectFiltered, 2)); err != nil {
		t.Fatal(err)
	}
	if len(s.selectedItems()) != 2 {
		t.Fatalf("mixed box did not select all: %v", s.selected)
	}
	if err := tt.Click(tx(u.L(), xSessionsClearSelection)); err != nil {
		t.Fatal(err)
	}
	if len(s.selected) != 0 {
		t.Fatalf("checked box did not clear: %v", s.selected)
	}
}

func TestSessionRowDeleteTargetsOnlyItsSession(t *testing.T) {
	var deleted []sessions.ManagedSession
	actions := SessionActions{
		Query: func(context.Context, store.Provider) ([]sessions.ManagedSession, error) { return nil, nil },
		Delete: func(_ context.Context, items []sessions.ManagedSession) ([]sessions.DeleteResult, error) {
			deleted = items
			return []sessions.DeleteResult{{Session: items[0], Removed: 1, Total: 1}}, nil
		},
	}
	u, tt, queue := settingsSessionsView(t, actions)
	s := u.sessions
	keep := sessionKey(s.items[1])
	s.selected[keep] = true
	tt.Frame()
	if err := tt.Click(tx(u.L(), xSessionsDeleteOne)); err != nil {
		t.Fatal(err)
	}
	if !s.confirm || s.confirmKey != sessionKey(s.items[0]) || !tt.HasText(tx(u.L(), xSessionsDeleteConfirmMessage, "Codex", 1)) {
		t.Fatalf("row delete confirmation: confirm=%t key=%q texts %q", s.confirm, s.confirmKey, tt.Texts())
	}
	cancel, ok := tt.Find(u.text(l10n.KeyCancel))
	if !ok {
		t.Fatal("cancel button missing")
	}
	tt.ClickAt(cancel.X+cancel.W+8+20, cancel.Y+cancel.H/2)
	finishDetail(t, queue) // delete
	finishDetail(t, queue) // rescan
	if len(deleted) != 1 || deleted[0].ID != "new" || s.confirmKey != "" {
		t.Fatalf("deleted %+v, selection %v, key %q", deleted, s.selected, s.confirmKey)
	}
}

func TestSessionDetailHidesProviderTabs(t *testing.T) {
	u, tt, _ := settingsSessionsView(t, SessionActions{})
	u.sessions.open = sessionKey(u.sessions.items[0])
	tt.Frame()
	if tt.HasText("Claude") || !tt.HasText(u.text(l10n.KeyBack)) {
		t.Fatalf("detail page texts %q", tt.Texts())
	}
	if err := tt.Click(tx(u.L(), xSessionsBack)); err != nil {
		t.Fatal(err)
	}
	if u.sessions.open != "" || !tt.HasText("Claude") {
		t.Fatalf("back did not return to the list; texts %q", tt.Texts())
	}
}
