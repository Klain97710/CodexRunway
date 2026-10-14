package app

import (
	"slices"
	"strconv"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/store"
)

// ---- Display ----

func (u *settingsUI) displayPage(c *ui.Context) {
	u.appearanceGroup(c)
	u.privacyGroup(c)
	u.modulesGroup(c)
	u.notificationsGroup(c)
}

func (u *settingsUI) privacyGroup(c *ui.Context) {
	s, l := u.s, u.L()
	settingsGroup(c, icEye, TintTeal, tx(l, xPrivacyDisplaySection), "", nil, func(row rowFunc) {
		row(l.Text(l10n.KeyAccountPrivacyMode), tx(l, xPrivacyDisplayDescription), func() {
			if v, ok := toggle(c, "account-privacy", l.Text(l10n.KeyAccountPrivacyMode), s.Prefs().AccountPrivacyModeEnabled, s.ReadOnly()); ok {
				s.SetAccountPrivacyMode(v)
			}
		})
	})
}

func (u *settingsUI) appearanceGroup(c *ui.Context) {
	s, l := u.s, u.L()
	p := s.Prefs()
	ro := s.ReadOnly()
	sys := s.System()
	macNote := func(supported bool) string {
		if supported {
			return ""
		}
		return tx(l, xMacOnly)
	}
	settingsGroup(c, icPalette, TintBlue, l.Text(l10n.KeyDisplayAppearanceSection), "", nil, func(row rowFunc) {
		if sys.Features.Grok {
			row(tx(l, xProvider), tx(l, xProviderDescription), func() {
				providers := store.Providers()
				selected := max(0, slices.Index(providers, p.SelectedProvider))
				if ProviderTabs(c.Key("provider"), &selected, ro, l.Text(l10n.KeyProviderCodex), l.Text(l10n.KeyProviderGrok)) {
					s.SetSelectedProvider(providers[selected])
				}
			})
		}
		row(l.Text(l10n.KeyAppearance), "", func() {
			if v, ok := pills(c, "appearance", p.Appearance, store.Appearances(),
				func(v store.Appearance) string { return l.Text(appearanceKey(v)) }, ro); ok {
				s.SetAppearance(v)
			}
		})
		detail := l.Text(l10n.KeyMainPanelBackgroundDescription)
		if !sys.PanelBackground {
			detail = macNote(false)
		}
		row(l.Text(l10n.KeyMainPanelBackground), detail, func() {
			if v, ok := pills(c, "panel-bg", p.MainPanelBackgroundStyle, store.PanelBackgrounds(),
				func(v store.PanelBackground) string { return l.Text(panelBackgroundKey(v)) }, ro || !sys.PanelBackground); ok {
				s.SetPanelBackground(v)
			}
		})
		row(tx(l, xPanelHeight), tx(l, xPanelHeightHint), func() {
			h := p.MainPanelHeight
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, strconv.Itoa(int(h+0.5))).FontSize(13).FontFeatures("tnum").SingleLine()
				if ui.Stepper(c.Key("panel-height"), &h, store.MinMainPanelHeight, store.MaxMainPanelHeight, 20).
					Label(tx(l, xPanelHeight)).Disabled(ro).Changed() {
					s.SetMainPanelHeight(h)
				}
			})
		})
		u.statusBarRows(c, row, p, ro, sys.StatusBarStyles, macNote(sys.StatusBarStyles))
	})
}

// statusBarRows are the settings of the status item. Where the OS has no
// such status item they stay, disabled and explained, with their values.
func (u *settingsUI) statusBarRows(c *ui.Context, row rowFunc, p store.Preferences, ro, supported bool, note string) {
	s, l := u.s, u.L()
	off := ro || !supported
	row(l.Text(l10n.KeyStatusBarStyle), note, func() {
		if v, ok := choose(c, "sb-style", l.Text(l10n.KeyStatusBarStyle), p.StatusBarDisplayStyle, store.StatusBarStyles(),
			func(v store.StatusBarStyle) string { return l.Text(statusBarStyleKey(v)) }, off); ok {
			s.SetStatusBarStyle(v)
		}
	})
	if s.System().Features.Grok {
		row(l.Text(l10n.KeyStatusBarProviderScope), note, func() {
			if v, ok := choose(c, "sb-scope", l.Text(l10n.KeyStatusBarProviderScope), p.StatusBarProviderScope, store.StatusBarProviderScopes(),
				func(v store.StatusBarProviderScope) string { return l.Text(providerScopeKey(v)) }, off); ok {
				s.SetStatusBarProviderScope(v)
			}
		})
	}
	if p.StatusBarDisplayStyle == store.StatusBarStyleBattery {
		row(l.Text(l10n.KeyStatusBarBatteryScope), note, func() {
			if v, ok := choose(c, "sb-bscope", l.Text(l10n.KeyStatusBarBatteryScope), p.StatusBarBatteryScope, store.BatteryScopes(),
				func(v store.BatteryScope) string { return l.Text(batteryScopeKey(v)) }, off); ok {
				s.SetStatusBarBatteryScope(v)
			}
		})
		row(l.Text(l10n.KeyStatusBarBatteryDetailStyle), note, func() {
			if v, ok := choose(c, "sb-bdetail", l.Text(l10n.KeyStatusBarBatteryDetailStyle), p.StatusBarBatteryDetailStyle, store.BatteryDetails(),
				func(v store.BatteryDetail) string { return l.Text(batteryDetailKey(v)) }, off); ok {
				s.SetStatusBarBatteryDetail(v)
			}
		})
	}
}

// ---- Main panel modules ----

func (u *settingsUI) modulesGroup(c *ui.Context) {
	s, l := u.s, u.L()
	t := c.Theme()
	p := s.Prefs()
	ro := s.ReadOnly()
	atDefault := slices.Equal(p.MainPanelModuleOrder, store.Modules())
	Panel(c).Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Padding(0, 0, 8, 0).Children(func() {
			Tile(c, icLayers, TintPurple, 30, false)
			ui.Column(c).Grow(1).Basis(0).Gap(1).Children(func() {
				ui.Text(c, l.Text(l10n.KeyMainPanelModules)).FontSize(17).FontWeight(600).TextColor(TintPurple.Text(t))
				ui.Text(c, l.Text(l10n.KeyMainPanelModulesDescription)).FontSize(11).TextColor(t.TextMuted)
			})
			if IconButton(c, icUndo, l.Text(l10n.KeyRestoreDefaultOrder), ro || atDefault).Clicked() {
				s.ResetMainPanelModuleOrder()
			}
		})
		for i, m := range p.MainPanelModuleOrder {
			u.moduleCard(c, p, m, i, len(p.MainPanelModuleOrder))
		}
	})
}

func (u *settingsUI) moduleCard(c *ui.Context, p store.Preferences, m store.Module, index, count int) {
	s, l := u.s, u.L()
	t := c.Theme()
	ro := s.ReadOnly()
	info := moduleInfoOf(m)
	shown := ModuleShown(p, m)
	key := string(m)
	applies := l.Text(l10n.KeyModuleAppliesCodex)
	if info.both {
		applies = l.Text(l10n.KeyModuleAppliesBoth)
	}
	ui.Column(c).Children(func() {
		if index > 0 {
			ui.Box(c).Height(1).Background(pal(t).Line)
		}
		ui.Column(c).Gap(10).Padding(14, 0).Children(func() {
			ui.Row(c).Gap(10).AlignItems(ui.Start).Children(func() {
				ui.Row(c).Size(30, 26).Shrink(0).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
					ui.Icon(c, info.icon).Size(16, 16).TextColor(t.TextMuted)
				})
				ui.Column(c).Grow(1).Basis(0).Gap(3).Children(func() {
					col := t.Text
					if !shown {
						col = t.TextMuted
					}
					ui.Row(c).Height(26).AlignItems(ui.Center).Children(func() {
						ui.Text(c, l.Text(info.title)).FontSize(13).FontWeight(600).TextColor(col).Grow(1).Basis(0).SingleLine().Tooltip(l.Text(info.title))
					})
					ui.Text(c, l.Text(info.subtitle)).FontSize(11).TextColor(t.TextMuted)
					ui.Text(c, applies).FontSize(10).TextColor(t.TextMuted).SingleLine()
				})
				ui.Row(c).Width(110).Shrink(0).Gap(8).AlignItems(ui.Center).Justify(ui.End).Children(func() {
					if IconButton(c.Key("up-"+key), icChevUp, l.Text(l10n.KeyAccountsMoveUp), ro || index == 0).Clicked() {
						s.MoveMainPanelModule(m, -1)
					}
					if IconButton(c.Key("down-"+key), icChevDown, l.Text(l10n.KeyAccountsMoveDown), ro || index+1 >= count).Clicked() {
						s.MoveMainPanelModule(m, 1)
					}
					name := l.Text(l10n.KeyModuleHidden)
					if shown {
						name = l.Text(l10n.KeyModuleVisible)
					}
					if v, ok := toggle(c, "show-"+key, name, shown, ro); ok {
						s.SetModuleShown(m, v)
					}
				})
			})
			if info.config && (shown || info.configWhenHidden) {
				ui.Column(c).Padding(0, 0, 0, 40).Children(func() { u.moduleConfig(c, m, p) })
			}
		})
	})
}

// moduleConfig is the setting a module has of its own.
func (u *settingsUI) moduleConfig(c *ui.Context, m store.Module, p store.Preferences) {
	s, l := u.s, u.L()
	t := c.Theme()
	ro := s.ReadOnly()
	line := func(title, detail string, control func()) {
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).Basis(0).Gap(1).Children(func() {
				ui.Text(c, title).FontSize(12).FontWeight(500)
				if detail != "" {
					ui.Text(c, detail).FontSize(11).TextColor(t.TextMuted)
				}
			})
			control()
		})
	}
	switch m {
	case store.ModuleQuota:
		line(l.Text(l10n.KeyShowModelSpecificQuotaUsage), l.Text(l10n.KeyModelSpecificQuotaUsageDescription), func() {
			if v, ok := toggle(c, "model-quota", l.Text(l10n.KeyShowModelSpecificQuotaUsage), p.ShowsModelSpecificQuotaUsage, ro); ok {
				s.SetShowsModelSpecificQuotaUsage(v)
			}
		})
	case store.ModuleTokenUsage:
		line(l.Text(l10n.KeyTokenUsageChartStyle), "", func() {
			if v, ok := choose(c, "chart-style", l.Text(l10n.KeyTokenUsageChartStyle), p.TokenUsageChartStyle, store.ChartStyles(),
				func(v store.ChartStyle) string { return l.Text(chartStyleKey(v)) }, ro); ok {
				s.SetTokenUsageChartStyle(v)
			}
		})
	case store.ModuleRateLimitResetToday:
		line(l.Text(l10n.KeyRateLimitResetTodayRefreshInterval), "", func() {
			if v, ok := choose(c, "reset-today-interval", l.Text(l10n.KeyRateLimitResetTodayRefreshInterval),
				p.RateLimitResetTodayRefreshIntervalSeconds, store.RateLimitResetTodayRefreshIntervalOptions(), u.resetTodayInterval, ro); ok {
				s.SetRateLimitResetTodayRefreshInterval(v)
			}
		})
	case store.ModuleAPICost:
		line(l.Text(l10n.KeyApiCostSummaryRange), "", func() {
			if v, ok := choose(c, "cost-range", l.Text(l10n.KeyApiCostSummaryRange), p.APICostSummaryRange, store.CostRanges(),
				func(v store.CostRange) string { return l.Text(costRangeKey(v)) }, ro); ok {
				s.SetAPICostRange(v)
			}
		})
	case store.ModuleQuotaEstimate:
		line(l.Text(l10n.KeyQuotaEstimateWindowMode), "", func() {
			if v, ok := choose(c, "estimate-window", l.Text(l10n.KeyQuotaEstimateWindowMode), p.QuotaEstimateWindowMode, store.WindowModes(),
				func(v store.WindowMode) string { return l.Text(windowModeKey(v)) }, ro); ok {
				s.SetQuotaEstimateWindowMode(v)
			}
		})
	}
}

// ---- Notifications ----

func (u *settingsUI) notificationsGroup(c *ui.Context) {
	s, l := u.s, u.L()
	p := s.Prefs()
	ro := s.ReadOnly()
	settingsGroup(c, icBell, TintRed, l.Text(l10n.KeyNotificationSettings), "", nil, func(row rowFunc) {
		row(l.Text(l10n.KeyQuotaAlerts), l.Text(l10n.KeyQuotaAlertsDescription), func() {
			if v, ok := toggle(c, "alert-quota", l.Text(l10n.KeyQuotaAlerts), p.QuotaAlertsEnabled, ro); ok {
				s.SetQuotaAlerts(v)
			}
		})
		row(l.Text(l10n.KeyResetCreditAlerts), l.Text(l10n.KeyResetCreditAlertsDescription), func() {
			if v, ok := toggle(c, "alert-credits", l.Text(l10n.KeyResetCreditAlerts), p.ResetCreditAlertsEnabled, ro); ok {
				s.SetResetCreditAlerts(v)
			}
		})
		row(l.Text(l10n.KeyRateLimitResetTodayAlerts), l.Text(l10n.KeyRateLimitResetTodayAlertsDescription), func() {
			if v, ok := toggle(c, "alert-reset-today", l.Text(l10n.KeyRateLimitResetTodayAlerts), p.RateLimitResetTodayAlertsEnabled, ro); ok {
				s.SetRateLimitResetTodayAlerts(v)
			}
		})
		row(l.Text(l10n.KeyTestNotification), l.Text(l10n.KeyTestNotificationSubtitle), func() {
			test := u.opts.Actions.TestNotification
			if PillButton(c, icBell, l.Text(l10n.KeyTestNotification), test == nil).Clicked() {
				u.showNotice(l.Text(l10n.KeyTestNotification), test())
			}
		})
	})
}
