package app

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/claude"
	"github.com/licoy/quosparr/internal/codex"
	"github.com/licoy/quosparr/internal/grok"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/runway"
	"github.com/licoy/quosparr/internal/store"
)

type accountIdentityView struct {
	account          codex.ManagedAccount
	provider         store.Provider
	current, privacy bool
	grokQuota        *runway.GrokAccountQuota
	claudeQuota      *runway.ClaudeAccountQuota
}

func accountSettingsIdentity(c *ui.Context, l l10n.Localizer, v accountIdentityView) {
	acc := v.account
	name := acc.ResolvedDisplayName()
	ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(4).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Text(c, maskAccount(v.privacy, name)).FontSize(13).FontWeight(600).SingleLine().MinWidth(0).Shrink(1).Tooltip(maskAccount(v.privacy, name))
			if v.current {
				runwayTagPadded(c, l.Text(l10n.KeyAccountsCurrent), toneGreen, 1, 5)
			}
			if v.provider == store.ProviderCodex {
				tierBadgePadded(c, codexTierLook(c.Theme(), acc.Tier()), l.Text(accountTierKey(acc.Tier())), 1, 5)
			} else if v.grokQuota != nil && v.grokQuota.Plan != "" {
				kind := grok.TierResolve(v.grokQuota.Plan)
				tierBadgePadded(c, grokTierLook(c.Theme(), kind), kind.DisplayName(), 1, 5)
			} else if v.claudeQuota != nil && v.claudeQuota.Plan != "" {
				tierBadgePadded(c, claudeTierLook(c.Theme(), claude.TierResolve(v.claudeQuota.Plan)), v.claudeQuota.Plan, 1, 5)
			}
		})
		if acc.Email != "" && acc.Email != name {
			dimText(c, maskAccount(v.privacy, acc.Email)).SingleLine().Tooltip(maskAccount(v.privacy, acc.Email))
		}
		if v.provider == store.ProviderCodex {
			codexIdentityDetails(c, l, acc, v.privacy)
		}
		if acc.Note != "" {
			dimText(c, accountDiagnosticText(acc.Note, v.privacy)).MaxLines(2)
		}
		accountSettingsStatus(c, l, v)
	})
}

// Match the Swift settings rows: identity, warnings, then cached quota and
// its reset countdown. Errors take precedence over stale quota readings.
func accountSettingsStatus(c *ui.Context, l l10n.Localizer, v accountIdentityView) {
	acc := v.account
	switch {
	case acc.RequiresReauth:
		key := l10n.KeyAccountsNeedsReauth
		if v.provider == store.ProviderGrok {
			key = l10n.KeyGrokReauthenticationRequired
		}
		ui.Text(c, l.Text(key)).FontSize(11).TextColor(c.Theme().Danger)
	case acc.LastError != "":
		ui.Text(c, accountDiagnosticText(acc.LastError, v.privacy)).FontSize(11).TextColor(sys(c.Theme(), sysOrange)).MaxLines(2).Tooltip(accountDiagnosticText(acc.LastError, v.privacy))
	case v.provider == store.ProviderClaude && v.claudeQuota != nil:
		q := v.claudeQuota
		percent := fmt.Sprintf("%d%% %s", max(0, min(100, 100-q.UsedPercent)), l.Text(l10n.KeyLeft))
		text := accountUpdatedSummary(l, percent, q.UpdatedAt)
		if q.ExtraUsage != "" {
			text = q.ExtraUsage + " · " + percent
		}
		ui.Row(c).Gap(10).AlignItems(ui.Center).Wrap().Children(func() {
			dimText(c, text).NoWrap().Shrink(0)
			accountSettingsReset(c, l, q.ResetsAt)
		})
	case v.provider == store.ProviderGrok && v.grokQuota != nil:
		q := v.grokQuota
		percent := fmt.Sprintf("%d%% %s", max(0, min(100, 100-q.UsedPercent)), l.Text(l10n.KeyLeft))
		text := accountUpdatedSummary(l, percent, q.UpdatedAt)
		if q.IncludedUSD != "" {
			text = q.IncludedUSD + " · " + percent
		}
		ui.Row(c).Gap(10).AlignItems(ui.Center).Wrap().Children(func() {
			dimText(c, text).NoWrap().Shrink(0)
			accountSettingsReset(c, l, q.ResetsAt)
		})
	case v.provider == store.ProviderCodex && acc.CachedQuota != nil:
		q := acc.CachedQuota
		ui.Row(c).Gap(10).AlignItems(ui.Center).Wrap().Children(func() {
			dimText(c, accountUpdatedSummary(l, fmt.Sprintf("%d%%", max(0, min(100, 100-q.PrimaryUsedPercent))), q.UpdatedAt)).NoWrap().Shrink(0)
			if q.PrimaryResetsAt != nil {
				accountSettingsReset(c, l, *q.PrimaryResetsAt)
			}
		})
	}
}

func accountUpdatedSummary(l l10n.Localizer, text string, at time.Time) string {
	if at.IsZero() {
		return text
	}
	return text + " · " + l.Text(l10n.KeyLastUpdated) + " " + at.In(time.Local).Format("15:04")
}

func accountSettingsReset(c *ui.Context, l l10n.Localizer, at time.Time) {
	if at.IsZero() {
		return
	}
	dimText(c, resetCountdown(l, at, time.Now())).FontFeatures("tnum").NoWrap().Shrink(0)
	c.After(time.Second)
}

func accountDiagnosticText(text string, privacy bool) string {
	if privacy {
		return codex.MaskEmailsIn(text)
	}
	return text
}
