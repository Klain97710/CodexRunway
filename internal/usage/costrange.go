package usage

import "time"

// CostRange is a time window together with the inclusive "yyyy-MM-dd" days
// the online analytics API is asked for.
type CostRange struct {
	Window       Window
	APIStartDate string
	APIEndDate   string
}

// RangeForWindow builds the range of an existing half-open window. When the
// window ends exactly on a day boundary, the last included day is the one
// before it.
func RangeForWindow(w Window, loc *time.Location) CostRange {
	return makeCostRange(w, newDayCalendar(loc, 1))
}

// PreviousCycle is the window of the same length ending where w starts.
func PreviousCycle(w Window, loc *time.Location) CostRange {
	d := w.End.Sub(w.Start)
	return makeCostRange(Window{Start: w.Start.Add(-d), End: w.Start}, newDayCalendar(loc, 1))
}

// ThisMonth runs from local midnight of the month's first day to now.
func ThisMonth(now time.Time, loc *time.Location) CostRange {
	c := newDayCalendar(loc, 1)
	y, m, _ := c.ymd(now)
	return makeCostRange(Window{Start: c.dateFrom(y, m, 1), End: now}, c)
}

// Today runs from local midnight to now.
func Today(now time.Time, loc *time.Location) CostRange {
	c := newDayCalendar(loc, 1)
	return makeCostRange(Window{Start: c.startOfDay(now), End: now}, c)
}

// CustomRange covers the whole local days of start through end. ok is false
// when start's day is after end's day.
func CustomRange(start, end time.Time, loc *time.Location) (CostRange, bool) {
	c := newDayCalendar(loc, 1)
	startDay, endDay := c.startOfDay(start), c.startOfDay(end)
	if startDay.After(endDay) {
		return CostRange{}, false
	}
	return CostRange{
		Window:       Window{Start: startDay, End: c.addDays(endDay, 1)},
		APIStartDate: c.dayString(startDay),
		APIEndDate:   c.dayString(endDay),
	}, true
}

func makeCostRange(w Window, c dayCalendar) CostRange {
	end := w.Start
	if w.End.After(w.Start) {
		end = w.End
		if c.startOfDay(end).Equal(end) {
			end = end.Add(-time.Second)
		}
	}
	return CostRange{Window: w, APIStartDate: c.dayString(w.Start), APIEndDate: c.dayString(end)}
}
