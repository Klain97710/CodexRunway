package app

import (
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/runway"
	"github.com/licoy/quosparr/internal/speed"
)

// Figure columns of the speed table; the model takes the rest of the row.
const (
	speedColPercentile float32 = 44
	speedColOutput     float32 = 52
	speedColCalls      float32 = 40
)

// speedDetail is the speed analysis page: the table by model and effort,
// the analysed sessions newest first and a note on range and method.
func speedDetail(c *ui.Context, l l10n.Localizer, v runway.CodexView) {
	r := v.Speed
	switch {
	case r == nil && v.SpeedError != "":
		ui.Text(c, v.SpeedError).FontSize(fsBody).TextColor(sys(c.Theme(), sysRed))
		return
	case r == nil:
		calloutDim(c, l.Text(l10n.KeyNotLoaded)).FontSize(fsBody)
		return
	case r.Empty():
		calloutDim(c, tx(l, xSpeedEmpty)).FontSize(fsBody)
		speedError(c, v.SpeedError)
		return
	}
	ui.Column(c).Gap(16).Children(func() {
		speedModelTable(c, l, r.Groups, !r.NoTTFT)
		speedSessionList(c, l, r.Sessions, !r.NoTTFT)
		speedNote(c, l, *r, v.SpeedError)
	})
}

// speedModelTable leaves the TTFT and call count columns out unless ttft:
// the call count only counts samples with a TTFT.
func speedModelTable(c *ui.Context, l l10n.Localizer, groups []speed.Stat, ttft bool) {
	ui.Column(c).Gap(8).Children(func() {
		ui.Text(c, tx(l, xSpeedByModel)).FontSize(fsHeadline).FontWeight(600)
		tableContainer(c, func() { speedHeadRow(c, l, ttft) }, func() {
			for i, g := range groups {
				speedTableRow(c, l, g, i == 0, ttft)
			}
		})
	})
}

// speedHeadRow stacks the metric over its percentile or unit so that the
// figure columns stay narrow in every language.
func speedHeadRow(c *ui.Context, l l10n.Localizer, ttft bool) {
	t := c.Theme()
	head := func(s string) ui.Element {
		return ui.Text(c, s).FontSize(fsCaption).FontWeight(600).TextColor(inkSecondary(t)).SingleLine().TextAlign(ui.End)
	}
	stacked := func(top, bottom string, width float32) {
		ui.Column(c).Width(width).Shrink(0).Children(func() {
			head(top)
			head(bottom)
		})
	}
	label := tx(l, xSpeedTTFT)
	ui.Row(c).Gap(6).Padding(7, 10).Grow(1).AlignItems(ui.End).Children(func() {
		head(tx(l, xSpeedModel)).TextAlign(ui.Start).Grow(1).Basis(0)
		if ttft {
			stacked(label, "P50", speedColPercentile)
			stacked(label, "P90", speedColPercentile)
		}
		stacked(tx(l, xSpeedOutput), "tok/s", speedColOutput)
		if ttft {
			head(tx(l, xSpeedCalls)).Width(speedColCalls).Shrink(0)
		}
	})
}

func speedTableRow(c *ui.Context, l l10n.Localizer, s speed.Stat, first, ttft bool) {
	ui.Column(c).Children(func() {
		tableRowRule(c, first)
		ui.Row(c).Gap(6).Padding(6, 10).AlignItems(ui.Center).Children(func() {
			speedModelCell(c, l, s)
			cell := func(text string, width float32) {
				ui.Text(c, text).FontSize(fsCaption).FontFeatures("tnum").Width(width).Shrink(0).TextAlign(ui.End).SingleLine()
			}
			if ttft {
				cell(speedTTFT(s.TTFTP50, s.Calls), speedColPercentile)
				cell(speedTTFT(s.TTFTP90, s.Calls), speedColPercentile)
			}
			cell(speedRate(s, false), speedColOutput)
			if ttft {
				cell(strconv.Itoa(s.Calls), speedColCalls)
			}
		})
	})
}

func speedSessionList(c *ui.Context, l l10n.Localizer, list []speed.SessionStat, ttft bool) {
	if len(list) == 0 {
		return
	}
	ui.Column(c).Gap(8).Children(func() {
		ui.Text(c, l.Text(l10n.KeyRecentSessions)).FontSize(fsHeadline).FontWeight(600)
		for _, s := range list {
			speedSessionCard(c, l, s, ttft)
		}
	})
}

// speedSessionCard is one conversation: its title, project and age over one
// line per model.
func speedSessionCard(c *ui.Context, l l10n.Localizer, s speed.SessionStat, ttft bool) {
	t := c.Theme()
	title := speedOneLine(s.Title)
	if title == "" {
		title = l.Text(l10n.KeyUnknown)
	}
	cardColumn(c, surfaceSunkenKind, radiusCard).Children(func() {
		ui.Column(c).Gap(2).Padding(8, 10).Children(func() {
			ui.Text(c, title).FontSize(fsCallout).FontWeight(500).SingleLine().Ellipsis("…").Tooltip(title)
			if meta := speedSessionMeta(l, s); meta != "" {
				ui.Text(c, meta).FontSize(fsCaption).TextColor(inkSecondary(t)).SingleLine().Ellipsis("…")
			}
		})
		for _, g := range s.Groups {
			speedGroupRow(c, l, g, false, ttft)
		}
	})
}

// speedSessionMeta is "project · 5 minutes ago", leaving out what is unknown.
func speedSessionMeta(l l10n.Localizer, s speed.SessionStat) string {
	var parts []string
	if project := speedOneLine(s.ProjectName); project != "" {
		parts = append(parts, project)
	}
	if !s.UpdatedAt.IsZero() {
		parts = append(parts, runway.RelativePast(s.UpdatedAt, time.Now(), l.Language(), false))
	}
	return strings.Join(parts, " · ")
}

// speedNote is the grey footer: the range the samples cover, what the
// figures mean and how they were measured.
func speedNote(c *ui.Context, l l10n.Localizer, r speed.Report, errText string) {
	t := c.Theme()
	ui.Column(c).Gap(6).Children(func() {
		if !r.From.IsZero() && !r.To.IsZero() {
			dimText(c, tx(l, xSpeedRange, r.From.Local().Format(time.DateOnly), r.To.Local().Format(time.DateOnly)))
		}
		for _, text := range speedInfo(l, &r) {
			ui.Text(c, text).FontSize(fsCaption).TextColor(inkSecondary(t))
		}
		speedError(c, errText)
	})
}
