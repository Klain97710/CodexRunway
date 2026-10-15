package claude

import "time"

// DayKey is the local civil date of t, yyyy-MM-dd.
// Claude totals follow the local day, the same way Grok does. The Codex UTC
// day boundary does not apply here.
func DayKey(t time.Time, loc *time.Location) string {
	if loc == nil {
		loc = time.Local
	}
	return t.In(loc).Format("2006-01-02")
}
