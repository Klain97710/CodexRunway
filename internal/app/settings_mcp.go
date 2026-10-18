package app

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/mcp"
	"github.com/licoy/quosparr/internal/store"
)

func (u *settingsUI) mcpPage(c *ui.Context) {
	s, actions := u.mcpState(), u.mcpActions()
	if !s.loaded && !s.task.Running && s.task.Err == nil {
		u.startMCPScan(actions)
	}
	ui.Column(c).Gap(14).Children(func() {
		u.mcpProviderTabs(c, actions)
		if s.open != "" {
			u.mcpDetail(c, actions)
			return
		}
		u.mcpList(c, actions)
	})
}

func (u *settingsUI) mcpProviderTabs(c *ui.Context, actions MCPActions) {
	s, l, t := u.mcpState(), u.L(), c.Theme()
	providers := store.Providers()
	selected := slicesIndexProvider(providers, s.provider)
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		seg := ui.SegmentedBase(c.Key("mcp-providers"), &selected, len(providers))
		seg.Track.Padding(3).Gap(2).Radius(7).Background(t.Text.Alpha(0.06)).Children(func() {
			for i, provider := range providers {
				part := seg.Segment(i).Padding(5, 10).Gap(6).Radius(5).Label(l.Text(providerKey(provider))).Disabled(s.work.Running)
				color := t.TextMuted
				if i == selected {
					color = t.Text
					part.Background(pal(t).Inset).Shadow(0, 1, 3, 0, ui.RGBA(0, 0, 0, 0.12))
				}
				part.Children(func() {
					ProviderIcon(c, provider, 15)
					ui.Text(c, l.Text(providerKey(provider))).FontSize(13).FontWeight(500).TextColor(color).SingleLine()
				})
			}
		})
		if seg.Track.Changed() {
			s.switchProvider(providers[selected])
			u.startMCPScan(actions)
		}
		ui.Spacer(c).Grow(1)
		if s.task.Running || s.work.Running {
			ui.Spinner(c).Size(14, 14)
		}
		if IconButton(c, icRefresh, l.Text(l10n.KeyRefresh), s.task.Running || s.work.Running || actions.Query == nil).Clicked() {
			u.startMCPScan(actions)
		}
	})
}

func slicesIndexProvider(providers []store.Provider, current store.Provider) int {
	for i, provider := range providers {
		if provider == current {
			return i
		}
	}
	return 0
}

func (u *settingsUI) mcpList(c *ui.Context, actions MCPActions) {
	s, l, t := u.mcpState(), u.L(), c.Theme()
	if !s.loaded {
		if s.task.Err != nil {
			ui.Text(c, tx(l, xMCPLoadFailed, s.task.Err.Error())).FontSize(12).TextColor(t.Danger)
		} else {
			skillsLoading(c)
		}
		return
	}
	if s.task.Err != nil {
		ui.Text(c, tx(l, xMCPRefreshFailed, s.task.Err.Error())).FontSize(12).TextColor(t.Danger)
	}
	ui.Text(c, tx(l, xMCPScopeInfo)).FontSize(11).TextColor(t.TextMuted)
	u.mcpToolbar(c, actions)
	u.mcpNotice(c)
	list := s.filtered(l)
	if len(list) == 0 {
		message := tx(l, xMCPEmpty)
		if len(s.snapshot.Servers) > 0 {
			message = tx(l, xMCPNoMatch)
		}
		skillsEmptyBox(c, message)
		return
	}
	Panel(c).Children(func() {
		for i, server := range list {
			settingsRule(c, i)
			u.mcpRow(c, server, actions)
		}
	})
}

func (u *settingsUI) mcpToolbar(c *ui.Context, actions MCPActions) {
	s, l, t := u.mcpState(), u.L(), c.Theme()
	list := s.filtered(l)
	selected := len(s.selectedServers())
	ui.Column(c).Gap(8).Children(func() {
		ui.TextInput(c.Key("mcp-search"), &s.search).Placeholder(tx(l, xMCPSearchHint)).Label(tx(l, xMCPSearch)).Grow(1)
		ui.Row(c).Gap(8).Wrap().AlignItems(ui.Center).Children(func() {
			ui.Text(c, tx(l, xMCPSelectionCounts, len(list), selected)).FontSize(12).FontFeatures("tnum").TextColor(t.TextMuted)
			ui.Spacer(c).Grow(1)
			if PillButton(c, icCheckCircle, tx(l, xMCPSelectFiltered, len(list)), s.task.Running || s.work.Running || s.stale || !hasDeletableMCP(list)).Clicked() {
				s.selected = map[string]bool{}
				for _, server := range list {
					if server.Deletable {
						s.selected[server.ID] = true
					}
				}
			}
			if PillButton(c, icClose, tx(l, xMCPClearSelection), s.work.Running || selected == 0).Clicked() {
				s.selected, s.confirm = map[string]bool{}, false
			}
			if PillButton(c, icTrash, tx(l, xMCPDeleteSelected), s.task.Running || s.work.Running || s.stale || selected == 0 || actions.Delete == nil).Clicked() {
				s.confirm = true
			}
		})
	})
}

func hasDeletableMCP(servers []mcp.Server) bool {
	for _, server := range servers {
		if server.Deletable {
			return true
		}
	}
	return false
}

func (u *settingsUI) mcpRow(c *ui.Context, server mcp.Server, actions MCPActions) {
	s, l, t := u.mcpState(), u.L(), c.Theme()
	checked := s.selected[server.ID]
	ui.Row(c).Gap(10).Padding(9, 0).AlignItems(ui.Center).Children(func() {
		if ui.Checkbox(c.Key("mcp-check-"+server.ID), &checked, "").Label(tx(l, xMCPSelectOne, server.Name)).Size(18, 18).Disabled(s.work.Running || !server.Deletable).Changed() {
			if checked {
				s.selected[server.ID] = true
			} else {
				delete(s.selected, server.ID)
			}
		}
		button := ui.ButtonBase(c.Key("mcp-open-" + server.ID)).Label(tx(l, xMCPOpenDetails, server.Name)).Grow(1).Basis(0).Padding(2).Radius(6)
		if button.Hovered() {
			button.Background(pal(t).Hover)
		}
		button.Children(func() {
			ui.Column(c).Gap(3).Grow(1).Basis(0).Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Text(c, server.Name).FontSize(13).FontWeight(550).SingleLine().Ellipsis("…").Grow(1).Basis(0)
					if !server.Enabled {
						ui.Text(c, tx(l, xMCPDisabled)).FontSize(10).TextColor(t.TextMuted)
					}
					if server.Problem != "" {
						ui.Text(c, tx(l, xMCPProblem)).FontSize(10).TextColor(t.Warning)
					}
				})
				ui.Text(c, mcpSummary(server)).FontSize(11).TextColor(t.TextMuted).SingleLine().Ellipsis("…")
			})
		})
		if button.Clicked() {
			s.open = server.ID
		}
		if actions.Reveal != nil && IconButton(c, icFolder, tx(l, xMCPReveal), false).Clicked() {
			if err := actions.Reveal(server.Path); err != nil {
				s.notice, s.noticeErr = err.Error(), true
			}
		}
	})
}

func mcpSummary(server mcp.Server) string {
	return strings.Join(nonEmpty([]string{server.Transport, server.Command, server.URL}), " · ")
}

func (u *settingsUI) mcpNotice(c *ui.Context) {
	s := u.mcpState()
	if s.notice == "" {
		return
	}
	color, icon := c.Theme().Success, icCheckCircle
	if s.noticeErr {
		color, icon = c.Theme().Danger, icWarnCircle
	}
	ui.Row(c).Gap(8).Padding(8, 10).Radius(8).Background(color.Alpha(0.10)).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, icon).Size(13, 13).TextColor(color)
		ui.Text(c, s.notice).FontSize(12).TextColor(color).Grow(1).Basis(0)
	})
}

func (u *settingsUI) mcpDetail(c *ui.Context, actions MCPActions) {
	s, l, t := u.mcpState(), u.L(), c.Theme()
	server, ok := s.find(s.open)
	if !ok {
		s.open = ""
		return
	}
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		if PillButton(c, icChevLeft, tx(l, xMCPBack), false).Clicked() {
			s.open = ""
		}
		ui.Text(c, server.Name).FontSize(16).FontWeight(650).Grow(1).Basis(0)
		if actions.Reveal != nil && PillButton(c, icFolder, tx(l, xMCPReveal), false).Clicked() {
			if err := actions.Reveal(server.Path); err != nil {
				s.notice, s.noticeErr = err.Error(), true
			}
		}
	})
	u.mcpNotice(c)
	Panel(c).Padding(14).Gap(10).Children(func() {
		mcpDetailRow(c, tx(l, xMCPPath), server.Path)
		mcpDetailRow(c, tx(l, xMCPTransport), server.Transport)
		mcpDetailRow(c, tx(l, xMCPCommand), server.Command)
		mcpDetailRow(c, tx(l, xMCPURL), server.URL)
		mcpDetailRow(c, tx(l, xMCPStatus), mcpStatusText(l, server))
		if server.Problem != "" {
			mcpDetailRow(c, tx(l, xMCPProblem), mcpProblemText(l, server.Problem))
		}
	})
	if server.Details != "" {
		ui.Text(c, tx(l, xMCPRedactedConfig)).FontSize(12).FontWeight(600)
		ui.Column(c).Padding(12).Radius(9).Background(pal(t).Inset).Border(1, pal(t).Line).Children(func() {
			ui.Text(c, server.Details).FontSize(11).TextColor(t.TextMuted)
		})
	}
}

func (s *settingsMCP) find(id string) (mcp.Server, bool) {
	for _, server := range s.snapshot.Servers {
		if server.ID == id {
			return server, true
		}
	}
	return mcp.Server{}, false
}

func mcpDetailRow(c *ui.Context, label, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	ui.Row(c).Gap(12).AlignItems(ui.Start).Children(func() {
		ui.Text(c, label).Width(110).FontSize(11).TextColor(c.Theme().TextMuted)
		ui.Text(c, value).Grow(1).Basis(0).FontSize(12)
	})
}

func mcpStatusText(l l10n.Localizer, server mcp.Server) string {
	status := tx(l, xMCPEnabled)
	if !server.Enabled {
		status = tx(l, xMCPDisabled)
	}
	if !server.Deletable {
		status += " · " + tx(l, xMCPReadOnly)
	}
	return status
}

func mcpProblemText(l l10n.Localizer, problem string) string {
	switch problem {
	case "missingTransport":
		return tx(l, xMCPMissingTransport)
	case "multipleTransports":
		return tx(l, xMCPMultipleTransports)
	case "invalidTransport":
		return tx(l, xMCPInvalidTransport)
	case "missingType":
		return tx(l, xMCPMissingType)
	case "transportMismatch":
		return tx(l, xMCPTransportMismatch)
	default:
		return tx(l, xMCPUnknownProblem)
	}
}

func (u *settingsUI) mcpPageKey() string {
	if u.tab != tabMCP || u.mcp == nil {
		return ""
	}
	return fmt.Sprintf("-%s-%s", u.mcp.provider, u.mcp.open)
}
