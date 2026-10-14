package usage

import (
	"time"
)

// dayCalendar reproduces the parts of Foundation's Gregorian Calendar the
// cost views use, fixed to one time zone. Its arithmetic follows Foundation's
// rules rather than Go's: adding days keeps the wall-clock time of the start
// instant (so a day whose midnight does not exist starts at the first valid
// instant and later dates reached from it keep that wall time), a wall time
// that does not exist moves forward by the length of the gap, and a wall time
// that occurs twice resolves to its first occurrence.
type dayCalendar struct {
	loc          *time.Location
	firstWeekday int // 1 = Sunday ... 7 = Saturday
}

func newDayCalendar(loc *time.Location, firstWeekday int) dayCalendar {
	if loc == nil {
		loc = time.UTC
	}
	return dayCalendar{loc: loc, firstWeekday: min(7, max(1, firstWeekday))}
}

// resolve turns local wall-clock fields (which may be out of range and are
// normalised like DateComponents rolls over) into an instant.
func (c dayCalendar) resolve(y, mo, d, h, mi, s, ns int) time.Time {
	wall := time.Date(y, time.Month(mo), d, h, mi, s, ns, time.UTC)
	l := wall.Unix()
	_, before := time.Unix(l-86400, 0).In(c.loc).Zone()
	_, after := time.Unix(l+86400, 0).In(c.loc).Zone()
	var best int64
	found := false
	for _, off := range [2]int{before, after} {
		u := l - int64(off)
		if _, got := time.Unix(u, 0).In(c.loc).Zone(); got != off {
			continue
		}
		if !found || u < best {
			best, found = u, true
		}
	}
	if !found {
		// The wall time falls into a gap: use the offset in force before it,
		// which lands the same wall time after the gap.
		best = l - int64(before)
	}
	return time.Unix(best, int64(wall.Nanosecond())).In(c.loc)
}

// startOfDay is the first instant of t's local day.
func (c dayCalendar) startOfDay(t time.Time) time.Time {
	y, m, d := t.In(c.loc).Date()
	return c.resolve(y, int(m), d, 0, 0, 0, 0)
}

// dateFrom is local midnight of the given day (rolling out-of-range fields).
func (c dayCalendar) dateFrom(y, m, d int) time.Time { return c.resolve(y, m, d, 0, 0, 0, 0) }

// addDays adds n calendar days keeping t's wall-clock time.
//
// When the zone's offset changes by a whole day or more in between (a day
// removed from the calendar, as Samoa did in 2011), Foundation adds 24-hour
// days and corrects only the remainder of the offset change.
func (c dayCalendar) addDays(t time.Time, n int) time.Time {
	naive := t.Add(time.Duration(n) * 24 * time.Hour)
	_, prev := t.In(c.loc).Zone()
	_, next := naive.In(c.loc).Zone()
	if delta := prev - next; delta >= 86400 || delta <= -86400 {
		return naive.Add(time.Duration(delta%86400) * time.Second).In(c.loc)
	}
	l := t.In(c.loc)
	y, m, d := l.Date()
	h, mi, s := l.Clock()
	return c.resolve(y, int(m), d+n, h, mi, s, l.Nanosecond())
}

// addMonths adds n calendar months keeping t's wall-clock time; a day of
// month past the end of the target month clamps to its last day.
func (c dayCalendar) addMonths(t time.Time, n int) time.Time {
	l := t.In(c.loc)
	y, m, d := l.Date()
	h, mi, s := l.Clock()
	target := time.Date(y, m+time.Month(n), 1, 0, 0, 0, 0, time.UTC)
	last := time.Date(target.Year(), target.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
	return c.resolve(target.Year(), int(target.Month()), min(d, last), h, mi, s, l.Nanosecond())
}

func (c dayCalendar) ymd(t time.Time) (int, int, int) {
	y, m, d := t.In(c.loc).Date()
	return y, int(m), d
}

// weekday is 1 for Sunday through 7 for Saturday.
func (c dayCalendar) weekday(t time.Time) int { return int(t.In(c.loc).Weekday()) + 1 }

func (c dayCalendar) startOfWeek(t time.Time) time.Time {
	day := c.startOfDay(t)
	delta := (c.weekday(day) - c.firstWeekday + 7) % 7
	return c.addDays(day, -delta)
}

func (c dayCalendar) endOfWeek(t time.Time) time.Time { return c.addDays(c.startOfWeek(t), 6) }

// dayString is the "yyyy-MM-dd" label of t's local day.
func (c dayCalendar) dayString(t time.Time) string {
	y, m, d := c.ymd(t)
	return formatDay(y, m, d)
}
