package app

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/sessions"
	"github.com/licoy/quosparr/internal/store"
)

func (u *settingsUI) sessionsPage(c *ui.Context) {
	s, actions := u.sessionsState(), u.sessionActions()
	if !s.loaded && !s.task.Running && s.task.Err == nil {
		u.startSessionsScan(actions)
	}
	ui.Column(c).Gap(14).Children(func() {
		u.sessionProviderTabs(c, actions)
		if s.open != "" {
			u.sessionDetailPage(c, actions)
			return
		}
		u.sessionFilterBar(c)
		u.sessionNotice(c)
		u.sessionList(c, actions)
	})
}

func (u *settingsUI) sessionProviderTabs(c *ui.Context, actions SessionActions) {
	s, l, t := u.sessionsState(), u.L(), c.Theme()
	providers := store.Providers()
	selected := 0
	for i, provider := range providers {
		if provider == s.provider {
			selected = i
		}
	}
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		seg := ui.SegmentedBase(c.Key("session-providers"), &selected, len(providers))
		seg.Track.Padding(3).Gap(2).Radius(7).Background(t.Text.Alpha(0.06)).Children(func() {
			for i, provider := range providers {
				segment := seg.Segment(i).Padding(5, 10).Gap(6).Radius(5).Label(l.Text(providerKey(provider))).Disabled(s.work.Running)
				color := t.TextMuted
				if i == selected {
					color = t.Text
					segment.Background(pal(t).Inset).Shadow(0, 1, 3, 0, ui.RGBA(0, 0, 0, 0.12))
				}
				segment.Children(func() {
					ProviderIcon(c, provider, 15)
					ui.Text(c, l.Text(providerKey(provider))).FontSize(13).FontWeight(500).TextColor(color).SingleLine()
				})
			}
		})
		if seg.Track.Changed() {
			s.switchProvider(providers[selected])
			u.startSessionsScan(actions)
		}
		ui.Spacer(c).Grow(1)
		if s.task.Running || s.work.Running {
			ui.Spinner(c).Size(14, 14)
		}
		if IconButton(c, icRefresh, l.Text(l10n.KeyRefresh), s.task.Running || s.work.Running || actions.Query == nil).Clicked() {
			u.startSessionsScan(actions)
		}
	})
}

func (u *settingsUI) sessionList(c *ui.Context, actions SessionActions) {
	s, l, t := u.sessionsState(), u.L(), c.Theme()
	if !s.loaded {
		if s.task.Err != nil {
			ui.Text(c, tx(l, xSessionsLoadFailed, s.task.Err.Error())).FontSize(12).TextColor(t.Danger)
		} else {
			skillsLoading(c)
		}
		return
	}
	if s.task.Err != nil {
		ui.Text(c, tx(l, xSessionsRefreshFailed, s.task.Err.Error())).FontSize(12).TextColor(t.Danger)
	}
	list := s.filtered()
	u.sessionSelectionBar(c, list, actions)
	if len(list) == 0 {
		message := tx(l, xSessionsEmpty)
		if len(s.items) > 0 {
			message = tx(l, xSessionsNoMatch)
		}
		skillsEmptyBox(c, message)
		return
	}
	shown := min(s.shown, len(list))
	ui.Text(c, tx(l, xSessionsShowing, shown, len(list))).FontSize(11).TextColor(t.TextMuted).Padding(0, 4)
	Panel(c).Children(func() {
		for i, item := range list[:shown] {
			settingsRule(c, i)
			u.sessionRow(c, item, actions)
		}
	})
	if shown < len(list) {
		ui.Row(c).Justify(ui.Center).Children(func() {
			if PillButton(c, nil, tx(l, xSessionsShowMore, min(sessionPageSize, len(list)-shown)), false).Clicked() {
				s.shown += sessionPageSize
			}
		})
	}
}

func (u *settingsUI) sessionSelectionBar(c *ui.Context, list []sessions.ManagedSession, actions SessionActions) {
	s, l, t := u.sessionsState(), u.L(), c.Theme()
	selected := len(s.selectedItems())
	ui.Row(c).Gap(8).Wrap().AlignItems(ui.Center).Children(func() {
		ui.Text(c, tx(l, xSessionsSelectionCounts, len(list), selected)).FontSize(12).FontFeatures("tnum").TextColor(t.TextMuted)
		ui.Spacer(c).Grow(1)
		if PillButton(c, icCheckCircle, tx(l, xSessionsSelectAll), s.work.Running || len(list) == 0).
			Label(tx(l, xSessionsSelectFiltered, len(list))).Tooltip(tx(l, xSessionsSelectFiltered, len(list))).Clicked() {
			s.selected = make(map[string]bool, len(list))
			for _, item := range list {
				s.selected[sessionKey(item)] = true
			}
		}
		if PillButton(c, icClose, tx(l, xSessionsDeselectAll), s.work.Running || selected == 0).
			Label(tx(l, xSessionsClearSelection)).Tooltip(tx(l, xSessionsClearSelection)).Clicked() {
			s.clearSelection()
		}
		if PillButton(c, icTrash, tx(l, xSessionsDeleteShort), s.work.Running || selected == 0 || actions.Delete == nil).
			Label(tx(l, xSessionsDeleteSelected)).Tooltip(tx(l, xSessionsDeleteSelected)).Clicked() {
			s.confirm = true
		}
	})
}

func (u *settingsUI) sessionRow(c *ui.Context, item sessions.ManagedSession, actions SessionActions) {
	s, l, t := u.sessionsState(), u.L(), c.Theme()
	key := sessionKey(item)
	checked := s.selected[key]
	title := item.Title
	if strings.TrimSpace(title) == "" {
		title = l.Text(l10n.KeyUnknown)
	}
	ui.Row(c).Gap(10).Padding(8, 0).AlignItems(ui.Center).Children(func() {
		if ui.Checkbox(c.Key("session-check-"+key), &checked, "").Label(tx(l, xSessionsSelectOne, title)).Size(18, 18).Disabled(s.work.Running).Changed() {
			if checked {
				s.selected[key] = true
			} else {
				delete(s.selected, key)
			}
		}
		button := ui.ButtonBase(c.Key("session-open-" + key)).Label(tx(l, xSessionsOpenDetails, title)).Grow(1).Basis(0).Padding(2).Radius(6)
		if button.Hovered() {
			button.Background(pal(t).Hover)
		}
		button.Children(func() {
			ui.Column(c).Gap(2).Grow(1).Basis(0).Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Text(c, title).FontSize(13).FontWeight(550).SingleLine().Ellipsis("…").Grow(1).Basis(0)
					if item.Archived {
						ui.Text(c, tx(l, xSessionsArchived)).FontSize(10).TextColor(t.TextMuted)
					}
					if item.Problem != "" || item.State == sessions.StateNeedsAttention {
						ui.Text(c, tx(l, xSessionsNeedsAttention)).FontSize(10).TextColor(t.Warning)
					}
				})
				meta := []string{item.ProjectName, item.UpdatedAt.Local().Format("2006-01-02 15:04"), tokenCompact(item.Totals.TotalTokens) + " " + l.Text(l10n.KeyTokens)}
				ui.Text(c, strings.Join(nonEmpty(meta), " · ")).FontSize(11).TextColor(t.TextMuted).SingleLine().Ellipsis("…")
			})
		})
		if button.Clicked() {
			s.open = key
			u.loadSessionDetail(actions, item)
		}
		if actions.Reveal != nil && IconButton(c, icFolder, tx(l, xSessionsReveal), false).Clicked() {
			if err := actions.Reveal(item.Path); err != nil {
				s.notice, s.noticeErr = err.Error(), true
			}
		}
	})
}

func nonEmpty(values []string) []string {
	result := values[:0]
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			result = append(result, value)
		}
	}
	return result
}
