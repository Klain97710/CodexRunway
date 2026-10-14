package resettoday

import (
	"sync"
	"time"
	_ "time/tzdata" // America/Los_Angeles must resolve on machines without a zoneinfo database
	"unicode"
)

func isNumber(r rune) bool { return unicode.IsNumber(r) }

func locOrLocal(loc *time.Location) *time.Location {
	if loc == nil {
		return time.Local
	}
	return loc
}

var (
	sourceOnce sync.Once
	sourceLoc  *time.Location
)

// sourceLocation is the calendar the feed's date-precision schedules use.
func sourceLocation() *time.Location {
	sourceOnce.Do(func() {
		l, err := time.LoadLocation("America/Los_Angeles")
		if err != nil {
			l = time.FixedZone("PST", -8*3600) // unreachable with the embedded database
		}
		sourceLoc = l
	})
	return sourceLoc
}

func dayKey(t time.Time) int {
	y, m, d := t.Date()
	return y*10000 + int(m)*100 + d
}

// startOfDayYMD is the first instant of the given calendar day (month and day
// may overflow and are normalized). A midnight that does not exist resolves to
// the first valid instant of the day, an ambiguous one to the earlier.
func startOfDayYMD(y int, m time.Month, d int, loc *time.Location) time.Time {
	cand := time.Date(y, m, d, 0, 0, 0, 0, loc)
	// Noon exists in every zone, so it names the target day even when
	// midnight is skipped and cand lands on the previous evening.
	key := dayKey(time.Date(y, m, d, 12, 0, 0, 0, loc))
	c := cand.In(loc)
	if dayKey(c) == key && c.Hour() == 0 && c.Minute() == 0 && c.Second() == 0 &&
		dayKey(cand.Add(-time.Second).In(loc)) < key {
		return cand
	}
	// Slow path: the earliest instant whose local day is not before the target.
	lo := cand.Add(-48 * time.Hour).Unix()
	hi := cand.Add(48 * time.Hour).Unix()
	for lo < hi {
		mid := lo + (hi-lo)/2
		if dayKey(time.Unix(mid, 0).In(loc)) >= key {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	return time.Unix(lo, 0).In(loc)
}

// startOfDay is calendar.startOfDay(for:).
func startOfDay(t time.Time, loc *time.Location) time.Time {
	y, m, d := t.In(loc).Date()
	return startOfDayYMD(y, m, d, loc)
}

// startOfNextDay is the end of the local day t falls on.
func startOfNextDay(t time.Time, loc *time.Location) time.Time {
	y, m, d := t.In(loc).Date()
	return startOfDayYMD(y, m, d+1, loc)
}

// sameDay is calendar.isDate(_:inSameDayAs:).
func sameDay(a, b time.Time, loc *time.Location) bool {
	return dayKey(a.In(loc)) == dayKey(b.In(loc))
}

// addDay is calendar.date(byAdding: .day, value: 1, to:): the same wall-clock
// time on the next day.
func addDay(t time.Time, loc *time.Location) time.Time {
	return t.In(loc).AddDate(0, 0, 1)
}
