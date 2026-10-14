package usage

import (
	"bytes"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/licoy/quosparr/internal/usage/decimal"
)

// ErrInvalidAnalytics reports an analytics response that cannot be decoded.
var ErrInvalidAnalytics = errors.New("usage: invalid analytics response")

// WarningAnalyticsTokenPartsMissing is added when the online data lacks the
// token classes needed to price it.
const WarningAnalyticsTokenPartsMissing = "analytics-token-parts-missing"

type analyticsTotals struct {
	credits         float64
	creditsReported bool
	turns, threads  int64
	total           int64
	cached          int64
	uncached        int64
	output          int64
}

// field reads one of the totals members; ok=false means the key is not a
// totals member.
func (t *analyticsTotals) field(d *jsonDec, key string) (ok bool, err error) {
	var dst *int64
	switch key {
	case "credits":
		v, err := d.flex()
		if err != nil {
			return true, err
		}
		f, good := v.flexDouble()
		if !good {
			return true, ErrInvalidAnalytics
		}
		if f < 0 {
			return true, invalid("analytics credits")
		}
		t.credits, t.creditsReported = f, v.kind != flexNull
		return true, nil
	case "turns":
		dst = &t.turns
	case "threads":
		dst = &t.threads
	case "text_total_tokens":
		dst = &t.total
	case "cached_text_input_tokens":
		dst = &t.cached
	case "uncached_text_input_tokens":
		dst = &t.uncached
	case "text_output_tokens":
		dst = &t.output
	default:
		return false, nil
	}
	v, err := d.flex()
	if err != nil {
		return true, err
	}
	n, good := v.flexInt()
	if !good {
		return true, ErrInvalidAnalytics
	}
	*dst = n
	return true, nil
}

func (t analyticsTotals) checked() (Totals, error) {
	if t.turns < 0 || t.threads < 0 || t.total < 0 || t.cached < 0 || t.uncached < 0 || t.output < 0 {
		return Totals{}, invalid("analytics token usage")
	}
	input, err := checkedAdd(t.uncached, t.cached, "analytics input tokens")
	if err != nil {
		return Totals{}, err
	}
	parts, err := checkedAdd(input, t.output, "analytics total tokens")
	if err != nil {
		return Totals{}, err
	}
	total := parts
	if t.total > 0 {
		total = t.total
	}
	return Totals{TotalTokens: total, UncachedInputTokens: t.uncached, CachedInputTokens: t.cached,
		OutputTokens: t.output, Turns: t.turns, Threads: t.threads}, nil
}

type analyticsItem struct {
	clientID, model, name, id *string
	totals                    analyticsTotals
}

func (i analyticsItem) displayName() string {
	for _, s := range []*string{i.clientID, i.model, i.name, i.id} {
		if s == nil {
			continue
		}
		if t := strings.TrimFunc(*s, isSwiftWhitespace); t != "" {
			return t
		}
	}
	return "UNKNOWN"
}

func (d *jsonDec) analyticsItems() ([]analyticsItem, error) {
	var items []analyticsItem
	err := d.optElements(func() error {
		var it analyticsItem
		err := d.optObjectRequired(func(key string, first bool) error {
			if !first {
				return d.skip()
			}
			switch key {
			case "client_id":
				return d.lenientString(&it.clientID)
			case "model":
				return d.lenientString(&it.model)
			case "name":
				return d.lenientString(&it.name)
			case "id":
				return d.lenientString(&it.id)
			}
			if ok, err := it.totals.field(d, key); ok || err != nil {
				return err
			}
			return d.skip()
		})
		items = append(items, it)
		return err
	})
	return items, err
}

// optObjectRequired reads an object; null and other types are errors.
func (d *jsonDec) optObjectRequired(fn func(key string, first bool) error) error {
	d.skipSpace()
	return d.object(fn)
}

type analyticsDay struct {
	date    string
	totals  *analyticsTotals
	models  []analyticsItem
	clients []analyticsItem
}

func decodeAnalyticsDays(data []byte) ([]analyticsDay, error) {
	d := &jsonDec{b: data}
	if bytes.HasPrefix(d.b, []byte{0xEF, 0xBB, 0xBF}) {
		d.i = 3
	}
	var days []analyticsDay
	sawData := false
	err := d.object(func(key string, first bool) error {
		if key != "data" || !first {
			return d.skip()
		}
		sawData = true
		return d.elements(func() error {
			var day analyticsDay
			sawDate := false
			err := d.object(func(k string, first bool) error {
				if !first {
					return d.skip()
				}
				switch k {
				case "date":
					s, err := d.requiredString()
					day.date, sawDate = s, true
					return err
				case "totals":
					if d.peek() == '{' {
						day.totals = &analyticsTotals{}
					}
					return d.optObject(func(tk string, tfirst bool) error {
						if !tfirst {
							return d.skip()
						}
						if ok, err := day.totals.field(d, tk); ok || err != nil {
							return err
						}
						return d.skip()
					})
				case "models":
					var err error
					day.models, err = d.analyticsItems()
					return err
				case "clients":
					var err error
					day.clients, err = d.analyticsItems()
					return err
				}
				return d.skip()
			})
			if err != nil {
				return err
			}
			if !sawDate {
				return errMalformed
			}
			days = append(days, day)
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	d.skipSpace()
	if d.i != len(d.b) || !sawData {
		return nil, errMalformed
	}
	return days, nil
}

// DecodeAnalytics turns the online analytics response into a Summary of
// window. startDate and endDate are the inclusive "yyyy-MM-dd" strings used in
// the request; when empty they are derived from window in UTC. Online data is
// priced at the built-in equivalent rate and is only "priced" when every day
// reported its token classes.
func DecodeAnalytics(data []byte, window Window, calculatedAt time.Time, startDate, endDate string) (Summary, error) {
	days, err := decodeAnalyticsDays(data)
	if err != nil {
		if errors.Is(err, errMalformed) {
			err = ErrInvalidAnalytics
		}
		return Summary{}, err
	}
	if startDate == "" {
		startDate = apiDay(window.Start)
	}
	if endDate == "" {
		endDate = apiDay(window.End)
	}
	lower, upper := min(startDate, endDate), max(startDate, endDate)
	var items []analyticsDay
	for _, day := range days {
		if day.date >= lower && day.date <= upper {
			items = append(items, day)
		}
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].date < items[j].date })

	rows := make([]DailyRow, 0, len(items))
	for _, it := range items {
		t := Totals{}
		credits, reported := 0.0, false
		if it.totals != nil {
			if t, err = it.totals.checked(); err != nil {
				return Summary{}, err
			}
			credits, reported = it.totals.credits, it.totals.creditsReported
		}
		row := DailyRow{Date: it.date, Totals: t, RawCredits: credits,
			CreditsReported: boolPtr(reported), TotalsReported: boolPtr(it.totals != nil)}
		if t.HasTokenParts() {
			c := EquivalentCost(t)
			row.EstimatedUSD = &c
		}
		rows = append(rows, row)
	}
	all := make([]Totals, len(rows))
	credits := make([]float64, len(rows))
	estimated := decimal.Zero
	unpriced := false
	for i, r := range rows {
		all[i], credits[i] = r.Totals, r.RawCredits
		if r.EstimatedUSD != nil {
			estimated = estimated.Add(*r.EstimatedUSD)
		}
		if *r.TotalsReported != true || (r.Totals.TotalTokens > 0 && r.EstimatedUSD == nil) {
			unpriced = true
		}
	}
	totals, err := sumTotals(all)
	if err != nil {
		return Summary{}, err
	}
	hasCompleteCost := totals.HasTokenParts() && !unpriced
	rawCredits, err := checkedFiniteSum(credits, "analytics credits")
	if err != nil {
		return Summary{}, err
	}
	var modelItems, clientItems []analyticsItem
	for _, it := range items {
		modelItems = append(modelItems, it.models...)
		clientItems = append(clientItems, it.clients...)
	}
	modelRows, err := analyticsBreakdown(modelItems)
	if err != nil {
		return Summary{}, err
	}
	clientRows, err := analyticsBreakdown(clientItems)
	if err != nil {
		return Summary{}, err
	}
	s := Summary{
		Source:         SourceOnlineAnalytics,
		Window:         window,
		Totals:         totals,
		DailyRows:      rows,
		ModelRows:      modelRows,
		ProjectRows:    []BreakdownRow{},
		ClientRows:     clientRows,
		RawCredits:     rawCredits,
		Warnings:       []string{},
		PricingVersion: PricingVersion,
		CalculatedAt:   calculatedAt,
	}
	switch {
	case hasCompleteCost:
		s.Confidence = ConfidencePriced
		s.EstimatedUSD = &estimated
	case totals.TotalTokens > 0:
		s.Confidence = ConfidenceTokensOnly
	default:
		s.Confidence = ConfidenceUnavailable
	}
	if !hasCompleteCost {
		s.Warnings = []string{WarningAnalyticsTokenPartsMissing}
	}
	return s, nil
}

func boolPtr(b bool) *bool { return &b }

// apiDay is the UTC calendar day of t as "yyyy-MM-dd".
func apiDay(t time.Time) string { return dayKey(unixSeconds(t), time.UTC) }

// analyticsBreakdown groups items by display name. Rows without tokens, turns
// or credits are dropped; the rest are sorted by total tokens, largest first
// (ties by name, where the Swift version leaves the order unspecified).
func analyticsBreakdown(items []analyticsItem) ([]BreakdownRow, error) {
	type group struct {
		totals  []Totals
		credits []float64
	}
	groups := map[string]*group{}
	for _, it := range items {
		t, err := it.totals.checked()
		if err != nil {
			return nil, err
		}
		g := groups[it.displayName()]
		if g == nil {
			g = &group{}
			groups[it.displayName()] = g
		}
		g.totals = append(g.totals, t)
		g.credits = append(g.credits, it.totals.credits)
	}
	rows := []BreakdownRow{}
	for _, name := range sortedKeys(groups) {
		g := groups[name]
		t, err := sumTotals(g.totals)
		if err != nil {
			return nil, err
		}
		credits, err := checkedFiniteSum(g.credits, "analytics breakdown credits")
		if err != nil {
			return nil, err
		}
		if t.TotalTokens <= 0 && t.Turns <= 0 && credits <= 0 {
			continue
		}
		row := BreakdownRow{Name: name, Totals: t, RawCredits: credits}
		if t.HasTokenParts() {
			c := EquivalentCost(t)
			row.EstimatedUSD = &c
		}
		rows = append(rows, row)
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Totals.TotalTokens > rows[j].Totals.TotalTokens })
	return rows, nil
}
