package app

import (
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/skills"
)

// skillIssueFix is an issue's Fix button. It opens a confirmation that says
// what the fix changes and what it risks before anything is written; an
// issue with two fixes chooses between them there. place keeps the buttons
// for one issue in the list, on the skill's page and in the diagnostics
// apart.
func (u *settingsUI) skillIssueFix(c *ui.Context, place string, sum skills.Summary, is skills.Issue, acts SkillActions) {
	fixes := availableSkillFixes(acts, is)
	if len(fixes) == 0 {
		return
	}
	s := u.skillsState()
	l := u.L()
	sk, _ := skillByFile(sum, is.Path)
	id := "fix\x00" + place + "\x00" + string(is.Kind) + "\x00" + is.Path
	b := SmallPillButton(c.Key("skill-fix-"+place+"-"+is.Path), icToolbox, tx(l, xSkillsFix), s.work.Running).Cursor(ui.CursorPointer)
	open := s.confirm == id
	if b.Clicked() && !s.work.Running {
		s.confirm, open, s.fixChoice, s.fixDesc = id, true, 0, ""
		if fixes[0] == skills.FixAddDescription && acts.SuggestDescription != nil {
			s.fixDesc = acts.SuggestDescription(sk)
		}
	}
	fix := fixes[min(max(s.fixChoice, 0), len(fixes)-1)]
	panelPopover(c, b, &open, func() {
		ui.Column(c).Gap(12).Width(340).Padding(4).Children(func() {
			ui.Text(c, tx(l, xSkillsFixTitle, valueOr(is.Name, filepath.Base(filepath.Dir(is.Path))))).FontSize(fsTitle3).FontWeight(600)
			ui.Text(c, skillIssueText(l, is.Kind)).FontSize(12).TextColor(c.Theme().Warning)
			if len(fixes) > 1 {
				labels := make([]string, len(fixes))
				for i, f := range fixes {
					labels[i] = skillFixAction(l, f)
				}
				PillTabs(c.Key("skill-fix-choice"), &s.fixChoice, false, labels...)
			}
			calloutDim(c, skillFixMessage(l, sum, is, sk, fix))
			if fix == skills.FixAddDescription {
				ui.TextInput(c.Key("skill-fix-desc"), &s.fixDesc).Label(tx(l, xSkillsFixDescLabel)).FontSize(12).Height(listControlHeight)
			}
			if risks := skillFixRisks(l, sum, is, sk, fix); len(risks) > 0 {
				u.skillFixRiskBox(c, l, risks)
			}
			ui.Row(c).Gap(8).Children(func() {
				ui.Spacer(c).Grow(1)
				if PillButton(c, nil, l.Text(l10n.KeyCancel), false).Clicked() {
					open = false
				}
				empty := fix == skills.FixAddDescription && strings.TrimSpace(s.fixDesc) == ""
				if PillButton(c, nil, skillFixAction(l, fix), empty).Clicked() && !empty {
					open = false
					u.runSkillFix(acts, sum, is, sk, fix)
				}
			})
		})
	})
	if !open && s.confirm == id {
		s.confirm = ""
	}
}

// availableSkillFixes are the issue's fixes this build can carry out.
func availableSkillFixes(acts SkillActions, is skills.Issue) []skills.Fix {
	var out []skills.Fix
	for _, f := range is.Fixes {
		ok := false
		switch f {
		case skills.FixRemoveLink:
			ok = acts.DeleteLink != nil
		case skills.FixLiftNested:
			ok = acts.LiftNested != nil
		case skills.FixDisableCopy:
			ok = acts.SetEnabled != nil
		case skills.FixTrashCopy:
			ok = acts.Delete != nil
		case skills.FixAddDescription:
			ok = acts.AddDescription != nil
		}
		if ok {
			out = append(out, f)
		}
	}
	return out
}

// skillFixFor is the issue a skill is fixed for from its row and in a
// batch: the first of its own issues with a fix, nesting first, then
// duplicates, then a missing description.
func skillFixFor(acts SkillActions, sum skills.Summary, sk skills.Skill) (skills.Issue, []skills.Fix, bool) {
	var best skills.Issue
	var fixes []skills.Fix
	rank := func(k skills.IssueKind) int {
		switch k {
		case skills.IssueNested:
			return 0
		case skills.IssueDuplicateName:
			return 1
		}
		return 2
	}
	for _, is := range sum.Issues {
		if is.Path != sk.File {
			continue
		}
		if f := availableSkillFixes(acts, is); len(f) > 0 && (fixes == nil || rank(is.Kind) < rank(best.Kind)) {
			best, fixes = is, f
		}
	}
	return best, fixes, fixes != nil
}

// skillNameList joins names, the first eight of them.
func skillNameList(names []string) string {
	if len(names) > 8 {
		return strings.Join(names[:8], ", ") + " …"
	}
	return strings.Join(names, ", ")
}

func skillByFile(sum skills.Summary, file string) (skills.Skill, bool) {
	for _, sk := range sum.Skills {
		if sk.File == file {
			return sk, true
		}
	}
	return skills.Skill{}, false
}

func skillFixAction(l l10n.Localizer, f skills.Fix) string {
	switch f {
	case skills.FixRemoveLink:
		return tx(l, xSkillsRemoveLink)
	case skills.FixLiftNested:
		return tx(l, xSkillsFixLift)
	case skills.FixDisableCopy:
		return tx(l, xSkillsFixDisable)
	case skills.FixTrashCopy:
		return tx(l, xSkillsFixTrash)
	default:
		return tx(l, xSkillsFixDesc)
	}
}

// skillFixMessage says what the fix changes.
func skillFixMessage(l l10n.Localizer, sum skills.Summary, is skills.Issue, sk skills.Skill, f skills.Fix) string {
	switch f {
	case skills.FixRemoveLink:
		return tx(l, xSkillsRemoveLinkMsg, skillPath(sum, is.Path))
	case skills.FixLiftNested:
		lifts := is.Lift.Lifts
		if len(lifts) == 1 {
			return tx(l, xSkillsFixLiftMsg, skillPath(sum, sk.Dir), skillPath(sum, lifts[0].Target))
		}
		names := make([]string, len(lifts))
		for i, lift := range lifts {
			names[i] = filepath.Base(lift.Target)
		}
		return tx(l, xSkillsFixLiftAllMsg, skillPath(sum, is.Lift.Wrapper), len(lifts), skillNameList(names))
	case skills.FixDisableCopy:
		return tx(l, xSkillsFixDisableMsg, skillPath(sum, is.Path), skillPath(sum, is.Detail))
	case skills.FixTrashCopy:
		return tx(l, xSkillsFixTrashMsg, skillPath(sum, sk.Removal.Path), skillPath(sum, is.Detail))
	default:
		return tx(l, xSkillsFixDescMsg, skillPath(sum, is.Path))
	}
}

// skillFixRisks are what the fix may cost, shown before it runs.
func skillFixRisks(l l10n.Localizer, sum skills.Summary, is skills.Issue, sk skills.Skill, f skills.Fix) []string {
	var risks []string
	shared := func() {
		if len(sk.SharedWith) > 0 {
			risks = append(risks, tx(l, xSkillsDeleteShared, skillProviders(l, sk.SharedWith)))
		}
	}
	switch f {
	case skills.FixLiftNested:
		lift := is.Lift
		wrapper := skillPath(sum, lift.Wrapper)
		switch {
		case lift.IsLink:
			risks = append(risks, tx(l, xSkillsRiskWrapperLink, wrapper))
		case lift.Extra > 0:
			risks = append(risks, tx(l, xSkillsRiskWrapper, wrapper, lift.Extra))
		default:
			risks = append(risks, tx(l, xSkillsRiskWrapperOnly, wrapper))
		}
		risks = append(risks, tx(l, xSkillsRiskReinstall), tx(l, xSkillsRiskRestart))
	case skills.FixDisableCopy:
		risks = append(risks, tx(l, xSkillsRiskConfigBackup), tx(l, xSkillsRiskRestart))
	case skills.FixTrashCopy:
		if sk.Removal.IsLink {
			risks = append(risks, tx(l, xSkillsDeleteLink))
		}
		if n := len(sk.Removal.Covers); n > 1 {
			risks = append(risks, tx(l, xSkillsDeleteCovers, skillNameList(sk.Removal.Covers)))
		}
		shared()
		risks = append(risks, tx(l, xSkillsDeleteRestart))
	case skills.FixAddDescription:
		risks = append(risks, tx(l, xSkillsRiskEditBackup))
		if sk.LinkPath != "" {
			risks = append(risks, tx(l, xSkillsRiskEditLink))
		}
		if len(sk.SharedWith) > 0 {
			risks = append(risks, tx(l, xSkillsRiskEditShared, skillProviders(l, sk.SharedWith)))
		}
		risks = append(risks, tx(l, xSkillsRiskEditUpdate), tx(l, xSkillsRiskRestart))
	case skills.FixRemoveLink:
		risks = append(risks, tx(l, xSkillsDeleteRestart))
	}
	return risks
}

func (u *settingsUI) skillFixRiskBox(c *ui.Context, l l10n.Localizer, risks []string) {
	t := c.Theme()
	ui.Column(c).Gap(6).Padding(10).Radius(8).Background(t.Warning.Alpha(0.10)).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, icWarnCircle).Size(13, 13).TextColor(t.Warning)
			ui.Text(c, tx(l, xSkillsFixRisks)).FontSize(12).FontWeight(600).TextColor(t.Warning)
		})
		for _, r := range risks {
			ui.Row(c).Gap(6).AlignItems(ui.Start).Children(func() {
				ui.Text(c, "•").FontSize(12).TextColor(t.TextMuted)
				ui.Text(c, r).FontSize(12).TextColor(t.Text).Grow(1).Basis(0)
			})
		}
	})
}

// runSkillFix carries the confirmed fix out in the background and rescans.
func (u *settingsUI) runSkillFix(acts SkillActions, sum skills.Summary, is skills.Issue, sk skills.Skill, f skills.Fix) {
	l := u.L()
	desc := strings.TrimSpace(u.skillsState().fixDesc)
	u.skillWork(acts, func() (string, error) {
		return applySkillFix(l, acts, sum, is, sk, f, desc)
	}, xSkillsFixFailed, nil)
}

// applySkillFix makes one fix and says what it did. It runs off the UI.
func applySkillFix(l l10n.Localizer, acts SkillActions, sum skills.Summary, is skills.Issue, sk skills.Skill, f skills.Fix, desc string) (string, error) {
	switch f {
	case skills.FixRemoveLink:
		system, err := acts.DeleteLink(sum, is.Path)
		return skillMovedText(l, system), err
	case skills.FixLiftNested:
		got, err := acts.LiftNested(sum, sk)
		text := tx(l, xSkillsFixedLiftAll, len(got.Paths))
		if len(got.Paths) == 1 {
			text = tx(l, xSkillsFixedLift, skillPath(sum, got.Paths[0]))
		}
		if got.WrapperInSystemTrash {
			text += " · " + tx(l, xSkillsMovedSystem)
		}
		return text, err
	case skills.FixDisableCopy:
		return tx(l, xSkillsFixedDisable), acts.SetEnabled(sk, false)
	case skills.FixTrashCopy:
		system, err := acts.Delete(sk, sum.Roots)
		return skillMovedText(l, system), err
	default:
		return tx(l, xSkillsFixedDesc), acts.AddDescription(sk, desc)
	}
}
