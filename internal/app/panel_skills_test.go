package app

import (
	"context"
	"errors"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/skills"
	"github.com/licoy/quosparr/internal/store"
)

const (
	skillsTestRoot   = "/home/u/.codex/skills"
	skillsTestShared = "/home/u/.agents/skills"
)

func skillsFixture() map[store.Provider]skills.Summary {
	user := func(name, root, label, desc string) skills.Skill {
		file := root + "/" + name + "/SKILL.md"
		return skills.Skill{
			ID: "codex\x00" + file, Provider: skills.ProviderCodex, Name: name, InvokeName: name, Description: desc,
			File: file, Dir: root + "/" + name, RealFile: file, Scope: skills.ScopeUser, Root: root, RootLabel: label,
			State: skills.StateOn, Deletable: true, Toggleable: true,
			Removal: skills.Removal{Path: root + "/" + name, Covers: []string{name}},
		}
	}
	review := user("review", skillsTestRoot, "~/.codex/skills", "Review a pull request")
	pdf := user("pdf", skillsTestShared, "~/.agents/skills", "Fill in PDF forms")
	pdf.SharedWith = []skills.Provider{skills.ProviderGrok}
	pdf.State, pdf.Reason = skills.StateOff, skills.ReasonConfigPath
	system := skills.Skill{
		ID: "codex\x00sys", Provider: skills.ProviderCodex, Name: "imagegen", InvokeName: "imagegen", Description: "Make images",
		File: skillsTestRoot + "/.system/imagegen/SKILL.md", Dir: skillsTestRoot + "/.system/imagegen", Scope: skills.ScopeSystem,
		Root: skillsTestRoot + "/.system", RootLabel: "~/.codex/skills/.system", State: skills.StateOn,
		Toggleable: false, ToggleBlock: skills.ReasonReadOnlyScope,
	}
	codexSum := skills.Summary{
		Provider: skills.ProviderCodex,
		Skills:   []skills.Skill{review, pdf, system},
		Issues:   []skills.Issue{{Kind: skills.IssueBrokenLink, Path: skillsTestRoot + "/gone", Name: "gone", Removable: true}},
		Roots: []skills.RootInfo{
			{Path: skillsTestRoot, Label: "~/.codex/skills", Scope: skills.ScopeUser, Exists: true, Count: 1},
			{Path: skillsTestShared, Label: "~/.agents/skills", Scope: skills.ScopeUser, Exists: true, Count: 1},
		},
	}
	return map[store.Provider]skills.Summary{store.ProviderCodex: codexSum}
}

// onlySkillsModule hides every module of the main panel but the skills.
func onlySkillsModule(p *store.Preferences) {
	for _, m := range store.Modules() {
		if v := moduleShown(p, m); v != nil {
			*v = m == store.ModuleSkills
		}
	}
}

type skillsCalls struct {
	queries  int
	enabled  []bool
	deleted  []string
	links    []string
	revealed []string
}

func skillsPanel(t *testing.T, res map[store.Provider]skills.Summary) (*App, *ui.Tester, chan func(), *skillsCalls) {
	t.Helper()
	queue := detailMainQueue(t)
	calls := &skillsCalls{}
	acts := PanelActions{Skills: SkillActions{
		Query: func(context.Context) (map[store.Provider]skills.Summary, error) {
			calls.queries++
			return res, nil
		},
		ReadBody:   func(skills.Skill) (string, bool, error) { return "# Steps\n\nRead the diff.", false, nil },
		SetEnabled: func(_ skills.Skill, on bool) error { calls.enabled = append(calls.enabled, on); return nil },
		Delete: func(s skills.Skill, _ []skills.RootInfo) (bool, error) {
			calls.deleted = append(calls.deleted, s.Removal.Path)
			return false, nil
		},
		DeleteLink: func(_ skills.Summary, p string) (bool, error) {
			calls.links = append(calls.links, p)
			return false, nil
		},
		OpenFolder: func(string) error { return nil },
		Reveal:     func(p string) { calls.revealed = append(calls.revealed, p) },
	}}
	a, tt := newPanel(t, onlySkillsModule, panelFixture(), acts)
	finishDetail(t, queue) // the scan the module card starts
	tt.Frame()
	return a, tt, queue, calls
}

func TestPanelSkillsCardSummarizesAndOpensPage(t *testing.T) {
	l := l10n.New(l10n.English)
	a, tt, _, calls := skillsPanel(t, skillsFixture())
	for _, text := range []string{tx(l, xSkillsTitle), tx(l, xSkillsSummary, 3, 1), tx(l, xSkillsIssueCount, 1), tx(l, xSkillsManage)} {
		if !hasExact(tt, text) {
			t.Fatalf("skills card missing %q; texts %q", text, tt.Texts())
		}
	}
	if calls.queries != 1 {
		t.Fatalf("queries = %d, want one scan", calls.queries)
	}
	if err := tt.Click(tx(l, xSkillsManage)); err != nil {
		t.Fatal(err)
	}
	if a.panelState.detail != store.ModuleSkills {
		t.Fatalf("detail = %q", a.panelState.detail)
	}
	for _, text := range []string{"~/.codex/skills", "~/.agents/skills", tx(l, xSkillsScopeSystem), "review", "pdf", "imagegen", tx(l, xSkillsStateOff), tx(l, xSkillsIssuesTitle), tx(l, xSkillsIssueBrokenLink), "~/.codex/skills/gone"} {
		if !hasExact(tt, text) {
			t.Fatalf("skills page missing %q; texts %q", text, tt.Texts())
		}
	}
	if calls.queries != 1 {
		t.Fatalf("a fresh scan was repeated: %d", calls.queries)
	}
}

func TestPanelSkillsCardHiddenWhenPreferenceOff(t *testing.T) {
	l := l10n.New(l10n.English)
	_, tt := newPanel(t, func(p *store.Preferences) { p.ShowsSkills = false }, panelFixture(), PanelActions{})
	if tt.HasText(tx(l, xSkillsTitle)) {
		t.Fatalf("skills card still shown; texts %q", tt.Texts())
	}
}

func TestPanelSkillsSearchFilters(t *testing.T) {
	l := l10n.New(l10n.English)
	a, tt, _, _ := skillsPanel(t, skillsFixture())
	a.panelState.detail = store.ModuleSkills
	a.panelState.skills.search = "no such skill"
	tt.Frame()
	if !hasExact(tt, tx(l, xSkillsNoMatch)) {
		t.Fatalf("no-match text missing; texts %q", tt.Texts())
	}
	a.panelState.skills.search = "PDF"
	tt.Frame()
	if !hasExact(tt, "pdf") || hasExact(tt, "review") || hasExact(tt, "imagegen") {
		t.Fatalf("search did not filter; texts %q", tt.Texts())
	}
}

func TestPanelSkillItemPageToggleRevealAndBack(t *testing.T) {
	l := l10n.New(l10n.English)
	a, tt, queue, calls := skillsPanel(t, skillsFixture())
	a.panelState.detail = store.ModuleSkills
	tt.Frame()
	if err := tt.Click("pdf"); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"Fill in PDF forms", tx(l, xSkillsReasonConfigPath), tx(l, xSkillsEnabled), tx(l, xSkillsToggleCodex), "~/.agents/skills/pdf/SKILL.md", "Grok"} {
		if !hasExact(tt, text) {
			t.Fatalf("item page missing %q; texts %q", text, tt.Texts())
		}
	}
	if a.panelState.skills.enabled {
		t.Fatal("the switch of an off skill is on")
	}
	// The switch is at the right end of the row its label starts.
	label, _ := tt.Find(tx(l, xSkillsEnabled))
	tt.ClickAt(panelWidth-16-10-14, label.Y+label.H/2)
	finishDetail(t, queue) // the write
	finishDetail(t, queue) // the rescan after it
	if len(calls.enabled) != 1 || !calls.enabled[0] {
		t.Fatalf("SetEnabled calls = %v, want [true]", calls.enabled)
	}
	tt.Frame()
	if err := tt.Click(tx(l, xSkillsReveal)); err != nil {
		t.Fatal(err)
	}
	if len(calls.revealed) != 1 || calls.revealed[0] != skillsTestShared+"/pdf/SKILL.md" {
		t.Fatalf("revealed %v", calls.revealed)
	}
	for _, label := range []string{tx(l, xSkillsOpenFolder), tx(l, xSkillsReveal), tx(l, xSkillsCopyPath), tx(l, xSkillsDelete)} {
		assertSkillIconButton(t, tt, label)
	}
	if err := tt.Click(l.Text(l10n.KeyBack)); err != nil {
		t.Fatal(err)
	}
	if a.panelState.detail != store.ModuleSkills || a.panelState.skills.open != "" {
		t.Fatal("Back on a skill page did not return to the list")
	}
	if err := tt.Click(l.Text(l10n.KeyBack)); err != nil {
		t.Fatal(err)
	}
	if a.panelState.detail != "" {
		t.Fatal("Back on the list did not leave the page")
	}
}

func TestPanelSkillItemReadOnlyHasNoSwitchOrTrash(t *testing.T) {
	l := l10n.New(l10n.English)
	a, tt, _, _ := skillsPanel(t, skillsFixture())
	a.panelState.detail = store.ModuleSkills
	a.panelState.skills.open = "codex\x00sys"
	tt.Frame()
	if !hasExact(tt, tx(l, xSkillsReadOnlySystem)) || !hasExact(tt, tx(l, xSkillsBlockReadOnly)) {
		t.Fatalf("read-only hints missing; texts %q", tt.Texts())
	}
	if hasExact(tt, tx(l, xSkillsDelete)) || hasExact(tt, tx(l, xSkillsEnabled)) {
		t.Fatalf("a system skill offers delete or the switch; texts %q", tt.Texts())
	}
}

func TestPanelSkillDeleteConfirmsThenTrashes(t *testing.T) {
	l := l10n.New(l10n.English)
	a, tt, queue, calls := skillsPanel(t, skillsFixture())
	a.panelState.detail = store.ModuleSkills
	a.panelState.skills.open = "codex\x00" + skillsTestShared + "/pdf/SKILL.md"
	tt.Frame()
	if err := tt.Click(tx(l, xSkillsDelete)); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	for _, text := range []string{tx(l, xSkillsDeleteTitle), tx(l, xSkillsDeleteShared, "Grok"), tx(l, xSkillsDeleteRestart)} {
		if !hasExact(tt, text) {
			t.Fatalf("confirmation missing %q; texts %q", text, tt.Texts())
		}
	}
	if len(calls.deleted) != 0 {
		t.Fatal("deleted before the confirmation")
	}
	// The confirm button shares the trash button's label; it sits right of
	// the popover's Cancel.
	cancel, ok := tt.Find(l.Text(l10n.KeyCancel))
	if !ok {
		t.Fatalf("Cancel missing; texts %q", tt.Texts())
	}
	tt.ClickAt(cancel.X+cancel.W+8+20, cancel.Y+cancel.H/2)
	finishDetail(t, queue)
	finishDetail(t, queue)
	tt.Frame()
	if len(calls.deleted) != 1 || calls.deleted[0] != skillsTestShared+"/pdf" {
		t.Fatalf("deleted %v", calls.deleted)
	}
	if a.panelState.skills.open != "" || !hasExact(tt, tx(l, xSkillsMovedTrash)) {
		t.Fatalf("after delete: open=%q texts %q", a.panelState.skills.open, tt.Texts())
	}
}

func TestPanelSkillsScanFailureWaitsForRefresh(t *testing.T) {
	l := l10n.New(l10n.English)
	queue := detailMainQueue(t)
	n := 0
	acts := PanelActions{Skills: SkillActions{Query: func(context.Context) (map[store.Provider]skills.Summary, error) {
		n++
		return nil, errors.New("boom")
	}}}
	a, tt := newPanel(t, onlySkillsModule, panelFixture(), acts)
	finishDetail(t, queue)
	tt.Frame()
	tt.Frame()
	if n != 1 || !hasExact(tt, tx(l, xSkillsScanFailed)) {
		t.Fatalf("scans = %d; texts %q", n, tt.Texts())
	}
	a.startSkillsQuery(acts)
	finishDetail(t, queue)
	if n != 2 {
		t.Fatalf("refresh did not scan again: %d", n)
	}
}

func TestSkillSwitchableFollowsReason(t *testing.T) {
	for _, test := range []struct {
		state  skills.State
		reason skills.Reason
		want   bool
	}{
		{skills.StateOn, skills.ReasonNone, true},
		{skills.StateOff, skills.ReasonConfigPath, true},
		{skills.StateOff, skills.ReasonOverride, true},
		{skills.StateOff, skills.ReasonDisabledList, true},
		{skills.StateOff, skills.ReasonBundledOff, false},
		{skills.StateOff, skills.ReasonCompatOff, false},
		{skills.StateOff, skills.ReasonPluginOff, false},
		{skills.StateOverridden, skills.ReasonShadowed, true},
	} {
		sk := skills.Skill{Toggleable: true, State: test.state, Reason: test.reason}
		if got := skillSwitchable(sk); got != test.want {
			t.Errorf("%s/%s: got %v", test.state, test.reason, got)
		}
	}
	if skillSwitchable(skills.Skill{State: skills.StateOn}) {
		t.Error("a skill the provider cannot toggle is switchable")
	}
}

func TestSkillPathUsesLongestRootLabel(t *testing.T) {
	sum := skillsFixture()[store.ProviderCodex]
	sum.Roots = append(sum.Roots, skills.RootInfo{Path: skillsTestRoot + "/.system", Label: "~/.codex/skills/.system"})
	for in, want := range map[string]string{
		skillsTestRoot + "/review/SKILL.md":           "~/.codex/skills/review/SKILL.md",
		skillsTestRoot + "/.system/imagegen/SKILL.md": "~/.codex/skills/.system/imagegen/SKILL.md",
		skillsTestRoot + "-other/x":                   skillsTestRoot + "-other/x",
		"/etc/codex/skills/a":                         "/etc/codex/skills/a",
	} {
		if got := skillPath(sum, in); got != want {
			t.Errorf("skillPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPanelSkillContentLoadsOnDemand(t *testing.T) {
	l := l10n.New(l10n.English)
	a, tt, queue, _ := skillsPanel(t, skillsFixture())
	a.panelState.detail = store.ModuleSkills
	a.panelState.skills.open = "codex\x00" + skillsTestRoot + "/review/SKILL.md"
	tt.Frame()
	if tt.HasText("Read the diff.") {
		t.Fatal("SKILL.md was shown before it was asked for")
	}
	if err := tt.Click(tx(l, xSkillsShowContent)); err != nil {
		t.Fatal(err)
	}
	finishDetail(t, queue)
	tt.Frame()
	if !tt.HasText("Steps") || !tt.HasText("Read the diff.") || !hasExact(tt, tx(l, xSkillsHideContent)) {
		t.Fatalf("content missing; texts %q", tt.Texts())
	}
	if err := tt.Click(tx(l, xSkillsHideContent)); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if tt.HasText("Read the diff.") {
		t.Fatal("Hide did not collapse the content")
	}
	if err := tt.Click(tx(l, xSkillsShowContent)); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("Read the diff.") {
		t.Fatal("showing again did not reuse the loaded text")
	}
}
