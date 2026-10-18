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

func (u *settingsUI) sessionFilterBar(c *ui.Context) {
	s, l, t := u.sessionsState(), u.L(), c.Theme()
	Panel(c).Padding(10, 12).Gap(8).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.TextInput(c.Key("session-search"), &s.draft.search).Placeholder(tx(l, xSessionsSearchHint)).Label(tx(l, xSessionsSearch)).Grow(1).Basis(0)
			u.sessionProjectPicker(c, sessionProjects(s.items))
			if PillButton(c, nil, tx(l, xSessionsApply), s.work.Running).Clicked() {
				s.applyFilter(time.Local)
			}
		})
		ui.Row(c).Gap(6).Wrap().AlignItems(ui.Center).Children(func() {
			u.sessionDateField(c, 0, tx(l, xSessionsStartDate), tx(l, xSessionsChooseStart), &s.draft.from)
			ui.Text(c, "–").FontSize(12).TextColor(t.TextMuted)
			u.sessionDateField(c, 1, tx(l, xSessionsEndDate), tx(l, xSessionsChooseEnd), &s.draft.to)
			ui.Text(c, tx(l, xSessionsQuickRange)).FontSize(11).TextColor(t.TextMuted)
			for _, option := range []struct {
				label string
				days  int
			}{{l.Text(l10n.KeyToday), 1}, {tx(l, xSessionsLast7Days), 7}, {tx(l, xSessionsLast30Days), 30}, {tx(l, xSessionsAllDates), 0}} {
				if PillButton(c, nil, option.label, s.work.Running).Clicked() {
					s.setQuickRange(time.Now(), option.days)
				}
			}
		})
		ui.Text(c, tx(l, xSessionsFilterSubmitHint)).FontSize(11).TextColor(t.TextMuted)
		if s.filterErr != nil {
			key := xSessionsInvalidDate
			if s.filterErr.Error() == "date range is reversed" {
				key = xSessionsReversedDate
			}
			ui.Text(c, tx(l, key)).FontSize(11).TextColor(t.Danger)
		}
	})
}

func (u *settingsUI) sessionDateField(c *ui.Context, index int, label, emptyLabel string, value *string) {
	s, l := u.sessionsState(), u.L()
	display := *value
	if display == "" {
		display = emptyLabel
	}
	anchor := PillButton(c, icCalendarAlert, display, s.work.Running).Label(label).Tooltip(label)
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

func (u *settingsUI) sessionProjectPicker(c *ui.Context, projects []sessionProject) {
	s, l, t := u.sessionsState(), u.L(), c.Theme()
	value, label := s.draft.project, tx(l, xSessionsAllProjects)
	if value != "" {
		label = s.draft.projectLabel
		if label == "" {
			label = strings.TrimPrefix(strings.TrimPrefix(value, "cwd:"), "name:")
		}
	}
	for _, project := range projects {
		if project.key == value {
			label = project.label
			break
		}
	}
	sel := ui.SelectBase(c.Key("session-project"), &value)
	trigger := sel.Trigger.Label(tx(l, xSessionsProjectFilter)).Disabled(s.work.Running).Width(190).Gap(5).Padding(5, 7).Radius(6).AlignItems(ui.Center)
	if !s.work.Running && (trigger.Hovered() || sel.Open()) {
		trigger.Background(pal(t).Hover)
	}
	trigger.Children(func() {
		ui.Text(c, label).FontSize(12).SingleLine().Ellipsis("…").Tooltip(label).Grow(1).Basis(0)
		ui.Icon(c, icUpDown).Size(13, 13).TextColor(t.TextMuted)
	})
	sel.Popup(func(panel ui.Element) {
		popupMenu(c, panel.MaxWidth(420))
		ui.Scroll(c.Key("session-project-options")).MaxHeight(260).Children(func() {
			u.sessionProjectOption(c, sel, "", tx(l, xSessionsAllProjects), "", value)
			for _, project := range projects {
				u.sessionProjectOption(c, sel, project.key, project.label, project.path, value)
			}
		})
	})
	if trigger.Changed() {
		s.draft.project, s.draft.projectLabel = value, ""
		for _, project := range projects {
			if project.key == value {
				s.draft.projectLabel = project.label
				break
			}
		}
	}
}

func (u *settingsUI) sessionProjectOption(c *ui.Context, sel ui.SelectParts[string], key, label, path, current string) {
	t := c.Theme()
	item := sel.Item(key).Padding(5, 10, 5, 6).Gap(5).Radius(5).AlignItems(ui.Center)
	color := t.Text
	if item.Highlighted() {
		item.Background(t.Accent)
		color = t.AccentText
	}
	item.Children(func() {
		checkmark(c, key == current, color)
		ui.Column(c).Gap(1).MinWidth(0).MaxWidth(370).Children(func() {
			ui.Text(c, label).MaxWidth(370).FontSize(12).TextColor(color).SingleLine().Ellipsis("…").Tooltip(label)
			if path != "" && !strings.Contains(label, path) {
				ui.Text(c, path).MaxWidth(370).FontSize(10).TextColor(color.Alpha(0.72)).SingleLine().Ellipsis("…").Tooltip(path)
			}
		})
	})
}
