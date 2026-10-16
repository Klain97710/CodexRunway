package app

import (
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/runway"
)

// creditsDetail is the reset credits page: summary row, four stat cards,
// the expiry risk bar and the credit table.
func creditsDetail(c *ui.Context, l l10n.Localizer, v runway.CodexView) {
	sum := v.ResetCreditSummary
	if sum == nil {
		calloutDim(c, l.Text(l10n.KeyNotLoaded)).FontSize(fsBody)
		return
	}
	ui.Column(c).Gap(16).Children(func() {
		pageSummaryRow(c, l.Text(l10n.KeyResetCredits),
			l.Text(l10n.KeyLastUpdated)+" "+runway.CreditUpdatedText(sum.UpdatedAt, l.Language(), nil),
			strconv.Itoa(sum.AvailableCount)+"/"+strconv.Itoa(sum.TotalCount))
		creditStats(c, l, *sum)
		creditRisk(c, l, *sum)
		creditTable(c, l, v)
	})
}

func creditStats(c *ui.Context, l l10n.Localizer, sum runway.ResetCreditSummary) {
	t := c.Theme()
	lang := l.Language()
	remaining := func(at time.Time, secs float64) *float64 {
		if at.IsZero() {
			return nil
		}
		return &secs
	}
	next := runway.ExpiryDuration(remaining(sum.NextExpiry, sum.NextExpiryRemaining), lang)
	latest := runway.ExpiryDuration(remaining(sum.LatestExpiry, sum.LatestExpiryRemaining), lang)
	statGrid(c,
		func() { statCard(c, l.Text(l10n.KeyAvailable), strconv.Itoa(sum.AvailableCount), sys(t, sysGreen)) },
		func() { statCard(c, l.Text(l10n.KeyExpiringSoon), strconv.Itoa(sum.ExpiringCount), sys(t, sysYellow)) },
		func() { statCard(c, l.Text(l10n.KeyNextExpiry), next, sys(t, sysOrange)) },
		func() { statCard(c, l.Text(l10n.KeyLatestExpiry), latest, sys(t, sysBlue)) },
	)
}

// creditRisk is the three-segment expiry risk bar and its legend.
func creditRisk(c *ui.Context, l l10n.Localizer, sum runway.ResetCreditSummary) {
	t := c.Theme()
	green, yellow, red := sys(t, sysGreen), sys(t, sysYellow), sys(t, sysRed)
	ui.Column(c).Gap(8).Children(func() {
		ui.Text(c, l.Text(l10n.KeyExpiryRisk)).FontSize(fsHeadline).FontWeight(600)
		riskBar(c, sum, green, yellow, red)
		ui.Row(c).Gap(10).Children(func() {
			riskLegend(c, l.Text(l10n.KeyAvailable), sum.StableAvailableCount, green)
			riskLegend(c, l.Text(l10n.KeyExpiringSoon), sum.ExpiringCount, yellow)
			riskLegend(c, l.Text(l10n.KeyUnavailableCredits), sum.UnavailableCount, red)
		})
	})
}

func riskBar(c *ui.Context, sum runway.ResetCreditSummary, green, yellow, red ui.Color) {
	ui.Box(c).Height(8).Draw(func(p *ui.Painter, r ui.Rect) {
		x := r.X
		for _, seg := range []struct {
			n int
			c ui.Color
		}{{sum.StableAvailableCount, green}, {sum.ExpiringCount, yellow}, {sum.UnavailableCount, red}} {
			if seg.n <= 0 || sum.TotalCount <= 0 {
				continue
			}
			w := max(8, r.W*float32(seg.n)/float32(sum.TotalCount))
			p.Fill(ui.Rect{X: x, Y: r.Y, W: w, H: r.H}, seg.c, 3)
			x += w + 2
		}
	})
}

func riskLegend(c *ui.Context, title string, count int, color ui.Color) {
	ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Size(5, 5).Radius(2.5).Background(color)
		ui.Text(c, title+" "+strconv.Itoa(count)).FontSize(fsCaption).FontFeatures("tnum").TextColor(inkSecondary(c.Theme())).SingleLine()
	})
}

func creditTable(c *ui.Context, l l10n.Localizer, v runway.CodexView) {
	rows := v.CreditDetails
	if len(rows) == 0 && v.ResetCredits != nil {
		rows = runway.CreditDetails(l, *v.ResetCredits)
	}
	ui.Column(c).Gap(8).Children(func() {
		ui.Text(c, l.Text(l10n.KeyResetCreditDetails)).FontSize(fsHeadline).FontWeight(600)
		if len(rows) == 0 {
			calloutDim(c, l.Text(l10n.KeyNoAvailableResetCredits))
			return
		}
		tableContainer(c, func() { creditTableHead(c, l) }, func() {
			for i, row := range rows {
				creditTableRow(c, l, row, i == 0)
			}
		})
	})
}

// Column widths of the credit table.
const (
	creditColTitle     float32 = 62
	creditColRemaining float32 = 72
)

func creditTableHead(c *ui.Context, l l10n.Localizer) {
	t := c.Theme()
	ui.Row(c).Gap(8).Padding(7, 10).Grow(1).Children(func() {
		head := func(s string) ui.Element {
			return ui.Text(c, s).FontSize(fsCaption).FontWeight(600).TextColor(inkSecondary(t)).SingleLine()
		}
		head(l.Text(l10n.KeyCredit)).Width(creditColTitle)
		head(l.Text(l10n.KeyStatus)).Grow(1).Basis(0)
		head(l.Text(l10n.KeyExpiresAt)).Grow(1).Basis(0).TextAlign(ui.End)
		head(l.Text(l10n.KeyLeft)).Width(creditColRemaining).TextAlign(ui.End)
	})
}

func creditTableRow(c *ui.Context, l l10n.Localizer, row runway.CreditDetail, first bool) {
	t := c.Theme()
	tableRowRule(c, first)
	ui.Row(c).Gap(8).Padding(6, 10).AlignItems(ui.Center).Children(func() {
		ui.Text(c, row.Title).FontSize(fsCaption).FontFeatures("tnum").Width(creditColTitle).SingleLine()
		ui.Row(c).Grow(1).Basis(0).Children(func() { runwayTag(c, creditStatus(l, row), creditTone(row.State)) })
		expiry, left := l.Text(l10n.KeyNoExpiry), "--"
		if !row.ExpiresAt.IsZero() {
			expiry = runway.ResetCreditExpiresAt(row.ExpiresAt, l.Language(), nil)
			left = runway.DurationLocalized(row.RemainingDuration.Seconds(), l.Language(), false)
		}
		ui.Text(c, expiry).FontSize(fsCaption).FontFeatures("tnum").TextColor(inkSecondary(t)).Grow(1).Basis(0).TextAlign(ui.End).SingleLine()
		ui.Text(c, left).FontSize(fsCaption).FontFeatures("tnum").TextColor(inkSecondary(t)).Width(creditColRemaining).TextAlign(ui.End).SingleLine()
	})
}

func creditStatus(l l10n.Localizer, row runway.CreditDetail) string {
	if row.State == runway.CreditExpiring {
		return l.Text(l10n.KeyExpiringSoon)
	}
	return row.StatusText
}

func creditTone(state runway.CreditRisk) tagTone {
	switch state {
	case runway.CreditExpiring:
		return toneOrange
	case runway.CreditUnavailable:
		return toneRed
	}
	return toneGreen
}
