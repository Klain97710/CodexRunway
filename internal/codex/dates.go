package codex

import (
	"strconv"
	"time"

	"github.com/licoy/quosparr/internal/export/swiftjson"
)

// FormatTime renders t the way the Swift app writes dates into index.json and
// transfer packs: ISO-8601 in UTC with millisecond precision, rounded to the
// nearest millisecond ("2026-06-29T05:13:20.123Z").
func FormatTime(t time.Time) string {
	return swiftjson.FormatMillis(t)
}

// formatSeconds renders t as ISO-8601 UTC to whole seconds, the format of
// auth.json's last_refresh ("2026-06-29T05:13:20Z").
func formatSeconds(t time.Time) string {
	return t.UTC().Truncate(time.Second).Format("2006-01-02T15:04:05Z")
}

// ParseTime reads an ISO-8601 date with or without fractional seconds and with
// either a Z or a numeric UTC offset, the shapes the Swift app accepts.
func ParseTime(s string) (time.Time, bool) {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

func itoa(n int) string { return strconv.Itoa(n) }
