package usage

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/l10n"
)

// day parses "YYYY-MM-DD" as noon UTC, or a full RFC 3339 time.
func day(s string) time.Time {
	if len(s) == 10 {
		s += "T12:00:00Z"
	}
	return mustTime(s)
}

func cells(h HeatmapSnapshot) []HeatmapCell {
	var out []HeatmapCell
	for _, w := range h.Weeks {
		out = append(out, w...)
	}
	return out
}

func inRange(h HeatmapSnapshot) []HeatmapCell {
	var out []HeatmapCell
	for _, c := range cells(h) {
		if c.InRange {
			out = append(out, c)
		}
	}
	return out
}

func cellFor(h HeatmapSnapshot, key string) *HeatmapCell {
	for _, c := range cells(h) {
		if c.DayKey == key {
			return &c
		}
	}
	return nil
}

func local(daily map[string]int64, mode HeatmapMode, now string, fw int) HeatmapInput {
	return HeatmapInput{Local: daily, Mode: mode, Now: day(now), FirstWeekday: fw, Loc: time.UTC}
}

// --- ApiCostRange -----------------------------------------------------------

func TestCostRangeCurrentAndPreviousCycle(t *testing.T) {
	w := Window{Start: day("2026-06-24T00:00:00Z"), End: day("2026-07-01T00:00:00Z")}
	r := RangeForWindow(w, time.UTC)
	if r.Window != w || r.APIStartDate != "2026-06-24" || r.APIEndDate != "2026-06-30" {
		t.Fatalf("current: %+v", r)
	}
	p := PreviousCycle(w, time.UTC)
	if !p.Window.Start.Equal(day("2026-06-17T00:00:00Z")) || !p.Window.End.Equal(w.Start) ||
		p.APIStartDate != "2026-06-17" || p.APIEndDate != "2026-06-23" {
		t.Fatalf("previous: %+v", p)
	}
}

func TestCostRangeLocalCalendar(t *testing.T) {
	sh := loadZone(t, "Asia/Shanghai")
	now := day("2026-06-30T06:03:12Z")
	m := ThisMonth(now, sh)
	if !m.Window.Start.Equal(day("2026-05-31T16:00:00Z")) || !m.Window.End.Equal(now) || m.APIStartDate != "2026-06-01" || m.APIEndDate != "2026-06-30" {
		t.Fatalf("month: %+v", m)
	}
	td := Today(now, sh)
	if !td.Window.Start.Equal(day("2026-06-29T16:00:00Z")) || td.APIStartDate != "2026-06-30" || td.APIEndDate != "2026-06-30" {
		t.Fatalf("today: %+v", td)
	}
	c, ok := CustomRange(day("2026-06-10T23:30:00Z"), day("2026-06-12T01:00:00Z"), sh)
	if !ok || !c.Window.Start.Equal(day("2026-06-10T16:00:00Z")) || !c.Window.End.Equal(day("2026-06-12T16:00:00Z")) ||
		c.APIStartDate != "2026-06-11" || c.APIEndDate != "2026-06-12" {
		t.Fatalf("custom: %+v ok=%v", c, ok)
	}
	if _, ok := CustomRange(day("2026-06-12T00:00:00Z"), day("2026-06-11T00:00:00Z"), time.UTC); ok {
		t.Fatal("start after end must be rejected")
	}
}

// --- profile ----------------------------------------------------------------

func TestProfileDailyUsageDecode(t *testing.T) {
	u, err := DecodeProfileTokenUsage([]byte(`{
	  "stats": {"daily_usage_buckets": [
	    {"start_date":"2026-07-25","tokens":488682728},
	    {"start_date":"2026-07-26","tokens":159644197}]},
	  "metadata": {"generated_at": "2026-07-27T08:03:40.648014Z", "stats_as_of": "2026-07-27", "stats_error": null}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if u.DailyTokens["2026-07-25"] != 488_682_728 || u.DailyTokens["2026-07-26"] != 159_644_197 || u.StatsAsOf != "2026-07-27" {
		t.Fatalf("%+v", u)
	}
	// The fraction is truncated to milliseconds like the system parser does.
	if want := time.Date(2026, 7, 27, 8, 3, 40, 648_000_000, time.UTC); !u.GeneratedAt.Equal(want) {
		t.Fatalf("generatedAt = %v", u.GeneratedAt)
	}
}

func TestProfileDailyUsageRejects(t *testing.T) {
	for name, body := range map[string]string{
		"negative":        `{"stats":{"daily_usage_buckets":[{"start_date":"2026-07-26","tokens":-1}]}}`,
		"invalid date":    `{"stats":{"daily_usage_buckets":[{"start_date":"2026-02-30","tokens":1}]}}`,
		"stats error":     `{"stats":{"daily_usage_buckets":[]},"metadata":{"stats_error":"failed"}}`,
		"bad metadata":    `{"stats":{"daily_usage_buckets":[]},"metadata":{"generated_at":"not-a-date","stats_as_of":"2026-02-30"}}`,
		"bad as-of":       `{"stats":{"daily_usage_buckets":[]},"metadata":{"stats_as_of":"2026-02-30"}}`,
		"duplicate":       `{"stats":{"daily_usage_buckets":[{"start_date":"2026-07-26","tokens":1},{"start_date":"2026-07-26","tokens":2}]}}`,
		"missing buckets": `{"stats":{}}`,
		"null buckets":    `{"stats":{"daily_usage_buckets":null}}`,
		"missing stats":   `{"metadata":{}}`,
		"missing tokens":  `{"stats":{"daily_usage_buckets":[{"start_date":"2026-07-26"}]}}`,
		"null tokens":     `{"stats":{"daily_usage_buckets":[{"start_date":"2026-07-26","tokens":null}]}}`,
		"string tokens":   `{"stats":{"daily_usage_buckets":[{"start_date":"2026-07-26","tokens":"5"}]}}`,
		"fraction":        `{"stats":{"daily_usage_buckets":[{"start_date":"2026-07-26","tokens":1.5}]}}`,
		"blank error ok?": `{"stats":{"daily_usage_buckets":[]},"metadata":{"stats_error":5}}`,
		"trailing":        `{"stats":{"daily_usage_buckets":[]}} x`,
	} {
		if _, err := DecodeProfileTokenUsage([]byte(body)); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
	// A blank stats error and blank dates are ignored.
	u, err := DecodeProfileTokenUsage([]byte(`{"stats":{"daily_usage_buckets":[]},"metadata":{"stats_error":"  ","stats_as_of":" ","generated_at":""}}`))
	if err != nil || u.StatsAsOf != "" || !u.GeneratedAt.IsZero() || len(u.DailyTokens) != 0 {
		t.Fatalf("blank metadata: %+v %v", u, err)
	}
}

func TestProfileDayKeyValidation(t *testing.T) {
	for k, want := range map[string]bool{
		"2026-02-28": true, "2024-02-29": true, "2026-02-29": false, "2026-13-01": false, "2026-00-10": false,
		"2026-04-31": false, "0100-02-29": true, "1900-02-29": false, "1582-10-10": false, "1582-10-04": true,
		"1582-10-15": true, "0000-01-01": false, "2026-1-01": false, "2026-01-1": false, "+202-01-01": true,
		"-123-01-01": false, "2026-01-+1": true, "2026-01-01 ": false, "20260101": false, "٢٠٢٦-01-01": false,
	} {
		if got := isValidProfileDay(k); got != want {
			t.Errorf("isValidProfileDay(%q) = %v, want %v", k, got, want)
		}
	}
}

// --- heatmap ----------------------------------------------------------------

func TestHeatmapEmptyDataBuildsGrid(t *testing.T) {
	h := BuildHeatmap(local(nil, HeatmapDaily, "2026-07-15", 1))
	if len(h.Weeks) == 0 || h.TotalTokens != 0 || h.HasUsage {
		t.Fatalf("%+v", h)
	}
	for _, w := range h.Weeks {
		if len(w) != 7 {
			t.Fatal("week must have 7 cells")
		}
	}
	for _, c := range inRange(h) {
		if c.Level != 0 {
			t.Fatal("empty data must be level 0")
		}
	}
}

func TestHeatmapDailyWeeklyCumulative(t *testing.T) {
	h := BuildHeatmap(local(map[string]int64{"2026-01-05": 100, "2026-01-06": 400}, HeatmapDaily, "2026-01-10", 1))
	j5, j6 := cellFor(h, "2026-01-05"), cellFor(h, "2026-01-06")
	if j5.Tokens != 100 || j6.Tokens != 400 || h.TotalTokens != 500 || !h.HasUsage || j6.Level < j5.Level {
		t.Fatalf("daily: %+v %+v", j5, j6)
	}
	w := BuildHeatmap(local(map[string]int64{"2026-01-04": 10, "2026-01-05": 30, "2026-01-06": 60}, HeatmapWeekly, "2026-01-10", 1))
	for _, k := range []string{"2026-01-04", "2026-01-05", "2026-01-06"} {
		if cellFor(w, k).Tokens != 100 {
			t.Fatalf("weekly %s = %d", k, cellFor(w, k).Tokens)
		}
	}
	c := BuildHeatmap(local(map[string]int64{"2026-01-01": 10, "2026-01-02": 20, "2026-01-03": 30}, HeatmapCumulative, "2026-01-05", 1))
	for k, want := range map[string]int64{"2026-01-01": 10, "2026-01-02": 30, "2026-01-03": 60, "2026-01-04": 60, "2026-01-05": 60} {
		if got := cellFor(c, k).Tokens; got != want {
			t.Errorf("cumulative %s = %d, want %d", k, got, want)
		}
	}
}

func TestHeatmapZeroDaysFutureAndLevels(t *testing.T) {
	h := BuildHeatmap(local(map[string]int64{"2026-02-15": 1000}, HeatmapDaily, "2026-03-01", 2))
	for _, c := range inRange(h) {
		if c.DayKey != "2026-02-15" && (c.Level != 0 || c.Tokens != 0) {
			t.Fatalf("zero day %+v", c)
		}
	}
	if l := cellFor(h, "2026-02-15").Level; l != 1 {
		t.Fatalf("single positive sample level = %d", l)
	}
	f := BuildHeatmap(local(map[string]int64{"2026-06-20": 999}, HeatmapDaily, "2026-06-15", 1))
	if c := cellFor(f, "2026-06-20"); c.InRange || f.TotalTokens != 0 {
		t.Fatalf("future: %+v total %d", c, f.TotalTokens)
	}
	// Quartile thresholds of the positive samples 1..4 are 2, 3 and 3.
	q := BuildHeatmap(local(map[string]int64{"2026-01-01": 1, "2026-01-02": 2, "2026-01-03": 3, "2026-01-04": 4}, HeatmapDaily, "2026-01-05", 1))
	got := []int{cellFor(q, "2026-01-01").Level, cellFor(q, "2026-01-02").Level, cellFor(q, "2026-01-03").Level, cellFor(q, "2026-01-04").Level}
	if fmt.Sprint(got) != "[1 1 2 4]" {
		t.Fatalf("levels = %v", got)
	}
}

func TestHeatmapUTCAlignment(t *testing.T) {
	now := day("2026-08-30T01:00:00+08:00")
	if w := UTCYearToDateWindow(now); !w.Start.Equal(day("2026-01-01T00:00:00Z")) || !w.End.Equal(day("2026-08-30T00:00:00Z")) {
		t.Fatalf("window %+v", w)
	}
	h := BuildHeatmap(HeatmapInput{
		AllDevices: map[string]int64{"2026-08-29": 100, "2026-08-30": 200}, Local: map[string]int64{"2026-08-29": 10, "2026-08-30": 20},
		Mode: HeatmapDaily, Now: now, FirstWeekday: 1, Loc: time.UTC,
	})
	c := cellFor(h, "2026-08-29")
	if !c.InRange || c.AllDevicesTokens != 100 || c.LocalTokens != 10 {
		t.Fatalf("%+v", c)
	}
	for _, c := range cells(h) {
		if c.DayKey == "2026-08-30" && c.InRange {
			t.Fatal("tomorrow in UTC must be out of range")
		}
	}
	if h.TotalAllDevices != 100 || h.TotalLocal != 10 {
		t.Fatalf("totals %d %d", h.TotalAllDevices, h.TotalLocal)
	}
}

func TestYearToDateWindowDisplayCalendar(t *testing.T) {
	w := YearToDateWindow(day("2026-07-27T12:00:00Z"), time.UTC)
	if !w.Start.Equal(day("2026-01-01T00:00:00Z")) || !w.End.Equal(day("2026-07-28T00:00:00Z")) {
		t.Fatalf("%+v", w)
	}
}

func TestHeatmapDualSeries(t *testing.T) {
	h := BuildHeatmap(HeatmapInput{
		AllDevices: map[string]int64{"2026-01-05": 1_600_000_000, "2026-01-06": 100},
		Local:      map[string]int64{"2026-01-05": 1_000_000_000, "2026-01-06": 50},
		Mode:       HeatmapDaily, Now: day("2026-01-10"), FirstWeekday: 1, Loc: time.UTC,
	})
	c := cellFor(h, "2026-01-05")
	if c.AllDevicesTokens != 1_600_000_000 || c.LocalTokens != 1_000_000_000 || c.Tokens != 1_600_000_000 ||
		!h.HasAllDevicesData || !h.HasLocalData || h.TotalAllDevices != 1_600_000_100 || h.TotalLocal != 1_000_000_050 {
		t.Fatalf("%+v %+v", c, h)
	}
	// Local above official is never clamped.
	h = BuildHeatmap(HeatmapInput{AllDevices: map[string]int64{"2026-01-05": 998_000_000}, Local: map[string]int64{"2026-01-05": 1_611_000_000},
		Mode: HeatmapDaily, Now: day("2026-01-10"), FirstWeekday: 1, Loc: time.UTC})
	c = cellFor(h, "2026-01-05")
	if c.AllDevicesTokens != 998_000_000 || c.LocalTokens != 1_611_000_000 || c.Tokens != 998_000_000 {
		t.Fatalf("%+v", c)
	}
	// Weekly colouring uses week totals while tooltip fields stay daily.
	h = BuildHeatmap(HeatmapInput{AllDevices: map[string]int64{"2026-01-04": 10, "2026-01-05": 30, "2026-01-06": 60},
		Local: map[string]int64{"2026-01-05": 7}, Mode: HeatmapWeekly, Now: day("2026-01-10"), FirstWeekday: 1, Loc: time.UTC})
	c = cellFor(h, "2026-01-05")
	if c.Tokens != 100 || c.AllDevicesTokens != 30 || c.LocalTokens != 7 {
		t.Fatalf("%+v", c)
	}
}

func TestHeatmapMonthLabels(t *testing.T) {
	h := BuildHeatmap(local(nil, HeatmapDaily, "2026-03-20", 1))
	var months []int
	for _, l := range h.MonthLabels {
		months = append(months, l.Month)
	}
	if fmt.Sprint(months) != "[1 2 3]" {
		t.Fatalf("months = %v", months)
	}
}

func TestChartSeries(t *testing.T) {
	daily := map[string]int64{}
	for d := 1; d <= 20; d++ {
		daily[fmt.Sprintf("2026-01-%02d", d)] = 10
		daily[fmt.Sprintf("2026-02-%02d", d)] = 20
		daily[fmt.Sprintf("2026-03-%02d", d)] = 30
	}
	s := BuildChartSeries(local(daily, HeatmapDaily, "2026-03-20", 1))
	if len(s.Points) != ChartDailyWindowDays || s.Points[0].DayKey != "2026-02-19" || s.Points[len(s.Points)-1].DayKey != "2026-03-20" || !s.HasUsage {
		t.Fatalf("daily: %d points first %s", len(s.Points), s.Points[0].DayKey)
	}
	s = BuildChartSeries(local(map[string]int64{"2026-01-05": 100, "2026-01-06": 400}, HeatmapDaily, "2026-01-10", 1))
	if len(s.Points) != 10 || s.TotalTokens != 500 {
		t.Fatalf("early year: %d points total %d", len(s.Points), s.TotalTokens)
	}
	s = BuildChartSeries(local(map[string]int64{"2026-01-04": 10, "2026-01-05": 30, "2026-01-06": 60}, HeatmapWeekly, "2026-01-10", 1))
	if len(s.Points) != 2 {
		t.Fatalf("weekly points = %d", len(s.Points))
	}
	var week *ChartPoint
	for i := range s.Points {
		if s.Points[i].Tokens == 100 {
			week = &s.Points[i]
		}
	}
	if week == nil || week.AllDevicesTokens != 0 || week.LocalTokens != 100 {
		t.Fatalf("week point %+v", week)
	}
	s = BuildChartSeries(local(map[string]int64{"2026-01-01": 10, "2026-01-20": 20, "2026-02-05": 40, "2026-03-10": 100}, HeatmapCumulative, "2026-03-15", 1))
	if len(s.Points) != 3 || s.Points[0].Tokens != 30 || s.Points[1].Tokens != 40 || s.Points[2].Tokens != 100 {
		t.Fatalf("monthly: %+v", s.Points)
	}
	s = BuildChartSeries(HeatmapInput{AllDevices: map[string]int64{"2026-01-05": 1600, "2026-01-06": 100}, Local: map[string]int64{"2026-01-05": 50, "2026-01-06": 20},
		Mode: HeatmapDaily, Now: day("2026-01-10"), FirstWeekday: 1, Loc: time.UTC})
	var jan5 ChartPoint
	for _, p := range s.Points {
		if p.DayKey == "2026-01-05" {
			jan5 = p
		}
	}
	if jan5.Tokens != 1600 || jan5.AllDevicesTokens != 1600 || jan5.LocalTokens != 50 || !s.HasAllDevicesData || s.TotalTokens != 1700 {
		t.Fatalf("%+v total %d", jan5, s.TotalTokens)
	}
}

func TestHeatmapDSTGapMidnight(t *testing.T) {
	// Sao Paulo skipped midnight on 2018-11-04: the day starts at 01:00 and
	// every cell keeps a distinct day key.
	sp := loadZone(t, "America/Sao_Paulo")
	h := BuildHeatmap(HeatmapInput{Local: map[string]int64{"2018-11-04": 7}, Mode: HeatmapDaily, Now: time.Date(2018, 11, 10, 12, 0, 0, 0, sp), FirstWeekday: 1, Loc: sp})
	seen := map[string]bool{}
	for _, c := range cells(h) {
		if seen[c.DayKey] {
			t.Fatalf("duplicate day key %s", c.DayKey)
		}
		seen[c.DayKey] = true
	}
	if c := cellFor(h, "2018-11-04"); c == nil || !c.InRange || c.Tokens != 7 {
		t.Fatalf("%+v", c)
	}
}

func TestHeatmapFromRows(t *testing.T) {
	rows := []DailyRow{{Date: "2026-01-05", Totals: Totals{TotalTokens: 5}}, {Date: "2026-01-06", Totals: Totals{TotalTokens: -3}}}
	h := HeatmapFromRows(rows, HeatmapDaily, day("2026-01-10"), 1, time.UTC)
	if h.TotalLocal != 5 || cellFor(h, "2026-01-06").Tokens != 0 {
		t.Fatalf("%+v", h.TotalLocal)
	}
}

func TestCompactTokenCount(t *testing.T) {
	for _, c := range []struct {
		v    int64
		lang l10n.Language
		want string
	}{
		{999, l10n.SimplifiedChinese, "999"}, {3_000_100, l10n.SimplifiedChinese, "300.01万"},
		{982_000_000, l10n.SimplifiedChinese, "9.82亿"}, {10_000, l10n.SimplifiedChinese, "1万"},
		{1_500, l10n.English, "1.5K"}, {100_340_000, l10n.English, "100.34M"}, {2_000_000_000, l10n.English, "2B"},
		{10_000, l10n.TraditionalChinese, "1萬"}, {100_000_000, l10n.Japanese, "1億"}, {10_000, l10n.Korean, "1만"},
		{1_500, l10n.French, "1.5K"}, {1_500, l10n.Russian, "1.5K"}, {-5, l10n.English, "0"},
		{100_000_000, l10n.TraditionalChinese, "1億"}, {100_000_000, l10n.Korean, "1억"}, {100_000_000, l10n.SimplifiedChinese, "1亿"},
	} {
		if got := CompactTokenCount(c.v, c.lang); got != c.want {
			t.Errorf("CompactTokenCount(%d, %s) = %q, want %q", c.v, c.lang, got, c.want)
		}
	}
	if strings.Contains(CompactTokenCount(10_000, l10n.Japanese), "K") {
		t.Fatal("Japanese must not use Latin units")
	}
}
