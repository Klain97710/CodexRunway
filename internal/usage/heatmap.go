package usage

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/licoy/quosparr/internal/l10n"
)

// HeatmapMode selects the metric that drives intensity and chart height.
type HeatmapMode string

// Heatmap modes. Cumulative doubles as the per-month series of the line chart.
const (
	HeatmapDaily      HeatmapMode = "daily"
	HeatmapWeekly     HeatmapMode = "weekly"
	HeatmapCumulative HeatmapMode = "cumulative"
)

// ChartDailyWindowDays is the number of daily points shown on the charts.
const ChartDailyWindowDays = 30

// ChartPoint is one point of the line or bar chart.
type ChartPoint struct {
	DayKey string
	Date   time.Time
	// Tokens is the mode metric: the primary series' day, week or month total.
	Tokens int64
	// AllDevicesTokens and LocalTokens are the same period's totals of the
	// two sources, for tooltips.
	AllDevicesTokens int64
	LocalTokens      int64
}

// ChartSeries is the chart data of one mode.
type ChartSeries struct {
	Mode              HeatmapMode
	Points            []ChartPoint
	TotalTokens       int64
	TotalAllDevices   int64
	TotalLocal        int64
	HasAllDevicesData bool
	HasLocalData      bool
	CalculatedAt      time.Time
	HasUsage          bool
}

// HeatmapCell is one day of the contribution grid.
type HeatmapCell struct {
	DayKey string
	Date   time.Time
	// Tokens is the mode metric driving the intensity.
	Tokens int64
	// AllDevicesTokens and LocalTokens are always that day's own totals.
	AllDevicesTokens int64
	LocalTokens      int64
	// Level is 0 for no usage, 1 to 4 by quartile of the positive values.
	Level   int
	InRange bool
}

// HeatmapMonthLabel marks the first week column that contains a day of Month.
type HeatmapMonthLabel struct {
	WeekIndex int
	Month     int
}

// HeatmapSnapshot is a year-to-date grid: columns are weeks of seven cells
// starting at the calendar's first weekday.
type HeatmapSnapshot struct {
	Mode              HeatmapMode
	Weeks             [][]HeatmapCell
	MonthLabels       []HeatmapMonthLabel
	TotalTokens       int64
	TotalAllDevices   int64
	TotalLocal        int64
	HasAllDevicesData bool
	HasLocalData      bool
	CalculatedAt      time.Time
	HasUsage          bool
}

// HeatmapInput is what the builders need. Day maps are keyed by "yyyy-MM-dd"
// in Loc's calendar; negative values count as zero.
type HeatmapInput struct {
	AllDevices map[string]int64
	Local      map[string]int64
	Mode       HeatmapMode
	Now        time.Time
	// FirstWeekday is 1 (Sunday) to 7 (Saturday); out-of-range values clamp.
	FirstWeekday int
	Loc          *time.Location
}

// BuildHeatmap builds the year-to-date grid. Intensity follows the all-devices
// series when it has any usage, otherwise the local one.
func BuildHeatmap(in HeatmapInput) HeatmapSnapshot {
	cal := newDayCalendar(in.Loc, in.FirstWeekday)
	today := cal.startOfDay(in.Now)
	year, _, _ := cal.ymd(today)
	yearStart := cal.dateFrom(year, 1, 1)

	gridStart := cal.startOfWeek(yearStart)
	gridEnd := cal.endOfWeek(today)
	dayKeys := orderedDayKeys(cal, yearStart, today)
	allDaily := clippedDaily(dayKeys, in.AllDevices)
	localDaily := clippedDaily(dayKeys, in.Local)
	totalAll, totalLocal := sumValues(allDaily), sumValues(localDaily)
	hasAll, hasLocal := totalAll > 0, totalLocal > 0

	allMetric := metricSeries(in.Mode, dayKeys, allDaily, yearStart, today, cal)
	localMetric := metricSeries(in.Mode, dayKeys, localDaily, yearStart, today, cal)
	primary := localMetric
	total := totalLocal
	if hasAll {
		primary, total = allMetric, totalAll
	}
	levels := intensityLevels(primary)

	snap := HeatmapSnapshot{
		Mode: in.Mode, TotalTokens: total, TotalAllDevices: totalAll, TotalLocal: totalLocal,
		HasAllDevicesData: hasAll, HasLocalData: hasLocal, CalculatedAt: in.Now,
		HasUsage: totalAll > 0 || totalLocal > 0,
	}
	lastLabeled := 0
	cursor := gridStart
	for weekIndex := 0; !cursor.After(gridEnd); weekIndex++ {
		column := make([]HeatmapCell, 0, 7)
		columnMonth := 0
		for offset := 0; offset < 7; offset++ {
			date := cal.addDays(cursor, offset)
			key := cal.dayString(date)
			inRange := !date.Before(yearStart) && !date.After(today)
			cell := HeatmapCell{DayKey: key, Date: date, InRange: inRange}
			if inRange {
				cell.AllDevicesTokens, cell.LocalTokens = allDaily[key], localDaily[key]
				cell.Tokens, cell.Level = primary[key], levels[key]
				if columnMonth == 0 {
					_, columnMonth, _ = cal.ymd(date)
				}
			}
			column = append(column, cell)
		}
		snap.Weeks = append(snap.Weeks, column)
		if columnMonth != 0 && columnMonth != lastLabeled {
			snap.MonthLabels = append(snap.MonthLabels, HeatmapMonthLabel{WeekIndex: weekIndex, Month: columnMonth})
			lastLabeled = columnMonth
		}
		cursor = cal.addDays(cursor, 7)
	}
	return snap
}

// BuildChartSeries builds the chart points of in.Mode: the last
// ChartDailyWindowDays days (daily), one point per week (weekly) or one per
// month (cumulative), all within the current year.
func BuildChartSeries(in HeatmapInput) ChartSeries {
	cal := newDayCalendar(in.Loc, in.FirstWeekday)
	today := cal.startOfDay(in.Now)
	year, _, _ := cal.ymd(today)
	yearStart := cal.dateFrom(year, 1, 1)

	yearKeys := orderedDayKeys(cal, yearStart, today)
	allDaily := clippedDaily(yearKeys, in.AllDevices)
	localDaily := clippedDaily(yearKeys, in.Local)
	totalAll, totalLocal := sumValues(allDaily), sumValues(localDaily)
	hasAll, hasLocal := totalAll > 0, totalLocal > 0
	primaryDaily, total := localDaily, totalLocal
	if hasAll {
		primaryDaily, total = allDaily, totalAll
	}

	var points []ChartPoint
	switch in.Mode {
	case HeatmapDaily:
		windowStart := cal.addDays(today, -(ChartDailyWindowDays - 1))
		seriesStart := cal.startOfDay(windowStart)
		if yearStart.After(seriesStart) {
			seriesStart = yearStart
		}
		for _, key := range orderedDayKeys(cal, seriesStart, today) {
			date, ok := dateFromDayKey(cal, key)
			if !ok {
				continue
			}
			points = append(points, ChartPoint{DayKey: key, Date: date, Tokens: primaryDaily[key],
				AllDevicesTokens: allDaily[key], LocalTokens: localDaily[key]})
		}
	case HeatmapWeekly:
		points = weeklyChartPoints(cal, yearStart, today, allDaily, localDaily, primaryDaily)
	case HeatmapCumulative:
		points = monthlyChartPoints(cal, yearStart, today, allDaily, localDaily, primaryDaily)
	}
	return ChartSeries{Mode: in.Mode, Points: points, TotalTokens: total, TotalAllDevices: totalAll,
		TotalLocal: totalLocal, HasAllDevicesData: hasAll, HasLocalData: hasLocal,
		CalculatedAt: in.Now, HasUsage: totalAll > 0 || totalLocal > 0}
}

// HeatmapFromRows builds a local-only grid from the daily rows of a Summary.
func HeatmapFromRows(rows []DailyRow, mode HeatmapMode, now time.Time, firstWeekday int, loc *time.Location) HeatmapSnapshot {
	daily := make(map[string]int64, len(rows))
	for _, r := range rows {
		daily[r.Date] = max(0, r.Totals.TotalTokens)
	}
	return BuildHeatmap(HeatmapInput{Local: daily, Mode: mode, Now: now, FirstWeekday: firstWeekday, Loc: loc})
}

// YearToDateWindow is local January 1 through the end of today's local day.
func YearToDateWindow(now time.Time, loc *time.Location) Window {
	cal := newDayCalendar(loc, 1)
	today := cal.startOfDay(now)
	year, _, _ := cal.ymd(today)
	return Window{Start: cal.dateFrom(year, 1, 1), End: cal.addDays(today, 1)}
}

// UTCYearToDateWindow is YearToDateWindow on UTC days, which the official
// profile statistics use.
func UTCYearToDateWindow(now time.Time) Window { return YearToDateWindow(now, time.UTC) }

// CompactTokenCount abbreviates a token count: 万/亿 (or 萬/億, 만/억) for
// CJK languages and K/M/B otherwise, with up to two decimals.
func CompactTokenCount(value int64, lang l10n.Language) string {
	amount := max(0, value)
	switch lang.CompactNumberScale() {
	case l10n.ScaleWanYiSimplified:
		return compactAsian(amount, "万", "亿")
	case l10n.ScaleWanYiJapanese:
		return compactAsian(amount, "万", "億")
	case l10n.ScaleWanYiTraditional:
		return compactAsian(amount, "萬", "億")
	case l10n.ScaleManEok:
		return compactAsian(amount, "만", "억")
	}
	return compactLatin(amount)
}

func compactAsian(v int64, tenThousand, hundredMillion string) string {
	switch {
	case v >= 100_000_000:
		return trimTrailingZeros(fmt.Sprintf("%.2f", float64(v)/100_000_000)) + hundredMillion
	case v >= 10_000:
		return trimTrailingZeros(fmt.Sprintf("%.2f", float64(v)/10_000)) + tenThousand
	}
	return strconv.FormatInt(v, 10)
}

func compactLatin(v int64) string {
	switch {
	case v >= 1_000_000_000:
		return trimTrailingZeros(fmt.Sprintf("%.2f", float64(v)/1_000_000_000)) + "B"
	case v >= 1_000_000:
		return trimTrailingZeros(fmt.Sprintf("%.2f", float64(v)/1_000_000)) + "M"
	case v >= 1_000:
		return trimTrailingZeros(fmt.Sprintf("%.2f", float64(v)/1_000)) + "K"
	}
	return strconv.FormatInt(v, 10)
}

func trimTrailingZeros(s string) string {
	if !strings.Contains(s, ".") {
		return s
	}
	s = strings.TrimRight(s, "0")
	return strings.TrimSuffix(s, ".")
}

// --- internals --------------------------------------------------------------

func sumValues(m map[string]int64) int64 {
	var t int64
	for _, v := range m {
		t += v
	}
	return t
}

func clippedDaily(dayKeys []string, source map[string]int64) map[string]int64 {
	out := make(map[string]int64, len(dayKeys))
	for _, k := range dayKeys {
		out[k] = max(0, source[k])
	}
	return out
}

func orderedDayKeys(cal dayCalendar, start, end time.Time) []string {
	var keys []string
	cursor, last := cal.startOfDay(start), cal.startOfDay(end)
	for !cursor.After(last) {
		keys = append(keys, cal.dayString(cursor))
		cursor = cal.addDays(cursor, 1)
	}
	return keys
}

func dateFromDayKey(cal dayCalendar, key string) (time.Time, bool) {
	parts := strings.Split(key, "-")
	if len(parts) != 3 {
		return time.Time{}, false
	}
	var n [3]int
	for i, p := range parts {
		v, err := strconv.Atoi(p)
		if err != nil {
			return time.Time{}, false
		}
		n[i] = v
	}
	return cal.dateFrom(n[0], n[1], n[2]), true
}

func metricSeries(mode HeatmapMode, dayKeys []string, daily map[string]int64, yearStart, today time.Time, cal dayCalendar) map[string]int64 {
	switch mode {
	case HeatmapWeekly:
		return weeklyTotalsByDayKey(cal, yearStart, today, daily)
	case HeatmapCumulative:
		var running int64
		out := make(map[string]int64, len(dayKeys))
		for _, k := range dayKeys {
			running += daily[k]
			out[k] = running
		}
		return out
	}
	return daily
}

func weeklyTotalsByDayKey(cal dayCalendar, start, end time.Time, daily map[string]int64) map[string]int64 {
	out := make(map[string]int64, len(daily))
	cursor, last := cal.startOfDay(start), cal.startOfDay(end)
	for !cursor.After(last) {
		weekStart := cal.startOfWeek(cursor)
		var total int64
		var keys []string
		day := weekStart
		for i := 0; i < 7; i++ {
			if !day.Before(start) && !day.After(last) {
				k := cal.dayString(day)
				keys = append(keys, k)
				total += daily[k]
			}
			day = cal.addDays(day, 1)
		}
		for _, k := range keys {
			out[k] = total
		}
		cursor = cal.addDays(weekStart, 7)
	}
	return out
}

func weeklyChartPoints(cal dayCalendar, start, end time.Time, allDaily, localDaily, primaryDaily map[string]int64) []ChartPoint {
	var points []ChartPoint
	cursor, last := cal.startOfDay(start), cal.startOfDay(end)
	for !cursor.After(last) {
		weekStart := cal.startOfWeek(cursor)
		var primary, all, local int64
		lastInRange := weekStart
		day := weekStart
		for i := 0; i < 7; i++ {
			if !day.Before(start) && !day.After(last) {
				k := cal.dayString(day)
				primary += primaryDaily[k]
				all += allDaily[k]
				local += localDaily[k]
				lastInRange = day
			}
			day = cal.addDays(day, 1)
		}
		points = append(points, ChartPoint{DayKey: cal.dayString(lastInRange), Date: lastInRange,
			Tokens: primary, AllDevicesTokens: all, LocalTokens: local})
		cursor = cal.addDays(weekStart, 7)
	}
	return points
}

func monthlyChartPoints(cal dayCalendar, start, end time.Time, allDaily, localDaily, primaryDaily map[string]int64) []ChartPoint {
	var points []ChartPoint
	last := cal.startOfDay(end)
	sy, sm, _ := cal.ymd(start)
	cursor := cal.dateFrom(sy, sm, 1)
	for !cursor.After(last) {
		y, m, _ := cal.ymd(cursor)
		monthStart := cal.dateFrom(y, m, 1)
		nextMonth := cal.addMonths(monthStart, 1)
		rangeEnd := cal.addDays(nextMonth, -1)
		if last.Before(rangeEnd) {
			rangeEnd = last
		}
		var primary, all, local int64
		for day := monthStart; !day.After(rangeEnd); day = cal.addDays(day, 1) {
			if !day.Before(start) {
				k := cal.dayString(day)
				primary += primaryDaily[k]
				all += allDaily[k]
				local += localDaily[k]
			}
		}
		points = append(points, ChartPoint{DayKey: cal.dayString(rangeEnd), Date: rangeEnd,
			Tokens: primary, AllDevicesTokens: all, LocalTokens: local})
		cursor = nextMonth
	}
	return points
}

// intensityLevels maps positive values to 1...4 by the quartiles of the
// positive samples; zeros stay 0.
func intensityLevels(metrics map[string]int64) map[string]int {
	var positives []int64
	for _, v := range metrics {
		if v > 0 {
			positives = append(positives, v)
		}
	}
	out := make(map[string]int, len(metrics))
	if len(positives) == 0 {
		for k := range metrics {
			out[k] = 0
		}
		return out
	}
	sort.Slice(positives, func(i, j int) bool { return positives[i] < positives[j] })
	q1, q2, q3 := percentile(positives, 0.25), percentile(positives, 0.50), percentile(positives, 0.75)
	for k, v := range metrics {
		switch {
		case v <= 0:
			out[k] = 0
		case v <= q1:
			out[k] = 1
		case v <= q2:
			out[k] = 2
		case v <= q3:
			out[k] = 3
		default:
			out[k] = 4
		}
	}
	return out
}

func percentile(sorted []int64, p float64) int64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	rank := min(len(sorted)-1, max(0, int(math.Round(float64(len(sorted)-1)*p))))
	return sorted[rank]
}
