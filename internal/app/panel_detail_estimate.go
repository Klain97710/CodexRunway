package app

import (
	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/estimate"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/runway"
)

// estimateDetail is the quota estimate page: the weekly allowance, four
// stat cards, the week table with its total row and the cycle history.
func estimateDetail(c *ui.Context, l l10n.Localizer, v runway.CodexView) {
	t := c.Theme()
	s := v.Estimate
	switch {
	case s == nil && v.EstimateError != "":
		ui.Text(c, v.EstimateError).FontSize(fsBody).TextColor(sys(t, sysRed))
		return
	case s == nil:
		ui.Text(c, l.Text(l10n.KeyNotLoaded)).FontSize(fsBody).TextColor(inkSecondary(t))
		return
	}
	ui.Column(c).Gap(16).Children(func() {
		allowance := "—"
		if s.EstimatedCredits != nil {
			allowance = estCredits(*s.EstimatedCredits)
		}
		pageSummaryRow(c, l.Text(l10n.KeyQuotaEstimateWeeklyAllowance), estimateMethod(l, *s), allowance)
		estimateStats(c, l, *s)
		if s.UnavailableReason != nil {
			calloutDim(c, runway.EstimateUnavailableText(l, *s.UnavailableReason))
		}
		ui.Column(c).Gap(6).Children(func() {
			dimText(c, runway.EstimateDataTimeText(l, *s))
			if msg := runway.EstimateRefreshErrorText(l, v.EstimateError); msg != "" {
				ui.Text(c, msg).FontSize(fsCaption).TextColor(sys(t, sysRed))
			}
		})
		ui.Text(c, l.Text(l10n.KeyQuotaEstimateInfo)).FontSize(fsCaption).TextColor(inkSecondary(t))
		estimateWeekTable(c, l, *s)
		if hist := estimateHistoryRows(*s); len(hist) > 0 {
			estimateHistoryTable(c, l, hist)
		}
	})
}

func estimateMethod(l l10n.Localizer, s estimate.Snapshot) string {
	mode := l.Text(l10n.KeyQuotaEstimateMethodOfficial)
	if s.WindowMode == estimate.ModeRollingWeek {
		mode = l.Text(l10n.KeyQuotaEstimateMethodRolling)
	}
	return mode + " · " + s.CycleStartDate
}

func estimateStats(c *ui.Context, l l10n.Localizer, s estimate.Snapshot) {
	t := c.Theme()
	dash := func(ok bool, text string) string {
		if !ok {
			return "—"
		}
		return text
	}
	var allowance, usd string
	if s.EstimatedCredits != nil {
		allowance = estCredits(*s.EstimatedCredits)
	}
	if s.EstimatedUSD != nil {
		usd = estUSD(*s.EstimatedUSD)
	}
	statGrid(c,
		func() {
			statCard(c, l.Text(l10n.KeyQuotaEstimateUsedPercent), estPercent(s.UsedPercent), sys(t, sysBlue))
		},
		func() {
			statCard(c, l.Text(l10n.KeyQuotaEstimateThisWeekUsed), dash(s.CreditsComplete(), estCredits(s.UsedCredits)), sys(t, sysOrange))
		},
		func() {
			statCard(c, l.Text(l10n.KeyQuotaEstimateWeeklyAllowance), dash(s.EstimatedCredits != nil, allowance), sys(t, sysGreen))
		},
		func() {
			statCard(c, l.Text(l10n.KeyQuotaEstimateWeekValue), dash(s.EstimatedUSD != nil, usd), sys(t, sysTeal))
		},
	)
}

const estimateColDate float32 = 78

func estimateWeekTable(c *ui.Context, l l10n.Localizer, s estimate.Snapshot) {
	ui.Column(c).Gap(8).Children(func() {
		ui.Text(c, l.Text(l10n.KeyQuotaEstimateThisWeek)).FontSize(fsHeadline).FontWeight(600)
		if len(s.CurrentRows) == 0 {
			calloutDim(c, l.Text(l10n.KeyUsageAnalyticsEmpty))
			return
		}
		tableContainer(c, func() {
			estimateHeadRow(c, l.Text(l10n.KeyDate), l.Text(l10n.KeyQuotaEstimateCredits), l.Text(l10n.KeyTokens), l.Text(l10n.KeyEstimatedAPICost))
		}, func() {
			for i := len(s.CurrentRows) - 1; i >= 0; i-- {
				estimateWeekRow(c, s.CurrentRows[i], i == len(s.CurrentRows)-1)
			}
			estimateWeekTotal(c, l, s)
		})
	})
}

func estimateHeadRow(c *ui.Context, date, a, b, d string) {
	t := c.Theme()
	ui.Row(c).Gap(8).Padding(7, 10).Grow(1).Children(func() {
		head := func(s string) ui.Element {
			return ui.Text(c, s).FontSize(fsCaption).FontWeight(600).TextColor(inkSecondary(t)).SingleLine()
		}
		head(date).Width(estimateColDate)
		for _, s := range []string{a, b, d} {
			head(s).Grow(1).Basis(0).TextAlign(ui.End)
		}
	})
}

// numberRow lays out a date cell and three right-aligned figures.
func numberRow(c *ui.Context, date string, weight int, cells ...string) {
	ui.Row(c).Gap(8).Padding(6, 10).Children(func() {
		ui.Text(c, date).FontSize(fsCaption).FontWeight(weight).FontFeatures("tnum").Width(estimateColDate).SingleLine()
		for _, s := range cells {
			ui.Text(c, s).FontSize(fsCaption).FontWeight(weight).FontFeatures("tnum").Grow(1).Basis(0).TextAlign(ui.End).SingleLine()
		}
	})
}

func estimateWeekRow(c *ui.Context, r estimate.Row, first bool) {
	tableRowRule(c, first)
	credits, tokens, usd := "—", "—", "—"
	if r.CreditsReported {
		credits = estTableCredits(r.Credits)
	}
	if r.TotalsReported {
		tokens = estimate.CompactTokens(r.Tokens)
	}
	if r.USD != nil {
		usd = estUSD(*r.USD)
	}
	numberRow(c, r.Date, 400, credits, tokens, usd)
}

func estimateWeekTotal(c *ui.Context, l l10n.Localizer, s estimate.Snapshot) {
	tableRowRule(c, false)
	credits, tokens, usd := "—", "—", "—"
	if s.CreditsComplete() {
		credits = estTableCredits(s.UsedCredits)
	}
	all, sum := true, 0
	for _, r := range s.CurrentRows {
		all = all && r.TotalsReported
		sum += r.Tokens
	}
	if all {
		tokens = estimate.CompactTokens(sum)
	}
	if v := s.APIEquivalentUSD(); v != nil {
		usd = estUSD(*v)
	}
	numberRow(c, l.Text(l10n.KeyTotal), 600, credits, tokens, usd)
}

// estimateHistoryRow is one past cycle with its change against the one before.
type estimateHistoryRow struct {
	cycle   string
	credits float64
	change  *float64
}

// estimateHistoryRows is the history newest first, each with the change
// against the previous (older) cycle.
func estimateHistoryRows(s estimate.Snapshot) []estimateHistoryRow {
	rows := make([]estimateHistoryRow, len(s.History))
	for i, h := range s.History {
		rows[i] = estimateHistoryRow{cycle: h.CycleStartDate, credits: h.EstimatedCredits}
		if i > 0 && s.History[i-1].EstimatedCredits > 0 {
			prev := s.History[i-1].EstimatedCredits
			delta := (h.EstimatedCredits - prev) / prev * 100
			rows[i].change = &delta
		}
	}
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
	return rows
}

const estimateColChange float32 = 72

func estimateHistoryTable(c *ui.Context, l l10n.Localizer, rows []estimateHistoryRow) {
	t := c.Theme()
	ui.Column(c).Gap(8).Children(func() {
		ui.Text(c, l.Text(l10n.KeyQuotaEstimateHistory)).FontSize(fsHeadline).FontWeight(600)
		tableContainer(c, func() {
			ui.Row(c).Gap(8).Padding(7, 10).Grow(1).Children(func() {
				head := func(s string) ui.Element {
					return ui.Text(c, s).FontSize(fsCaption).FontWeight(600).TextColor(inkSecondary(t)).SingleLine()
				}
				head(l.Text(l10n.KeyDate)).Grow(1).Basis(0)
				head(l.Text(l10n.KeyQuotaEstimateWeeklyAllowance)).Grow(1).Basis(0).TextAlign(ui.End)
				head(l.Text(l10n.KeyQuotaEstimateVsLast)).Width(estimateColChange).TextAlign(ui.End)
			})
		}, func() {
			for i, r := range rows {
				tableRowRule(c, i == 0)
				change := "—"
				if r.change != nil {
					change = estSignedPercent(*r.change)
				}
				ui.Row(c).Gap(8).Padding(6, 10).Children(func() {
					ui.Text(c, r.cycle).FontSize(fsCaption).FontFeatures("tnum").Grow(1).Basis(0).SingleLine()
					ui.Text(c, estCredits(r.credits)).FontSize(fsCaption).FontFeatures("tnum").Grow(1).Basis(0).TextAlign(ui.End).SingleLine()
					ui.Text(c, change).FontSize(fsCaption).FontFeatures("tnum").TextColor(inkSecondary(t)).Width(estimateColChange).TextAlign(ui.End).SingleLine()
				})
			}
		})
	})
}
