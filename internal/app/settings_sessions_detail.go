package app

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/sessions"
)

func (u *settingsUI) sessionDetailPage(c *ui.Context, actions SessionActions) {
	s, l, t := u.sessionsState(), u.L(), c.Theme()
	item, ok := s.findOpen()
	if !ok {
		s.open = ""
		return
	}
	u.loadSessionDetail(actions, item)
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		if PillButton(c, icChevLeft, tx(l, xSessionsBack), false).Clicked() {
			s.open = ""
			s.detail = sessionDetailState{generation: s.detail.generation + 1}
			return
		}
		ui.Text(c, item.Title).FontSize(15).FontWeight(650).Grow(1).Basis(0).SingleLine().Ellipsis("…")
		if actions.Reveal != nil && IconButton(c, icFolder, tx(l, xSessionsReveal), false).Clicked() {
			if err := actions.Reveal(item.Path); err != nil {
				s.notice, s.noticeErr = err.Error(), true
			}
		}
	})
	u.sessionMetadata(c, item)
	switch {
	case s.detail.task.Running:
		skillsLoading(c)
	case s.detail.task.Err != nil:
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, tx(l, xSessionsPreviewFailed, s.detail.task.Err.Error())).FontSize(12).TextColor(t.Danger).Grow(1).Basis(0)
			if PillButton(c, icRefresh, tx(l, xSessionsRetry), actions.Detail == nil).Clicked() {
				s.detail = sessionDetailState{generation: s.detail.generation + 1}
				u.loadSessionDetail(actions, item)
			}
		})
	case len(s.detail.detail.Messages) == 0:
		skillsEmptyBox(c, tx(l, xSessionsNoPreview))
	default:
		u.sessionMessages(c, s.detail.detail)
	}
}

func (s *settingsSessions) findOpen() (sessions.ManagedSession, bool) {
	for _, item := range s.items {
		if sessionKey(item) == s.open {
			return item, true
		}
	}
	return sessions.ManagedSession{}, false
}

func (u *settingsUI) sessionMetadata(c *ui.Context, item sessions.ManagedSession) {
	l, t := u.L(), c.Theme()
	model := l.Text(l10n.KeyUnknown)
	if item.Model != nil && *item.Model != "" {
		model = *item.Model
	}
	cost := recentCost(item.Item)
	fields := []struct{ label, value string }{
		{tx(l, xSessionsID), item.ID},
		{tx(l, xSessionsProject), item.ProjectName},
		{tx(l, xSessionsModel), model},
		{tx(l, xSessionsUpdated), item.UpdatedAt.Local().Format("2006-01-02 15:04:05")},
		{tx(l, xSessionsTokenBreakdown), tx(l, xSessionsTokenValues, tokenCompact(item.Totals.TotalTokens), tokenCompact(item.Totals.UncachedInputTokens), tokenCompact(item.Totals.CachedInputTokens), tokenCompact(item.Totals.OutputTokens), item.Totals.Turns)},
		{tx(l, xSessionsEstimatedCost), cost},
		{tx(l, xSessionsFiles), fmt.Sprint(len(item.Files))},
		{tx(l, xSessionsPath), item.Path},
	}
	if item.Problem != "" {
		fields = append(fields, struct{ label, value string }{tx(l, xSessionsProblem), item.Problem})
	}
	Panel(c).Children(func() {
		for i, field := range fields {
			settingsRule(c, i)
			ui.Row(c).Gap(12).Padding(7, 0).Children(func() {
				ui.Text(c, field.label).Width(120).FontSize(11).TextColor(t.TextMuted)
				ui.Text(c, field.value).FontSize(11).Grow(1).Basis(0).Selectable()
			})
		}
	})
}

func (u *settingsUI) sessionMessages(c *ui.Context, detail sessions.ManagedDetail) {
	l, t := u.L(), c.Theme()
	ui.Column(c).Gap(8).Children(func() {
		ui.Text(c, tx(l, xSessionsPreview)).FontSize(13).FontWeight(600)
		Panel(c).Children(func() {
			for i, message := range detail.Messages {
				settingsRule(c, i)
				ui.Column(c).Gap(3).Padding(8, 0).Children(func() {
					header := sessionRole(l, message.Role)
					if !message.At.IsZero() {
						header += " · " + message.At.Local().Format("2006-01-02 15:04")
					}
					ui.Text(c, header).FontSize(10).FontWeight(600).TextColor(t.TextMuted)
					ui.Text(c, message.Text).FontSize(12).Selectable()
				})
			}
		})
		if detail.Truncated {
			ui.Text(c, tx(l, xSessionsPreviewTruncated)).FontSize(11).TextColor(t.Warning)
		}
	})
}

func sessionRole(l l10n.Localizer, role string) string {
	switch strings.ToLower(role) {
	case "user", "human":
		return tx(l, xSessionsRoleUser)
	case "assistant":
		return tx(l, xSessionsRoleAssistant)
	case "system":
		return tx(l, xSessionsRoleSystem)
	case "tool":
		return tx(l, xSessionsRoleTool)
	default:
		return role
	}
}
