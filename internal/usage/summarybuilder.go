package usage

import (
	"sort"
	"time"

	"github.com/licoy/quosparr/internal/usage/decimal"
)

// buildSummary aggregates index events into a Summary for window. Models the
// price book does not know are listed in warnings and left out of the cost, so
// a partly priced window never claims an exact total.
func buildSummary(events []indexedEvent, window Window, calculatedAt time.Time, book PriceBook, warnings []string) (Summary, error) {
	g, err := groupEvents(events)
	if err != nil {
		return Summary{}, err
	}
	var unknown []string
	for _, m := range sortedKeys(g.byModel) {
		if _, ok := book.Price(m); !ok {
			unknown = append(unknown, m)
		}
	}
	return summaryFromGroups(g, window, calculatedAt, book, unknown, warnings)
}

// summaryFromGroups turns grouped totals into a Summary. unknown lists the
// models to report as "unknown-model:<name>" warnings, ahead of warnings.
func summaryFromGroups(g eventGroups, window Window, calculatedAt time.Time, book PriceBook, unknown, warnings []string) (Summary, error) {
	dayTotals := make([]Totals, 0, len(g.byDay))
	for _, t := range g.byDay {
		dayTotals = append(dayTotals, t)
	}
	totals, err := sumTotals(dayTotals)
	if err != nil {
		return Summary{}, err
	}
	models := sortedKeys(g.byModel)
	estimated := estimatedCost(g.byModel, book)
	confidence := ConfidencePriced
	switch {
	case totals.TotalTokens == 0:
		confidence = ConfidenceUnavailable
	case estimated == nil:
		confidence = ConfidenceTokensOnly
	}
	source := SourceUnavailable
	if totals.TotalTokens > 0 {
		source = SourceLocalSessions
	}
	projectRows, err := projectRows(g.byProjectModel, book)
	if err != nil {
		return Summary{}, err
	}
	out := Summary{
		Source:         source,
		Confidence:     confidence,
		Window:         window,
		EstimatedUSD:   estimated,
		Totals:         totals,
		DailyRows:      dailyRows(g, book),
		ModelRows:      modelRows(g.byModel, models, book),
		ProjectRows:    projectRows,
		ClientRows:     []BreakdownRow{},
		Warnings:       []string{},
		PricingVersion: book.Version(),
		CalculatedAt:   calculatedAt,
	}
	for _, m := range unknown {
		out.Warnings = append(out.Warnings, "unknown-model:"+m)
	}
	out.Warnings = append(out.Warnings, warnings...)
	return out, nil
}

func newEventGroups() eventGroups {
	return eventGroups{
		byModel:        map[string]Totals{},
		byProjectModel: map[string]map[string]Totals{},
		byDay:          map[string]Totals{},
		byDayModel:     map[string]map[string]Totals{},
	}
}

// add accumulates t under the four groupings.
func (g eventGroups) add(model, project, day string, t Totals) error {
	if err := addInto(g.byModel, model, t); err != nil {
		return err
	}
	if err := addNested(g.byProjectModel, project, model, t); err != nil {
		return err
	}
	if err := addInto(g.byDay, day, t); err != nil {
		return err
	}
	return addNested(g.byDayModel, day, model, t)
}

type eventGroups struct {
	byModel        map[string]Totals
	byProjectModel map[string]map[string]Totals
	byDay          map[string]Totals
	byDayModel     map[string]map[string]Totals
}

func addInto(m map[string]Totals, key string, t Totals) error {
	sum, err := m[key].Add(t)
	if err != nil {
		return err
	}
	m[key] = sum
	return nil
}

func addNested(m map[string]map[string]Totals, outer, inner string, t Totals) error {
	if m[outer] == nil {
		m[outer] = map[string]Totals{}
	}
	return addInto(m[outer], inner, t)
}

func groupEvents(events []indexedEvent) (eventGroups, error) {
	g := newEventGroups()
	for _, e := range events {
		input, err := checkedAdd(e.Uncached, e.Cached, "input tokens")
		if err != nil {
			return g, err
		}
		total, err := checkedAdd(input, e.Output, "total tokens")
		if err != nil {
			return g, err
		}
		t := Totals{TotalTokens: total, UncachedInputTokens: e.Uncached, CachedInputTokens: e.Cached, OutputTokens: e.Output, Turns: e.Turns}
		if err := g.add(e.Model, e.Project, e.DayKey, t); err != nil {
			return g, err
		}
	}
	return g, nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func modelRows(byModel map[string]Totals, models []string, book PriceBook) []BreakdownRow {
	rows := make([]BreakdownRow, 0, len(models))
	for _, m := range models {
		t := byModel[m]
		row := BreakdownRow{Name: m, Totals: t}
		if c, ok := book.Cost(m, t); ok {
			row.EstimatedUSD = &c
		}
		rows = append(rows, row)
	}
	return rows
}

func projectRows(byProject map[string]map[string]Totals, book PriceBook) ([]BreakdownRow, error) {
	rows := make([]BreakdownRow, 0, len(byProject))
	for _, project := range sortedKeys(byProject) {
		byModel := byProject[project]
		all := make([]Totals, 0, len(byModel))
		for _, m := range sortedKeys(byModel) {
			all = append(all, byModel[m])
		}
		t, err := sumTotals(all)
		if err != nil {
			return nil, err
		}
		rows = append(rows, BreakdownRow{Name: project, Totals: t, EstimatedUSD: estimatedCost(byModel, book)})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Totals.TotalTokens != rows[j].Totals.TotalTokens {
			return rows[i].Totals.TotalTokens > rows[j].Totals.TotalTokens
		}
		return rows[i].Name < rows[j].Name
	})
	return rows, nil
}

func dailyRows(g eventGroups, book PriceBook) []DailyRow {
	days := sortedKeys(g.byDay)
	rows := make([]DailyRow, 0, len(days))
	for _, day := range days {
		rows = append(rows, DailyRow{Date: day, Totals: g.byDay[day], EstimatedUSD: estimatedCost(g.byDayModel[day], book)})
	}
	return rows
}

// estimatedCost prices every known model and sums them; it is nil when no
// model is known.
func estimatedCost(byModel map[string]Totals, book PriceBook) *decimal.Decimal {
	result := decimal.Zero
	priced := false
	for _, m := range sortedKeys(byModel) {
		c, ok := book.Cost(m, byModel[m])
		if !ok {
			continue
		}
		priced = true
		result = result.Add(c)
	}
	if !priced {
		return nil
	}
	return &result
}
