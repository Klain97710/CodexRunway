package app

import (
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/codex"
	"github.com/licoy/quosparr/internal/grok"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/runway"
	"github.com/licoy/quosparr/internal/store"
)

// panelHeader is the fixed header of the main page: title and provider menu,
// the four header buttons, the identity row and the subscription chip.
func panelHeader(c *ui.Context, a *App, l l10n.Localizer, prefs store.Preferences, snap runway.Snapshot, acts PanelActions) {
	grokView := prefs.SelectedProvider == store.ProviderGrok
	ui.Column(c).Padding(14, 16, 10, 16).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Quosparr").FontSize(fsTitle3).FontWeight(600).SingleLine()
			if a.settings.System().Features.Grok {
				providerMenu(c, a, l, prefs.SelectedProvider, acts.SelectProvider)
			}
			ui.Spacer(c).Grow(1)
			panelHeaderButtons(c, a, l, prefs, acts, panelBusy(snap, grokView))
		})
		if grokView {
			grokIdentityRow(c, a, l, prefs, snap)
			return
		}
		codexIdentityRow(c, a, l, prefs, snap)
		if exp := snap.Accounts.Display.SubscriptionExpiresAt; exp != nil {
			ui.Row(c).Padding(6, 0, 0, 0).Children(func() { expiryBadge(c, l, *exp, time.Now()) })
		}
	})
}

func panelBusy(snap runway.Snapshot, grokView bool) bool {
	if grokView && snap.Grok.Refreshing {
		return true
	}
	return snap.Refresh.Refreshing()
}

// providerMenu is the compact dropdown next to the title (a menu-style
// picker). With no SelectProvider action it stays disabled.
func providerMenu(c *ui.Context, a *App, l l10n.Localizer, cur store.Provider, selectFn func(store.Provider)) {
	t := c.Theme()
	name := l.Text(l10n.KeyProviderCodex)
	if cur == store.ProviderGrok {
		name = l.Text(l10n.KeyProviderGrok)
	}
	b := ui.ButtonBase(c).Label(name).Tooltip(l.Text(l10n.KeyProviderCodex)+" / "+l.Text(l10n.KeyProviderGrok)).
		Disabled(selectFn == nil).Padding(0, 9, 0, 12).Gap(8).Height(24).Radius(radiusRow).AlignItems(ui.Center).Cursor(ui.CursorPointer)
	fill := mono(t, 0.07, 0.11)
	if b.Hovered() {
		fill = mono(t, 0.10, 0.15)
	}
	b.Background(fill)
	b.Children(func() {
		ui.Text(c, name).FontSize(fsBody).SingleLine()
		chevronUpDown(c, inkSecondary(t))
	})
	b.Menu(func(m *ui.Menu) {
		for _, p := range store.Providers() {
			label := l.Text(l10n.KeyProviderCodex)
			if p == store.ProviderGrok {
				label = l.Text(l10n.KeyProviderGrok)
			}
			if m.Item(label).Checked(p == cur).Chosen() && selectFn != nil && p != cur {
				a.panelState.detail = ""
				selectFn(p)
			}
		}
	})
}

// chevronUpDown is the two small chevrons of a pop-up button.
func chevronUpDown(c *ui.Context, col ui.Color) {
	ui.Box(c).Size(8, 12).Draw(func(p *ui.Painter, r ui.Rect) {
		cx, cy := r.X+r.W/2, r.Y+r.H/2
		var arrows ui.Path
		arrows.MoveTo(cx-3, cy-1.5).LineTo(cx, cy-4.5).LineTo(cx+3, cy-1.5)
		arrows.MoveTo(cx-3, cy+1.5).LineTo(cx, cy+4.5).LineTo(cx+3, cy+1.5)
		p.StrokePath(&arrows, 1.4, col)
	})
}

func panelHeaderButtons(c *ui.Context, a *App, l l10n.Localizer, prefs store.Preferences, acts PanelActions, busy bool) {
	ui.Row(c).Gap(2).AlignItems(ui.Center).Children(func() {
		privacy := prefs.AccountPrivacyModeEnabled
		if privacyButton(c, l, privacy, 26, 24, radiusControl).Clicked() {
			a.settings.SetAccountPrivacyMode(!privacy)
		}
		check := a.opts.Actions.CheckForUpdates
		if toolButton(c, icCloudDown, l.Text(l10n.KeyCheckForUpdates), 26, 24, 14, check == nil).Clicked() && check != nil {
			check()
		}
		if toolButton(c, icGithub, "GitHub", 26, 24, 14, false).Clicked() {
			c.OpenURL(githubURL)
		}
		if busy {
			ui.Row(c).Size(26, 24).AlignItems(ui.Center).Justify(ui.Center).Children(func() { ui.Spinner(c).Size(14, 14) })
			return
		}
		if toolButton(c, icRefresh, l.Text(l10n.KeyRefresh), 26, 24, 14, acts.Refresh == nil).Clicked() && acts.Refresh != nil {
			refreshEverything(a, acts)
		}
	})
}

// refreshEverything is the header refresh: all sections, plus the open
// cost query or reaction when a page that shows them is open.
func refreshEverything(a *App, acts PanelActions) {
	acts.Refresh()
	if a.panelState.detail == store.ModuleAPICost && a.panelState.cost.query != nil {
		a.startCostQuery(*a.panelState.cost.query, acts)
	}
}

// headerIdentity is the name of the signed-in account; privacy mode masks
// it as soon as there is a real identity to mask.
func headerIdentity(l l10n.Localizer, prefs store.Preferences, snap runway.Snapshot) string {
	if prefs.SelectedProvider == store.ProviderGrok {
		return grokIdentity(l, prefs, snap)
	}
	name := snap.Accounts.Display.DisplayName
	if name == "" {
		name = activeManagedName(snap.Accounts)
	}
	if name == "" {
		if snap.Accounts.Display.Authenticated {
			return l.Text(l10n.KeyUnknownAccount)
		}
		return l.Text(l10n.KeyNotLoggedIn)
	}
	if prefs.AccountPrivacyModeEnabled {
		return codex.MaskAccountText(name)
	}
	return name
}

func activeManagedName(v runway.AccountsView) string {
	for _, acc := range v.Sidebar {
		if acc.ID == v.ActiveID {
			return acc.ResolvedDisplayName()
		}
	}
	return ""
}

func grokIdentity(l l10n.Localizer, prefs store.Preferences, snap runway.Snapshot) string {
	name := snap.Grok.Accounts.Display.DisplayName
	if name == "" {
		return l.Text(l10n.KeyGrokNotLoggedIn)
	}
	if prefs.AccountPrivacyModeEnabled {
		return codex.MaskAccountText(name)
	}
	return name
}

// identityPill is the hover pill around the identity row; the negative
// margin keeps the badge on the 16pt grid while the pill breathes.
func identityPill(c *ui.Context, label string) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Label(label).Tooltip(label).Padding(3, 6).Gap(6).Radius(radiusRow).AlignItems(ui.Center).Justify(ui.Start).Margin(8, -6, 0, -6).Cursor(ui.CursorPointer)
	if b.Hovered() {
		b.Background(hoverNeutral(t))
	}
	return b
}

func identityChevron(c *ui.Context) {
	ui.Icon(c, icChevRight).Size(10, 10).TextColor(inkTertiary(c.Theme()))
}

func codexIdentityRow(c *ui.Context, a *App, l l10n.Localizer, prefs store.Preferences, snap runway.Snapshot) {
	tier := snap.Accounts.Display.Tier
	look := codexTierLook(c.Theme(), tier)
	name := headerIdentity(l, prefs, snap)
	row := identityPill(c, l.Text(l10n.KeyAccounts))
	row.Children(func() {
		tierBadge(c, look, l.Text(accountTierKey(tier)))
		tierText(c, look, name).MaxWidth(200)
		identityChevron(c)
	})
	if row.Clicked() {
		a.panelState.detail = detailAccounts
	}
}

func grokIdentityRow(c *ui.Context, a *App, l l10n.Localizer, prefs store.Preferences, snap runway.Snapshot) {
	row := identityPill(c, l.Text(l10n.KeyGrokAccountsTitle))
	row.Children(func() {
		grokPlanBadge(c, l, snap.Grok.Plan)
		ui.Text(c, headerIdentity(l, prefs, snap)).FontSize(fsCallout).FontWeight(600).SingleLine().Ellipsis("…").MaxWidth(220)
		identityChevron(c)
	})
	if row.Clicked() {
		a.panelState.detail = detailAccounts
	}
}

// grokPlanBadge is the tier plate of a Grok plan, or the neutral provider
// tag while the plan is unknown.
func grokPlanBadge(c *ui.Context, l l10n.Localizer, plan string) {
	if plan == "" {
		tierBadge(c, plainLook(c.Theme()), l.Text(l10n.KeyProviderGrok))
		return
	}
	kind := grok.TierResolve(plan)
	tierBadge(c, grokTierLook(c.Theme(), kind), kind.DisplayName())
}
