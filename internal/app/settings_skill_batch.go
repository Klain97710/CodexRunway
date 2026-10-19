package app

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/skills"
)

// The skills list can select skills to delete or fix together. A batch
// confirms once, listing what it changes and every risk, then runs one
// skill at a time and reports what failed.

// skillBatchFix is one fix of a batch.
type skillBatchFix struct {
	sk  skills.Skill
	is  skills.Issue
	fix skills.Fix
}

// skillBatchError is a batch that failed part of the way.
type skillBatchError struct {
	done, failed int
	first        error
}

func (e *skillBatchError) Error() string { return e.first.Error() }

const (
	skillBatchFixConfirm    = "batch\x00fix"
	skillBatchDeleteConfirm = "batch\x00delete"
)

// skillSelectable is whether a skill's box can be checked: it can go to
// the trash or has a fix.
func skillSelectable(acts SkillActions, sum skills.Summary, sk skills.Skill) bool {
	if sk.Deletable && acts.Delete != nil {
		return true
	}
	_, _, ok := skillFixFor(acts, sum, sk)
	return ok
}

// skillPicked are the provider's selected skills that can still be
// selected after the last scan.
func (s *settingsSkills) skillPicked(acts SkillActions, sum skills.Summary) []skills.Skill {
	var out []skills.Skill
	for _, sk := range sum.Skills {
		if s.selected[sk.ID] && skillSelectable(acts, sum, sk) {
			out = append(out, sk)
		}
	}
	return out
}

// skillBatchFixes are the fixes for picked skills, each skill's first;
// skills lifted with one outer folder are lifted once.
func skillBatchFixes(acts SkillActions, sum skills.Summary, picked []skills.Skill) (out []skillBatchFix, fixable int) {
	wrappers := map[string]bool{}
	for _, sk := range picked {
		is, fixes, ok := skillFixFor(acts, sum, sk)
		if !ok || fixes[0] == skills.FixAddDescription && acts.SuggestDescription == nil {
			continue
		}
		fixable++
		if fixes[0] == skills.FixLiftNested {
			if wrappers[is.Lift.Wrapper] {
				continue
			}
			wrappers[is.Lift.Wrapper] = true
		}
		out = append(out, skillBatchFix{sk: sk, is: is, fix: fixes[0]})
	}
	return out, fixable
}

// skillBatchDeletes are the picked skills that can go to the trash, once
// per folder: a skill inside a folder already going is left out.
func skillBatchDeletes(acts SkillActions, picked []skills.Skill) []skills.Skill {
	if acts.Delete == nil {
		return nil
	}
	var out []skills.Skill
	for _, sk := range picked {
		if sk.Deletable && sk.Removal.Path != "" {
			out = append(out, sk)
		}
	}
	slices.SortStableFunc(out, func(a, b skills.Skill) int { return len(a.Removal.Path) - len(b.Removal.Path) })
	kept := out[:0]
	for _, sk := range out {
		inside := slices.ContainsFunc(kept, func(k skills.Skill) bool { return pathInside(k.Removal.Path, sk.Removal.Path) })
		if !inside {
			kept = append(kept, sk)
		}
	}
	return kept
}

// pathInside is whether path is dir or below it.
func pathInside(dir, path string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// skillsSelectBar is the select-all box over the list, the counts and the
// batch buttons.
func (u *settingsUI) skillsSelectBar(c *ui.Context, sum skills.Summary, list []skills.Skill, acts SkillActions) {
	s := u.skillsState()
	l, t := u.L(), c.Theme()
	selectable, all := 0, true
	for _, sk := range list {
		if skillSelectable(acts, sum, sk) {
			selectable++
			all = all && s.selected[sk.ID]
		}
	}
	all = all && selectable > 0
	picked := s.skillPicked(acts, sum)
	busy := s.work.Running
	listBar(c).Children(func() {
		label := tx(l, xSkillsSelectFiltered, selectable)
		if all {
			label = tx(l, xSkillsClearSelection)
		}
		if mixedCheckbox(c, "skills-select-all", all, len(picked) > 0, label, busy || selectable == 0) {
			s.selected = map[string]bool{}
			if !all {
				for _, sk := range list {
					if skillSelectable(acts, sum, sk) {
						s.selected[sk.ID] = true
					}
				}
			}
		}
		ui.Text(c, tx(l, xSkillsSelectionCounts, len(list), len(picked))).FontSize(12).FontFeatures("tnum").TextColor(t.TextMuted).
			SingleLine().Grow(1).Basis(0)
		u.skillBatchFixButton(c, sum, picked, acts)
		u.skillBatchDeleteButton(c, sum, picked, acts)
	})
}

func (u *settingsUI) skillBatchFixButton(c *ui.Context, sum skills.Summary, picked []skills.Skill, acts SkillActions) {
	s := u.skillsState()
	l := u.L()
	items, fixable := skillBatchFixes(acts, sum, picked)
	b := SmallPillButton(c.Key("skills-batch-fix"), icToolbox, tx(l, xSkillsFixSelected, fixable), s.work.Running || len(items) == 0).
		Cursor(ui.CursorPointer)
	open := s.confirm == skillBatchFixConfirm
	if b.Clicked() && len(items) > 0 && !s.work.Running {
		s.confirm, open = skillBatchFixConfirm, true
	}
	panelPopover(c, b, &open, func() {
		ui.Column(c).Gap(12).Width(360).Padding(4).Children(func() {
			ui.Text(c, tx(l, xSkillsBatchFixTitle, fixable)).FontSize(fsTitle3).FontWeight(600)
			calloutDim(c, tx(l, xSkillsBatchFixMsg))
			skillBatchFixCounts(c, l, items)
			if skipped := len(picked) - fixable; skipped > 0 {
				calloutDim(c, tx(l, xSkillsBatchSkipFix, skipped))
			}
			if slices.ContainsFunc(items, func(it skillBatchFix) bool { return it.fix == skills.FixAddDescription }) {
				calloutDim(c, tx(l, xSkillsBatchDescNote))
			}
			var risks []string
			for _, it := range items {
				risks = append(risks, skillFixRisks(l, sum, it.is, it.sk, it.fix)...)
			}
			u.skillFixRiskBox(c, l, skillUniqueRisks(risks))
			u.skillBatchButtons(c, l, &open, tx(l, xSkillsFix), func() { u.runSkillBatchFix(acts, sum, items) })
		})
	})
	if !open && s.confirm == skillBatchFixConfirm {
		s.confirm = ""
	}
}

// skillBatchFixCounts lists how many skills each kind of fix changes.
func skillBatchFixCounts(c *ui.Context, l l10n.Localizer, items []skillBatchFix) {
	t := c.Theme()
	order := []skills.Fix{skills.FixLiftNested, skills.FixDisableCopy, skills.FixTrashCopy, skills.FixAddDescription}
	ui.Column(c).Gap(4).Children(func() {
		for _, f := range order {
			n := 0
			for _, it := range items {
				switch {
				case it.fix != f:
				case f == skills.FixLiftNested:
					n += len(it.is.Lift.Lifts)
				default:
					n++
				}
			}
			if n == 0 {
				continue
			}
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "•").FontSize(12).TextColor(t.TextMuted)
				ui.Text(c, skillFixAction(l, f)).FontSize(12).Grow(1).Basis(0)
				ui.Text(c, fmt.Sprint(n)).FontSize(12).FontWeight(600).FontFeatures("tnum")
			})
		}
	})
}

func (u *settingsUI) skillBatchDeleteButton(c *ui.Context, sum skills.Summary, picked []skills.Skill, acts SkillActions) {
	s := u.skillsState()
	l := u.L()
	items := skillBatchDeletes(acts, picked)
	deletable := 0
	for _, sk := range picked {
		if sk.Deletable {
			deletable++
		}
	}
	b := SmallPillButton(c.Key("skills-batch-delete"), icTrash, tx(l, xSkillsDeleteSelected, deletable), s.work.Running || len(items) == 0).
		Cursor(ui.CursorPointer)
	open := s.confirm == skillBatchDeleteConfirm
	if b.Clicked() && len(items) > 0 && !s.work.Running {
		s.confirm, open = skillBatchDeleteConfirm, true
	}
	panelPopover(c, b, &open, func() {
		ui.Column(c).Gap(12).Width(360).Padding(4).Children(func() {
			ui.Text(c, tx(l, xSkillsBatchDeleteTitle, deletable)).FontSize(fsTitle3).FontWeight(600)
			calloutDim(c, tx(l, xSkillsBatchDeleteMsg))
			if skipped := len(picked) - deletable; skipped > 0 {
				calloutDim(c, tx(l, xSkillsBatchSkipDelete, skipped))
			}
			var risks []string
			for _, sk := range items {
				if sk.Removal.IsLink {
					risks = append(risks, tx(l, xSkillsDeleteLink))
				}
				if len(sk.Removal.Covers) > 1 {
					risks = append(risks, tx(l, xSkillsDeleteCovers, skillNameList(sk.Removal.Covers)))
				}
				if len(sk.SharedWith) > 0 {
					risks = append(risks, tx(l, xSkillsDeleteShared, skillProviders(l, sk.SharedWith)))
				}
			}
			risks = append(risks, tx(l, xSkillsDeleteRestart))
			u.skillFixRiskBox(c, l, skillUniqueRisks(risks))
			u.skillBatchButtons(c, l, &open, tx(l, xSkillsDelete), func() { u.runSkillBatchDelete(acts, sum, items) })
		})
	})
	if !open && s.confirm == skillBatchDeleteConfirm {
		s.confirm = ""
	}
}

// skillUniqueRisks drops repeated lines and keeps the first ten.
func skillUniqueRisks(risks []string) []string {
	var out []string
	for _, r := range risks {
		if !slices.Contains(out, r) {
			out = append(out, r)
		}
	}
	if len(out) > 10 {
		out = append(out[:10], "…")
	}
	return out
}

func (u *settingsUI) skillBatchButtons(c *ui.Context, l l10n.Localizer, open *bool, action string, run func()) {
	ui.Row(c).Gap(8).Children(func() {
		ui.Spacer(c).Grow(1)
		if PillButton(c, nil, l.Text(l10n.KeyCancel), false).Clicked() {
			*open = false
		}
		if PillButton(c, nil, action, false).Clicked() {
			*open = false
			run()
		}
	})
}

func (u *settingsUI) runSkillBatchFix(acts SkillActions, sum skills.Summary, items []skillBatchFix) {
	l := u.L()
	s := u.skillsState()
	u.skillWork(acts, func() (string, error) {
		var batch skillBatchError
		for _, it := range items {
			desc := ""
			if it.fix == skills.FixAddDescription {
				desc = acts.SuggestDescription(it.sk)
			}
			if _, err := applySkillFix(l, acts, sum, it.is, it.sk, it.fix, desc); err != nil {
				batch.failed++
				if batch.first == nil {
					batch.first = err
				}
				continue
			}
			batch.done++
		}
		return tx(l, xSkillsBatchFixed, batch.done), batch.err()
	}, xSkillsBatchFailed, func(error) { s.selected = nil })
}

func (u *settingsUI) runSkillBatchDelete(acts SkillActions, sum skills.Summary, items []skills.Skill) {
	l := u.L()
	s := u.skillsState()
	u.skillWork(acts, func() (string, error) {
		var batch skillBatchError
		for _, sk := range items {
			if _, err := acts.Delete(sk, sum.Roots); err != nil {
				batch.failed++
				if batch.first == nil {
					batch.first = err
				}
				continue
			}
			batch.done++
		}
		return tx(l, xSkillsBatchDeleted, batch.done), batch.err()
	}, xSkillsBatchFailed, func(error) { s.selected = nil })
}

func (e *skillBatchError) err() error {
	if e.failed == 0 {
		return nil
	}
	return e
}
