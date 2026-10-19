package app

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/sessions"
)

type sessionProject struct {
	key, name, path, label string
}

func sessionProjectKey(item sessions.ManagedSession) string {
	if item.Cwd != nil && strings.TrimSpace(*item.Cwd) != "" {
		return "cwd:" + *item.Cwd
	}
	return "name:" + item.ProjectName
}

func sessionProjects(items []sessions.ManagedSession) []sessionProject {
	byKey, nameCount := map[string]sessionProject{}, map[string]int{}
	for _, item := range items {
		key := sessionProjectKey(item)
		if _, ok := byKey[key]; ok {
			continue
		}
		path := ""
		if item.Cwd != nil {
			path = strings.TrimSpace(*item.Cwd)
		}
		name := strings.TrimSpace(item.ProjectName)
		byKey[key] = sessionProject{key: key, name: name, path: path, label: name}
		nameCount[name]++
	}
	projects := make([]sessionProject, 0, len(byKey))
	for _, project := range byKey {
		if project.label == "" {
			project.label = project.path
		}
		if nameCount[project.name] > 1 && project.path != "" {
			project.label = project.name + " · " + project.path
		}
		projects = append(projects, project)
	}
	slices.SortFunc(projects, func(a, b sessionProject) int {
		if n := strings.Compare(strings.ToLower(a.label), strings.ToLower(b.label)); n != 0 {
			return n
		}
		return strings.Compare(a.key, b.key)
	})
	return projects
}

// sessionFilterBar lays the filters out in one row, wrapping in a narrow
// window: search, project, date range, quick ranges and Apply.
func (u *settingsUI) sessionFilterBar(c *ui.Context) {
	s, l, t := u.sessionsState(), u.L(), c.Theme()
	Panel(c).Padding(8, 10).Gap(6).Children(func() {
		ui.Row(c).Gap(6).Wrap().AlignItems(ui.Center).Children(func() {
			ui.TextInput(c.Key("session-search"), &s.draft.search).Placeholder(tx(l, xSessionsSearchHint)).Label(tx(l, xSessionsSearch)).
				FontSize(12).Height(listControlHeight).Grow(1).Basis(0).MinWidth(140)
			u.sessionProjectPicker(c, sessionProjects(s.items))
			ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
				u.sessionDateField(c, 0, tx(l, xSessionsStartDate), tx(l, xSessionsChooseStart), &s.draft.from)
				ui.Text(c, "–").FontSize(11).TextColor(t.TextMuted)
				u.sessionDateField(c, 1, tx(l, xSessionsEndDate), tx(l, xSessionsChooseEnd), &s.draft.to)
			})
			u.sessionQuickRanges(c)
			// Apply turns the warning color while the draft waits for it.
			var apply ui.Element
			if !s.work.Running && s.filterPending() {
				apply = SmallWarningPillButton(c, tx(l, xSessionsApply))
			} else {
				apply = SmallPillButton(c, nil, tx(l, xSessionsApply), s.work.Running)
			}
			if apply.Height(listControlHeight).Cursor(ui.CursorPointer).Tooltip(tx(l, xSessionsFilterSubmitHint)).Clicked() {
				s.applyFilter(time.Local)
			}
		})
		if s.filterErr != nil {
			key := xSessionsInvalidDate
			if s.filterErr.Error() == "date range is reversed" {
				key = xSessionsReversedDate
			}
			ui.Text(c, tx(l, key)).FontSize(11).TextColor(t.Danger)
		}
	})
}

// sessionQuickRanges is a small calendar menu button that applies a date
// range at once, keeping the draft search and project, and checks the range
// applied.
func (u *settingsUI) sessionQuickRanges(c *ui.Context) {
	s, l := u.sessionsState(), u.L()
	now := time.Now()
	applied, matched := s.applied.quickRange(now)
	listMenuButton(c, "session-quick-range", icCalendarAlert, "", tx(l, xSessionsQuickRange), s.work.Running, func(m *ui.Menu) {
		for _, option := range []struct {
			label string
			days  int
		}{{l.Text(l10n.KeyToday), 1}, {tx(l, xSessionsLast7Days), 7}, {tx(l, xSessionsLast30Days), 30}, {tx(l, xSessionsAllDates), 0}} {
			if m.Item(option.label).Checked(matched && applied == option.days).Chosen() {
				s.setQuickRange(now, option.days)
			}
		}
	})
}

func (u *settingsUI) sessionDateField(c *ui.Context, index int, label, emptyLabel string, value *string) {
	s, l := u.sessionsState(), u.L()
	display := *value
	if display == "" {
		display = emptyLabel
	}
	anchor := SmallPillButton(c, nil, display, s.work.Running).Height(listControlHeight).Cursor(ui.CursorPointer).Label(label).Tooltip(label)
	if anchor.Clicked() {
		s.dateOpen[index] = !s.dateOpen[index]
		if date, err := time.ParseInLocation(dateLayout, *value, time.Local); err == nil {
			s.dateCursor[index] = date
		} else {
			now := time.Now().In(time.Local)
			s.dateCursor[index] = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
		}
	}
	ui.Popover(c, anchor, &s.dateOpen[index], func() {
		ui.Column(c).Padding(10).Gap(8).Children(func() {
			ui.Calendar(c.Key("session-calendar-"+fmt.Sprint(index)), &s.dateCursor[index]).Label(label).AutoFocus()
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if PillButton(c, icClose, tx(l, xSessionsClearDate), false).Clicked() {
					*value, s.dateOpen[index] = "", false
				}
				if PillButton(c, icCheckCircle, tx(l, xSessionsUseDate), false).Clicked() {
					*value, s.dateOpen[index] = s.dateCursor[index].Format(dateLayout), false
				}
			})
		})
	})
}

// sessionProjectPicker chooses the project as the font picker does: All
// projects first, then the scanned ones, which typing filters. A chosen
// project no longer scanned stays listed under its saved label.
func (u *settingsUI) sessionProjectPicker(c *ui.Context, projects []sessionProject) {
	s, l := u.sessionsState(), u.L()
	all := tx(l, xSessionsAllProjects)
	options, keys := []string{all}, map[string]string{all: ""}
	add := func(label, key string) {
		if _, ok := keys[label]; !ok {
			options, keys[label] = append(options, label), key
		}
	}
	selected := all
	for _, project := range projects {
		add(project.label, project.key)
		if project.key == s.draft.project {
			selected = project.label
		}
	}
	if s.draft.project != "" && selected == all {
		label := s.draft.projectLabel
		if label == "" {
			label = strings.TrimPrefix(strings.TrimPrefix(s.draft.project, "cwd:"), "name:")
		}
		add(label, s.draft.project)
		selected = label
	}
	if ui.Combobox(c.Key("session-project"), &selected, options).Label(tx(l, xSessionsProjectFilter)).
		FontSize(12).Width(170).Height(listControlHeight).Disabled(s.work.Running).Changed() {
		s.draft.project, s.draft.projectLabel = keys[selected], ""
		if s.draft.project != "" {
			s.draft.projectLabel = selected
		}
	}
}
