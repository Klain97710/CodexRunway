package grok

import (
	"strings"
	"time"
	"unicode"

	"github.com/licoy/quosparr/internal/export/swiftjson"
)

// Dates in Grok credential files and in grok-index.json go through
// ISO8601DateFormatter in the Swift app. These helpers match what the
// formatter reads and writes, including its leniencies (one-digit month and
// day, a 24:00:00 hour, +HHMM offsets, fractions cut to milliseconds).

// acctFormatMillis renders t like the index does: UTC, rounded to the
// nearest millisecond ("2026-07-31T12:00:00.124Z").
func acctFormatMillis(t time.Time) string {
	return swiftjson.FormatMillis(t)
}

// acctFormatSeconds renders t like credential files do: UTC, whole seconds
// with the fraction dropped ("2026-07-31T12:01:00Z").
func acctFormatSeconds(t time.Time) string {
	return t.Truncate(time.Second).UTC().Format("2006-01-02T15:04:05Z")
}

// acctParseDate reads an internet date-time with an optional fraction and a
// required zone (Z or a numeric offset). A fraction is cut, not rounded, to
// milliseconds. It reports false for anything else.
func acctParseDate(raw string) (time.Time, bool) {
	s := strings.TrimFunc(raw, unicode.IsSpace)
	datePart, rest, ok := strings.Cut(s, "T")
	if !ok {
		return time.Time{}, false
	}
	year, month, day, ok := acctSplitDate(datePart)
	if !ok {
		return time.Time{}, false
	}
	hour, minute, second, rest, ok := acctSplitClock(rest)
	if !ok {
		return time.Time{}, false
	}
	nanos, rest, ok := acctSplitFraction(rest)
	if !ok {
		return time.Time{}, false
	}
	offset, ok := acctSplitZone(rest)
	if !ok {
		return time.Time{}, false
	}
	if hour > 24 || (hour == 24 && (minute != 0 || second != 0)) || minute > 59 || second > 59 {
		return time.Time{}, false
	}
	t := time.Date(year, time.Month(month), day, hour, minute, second, nanos, time.UTC)
	return t.Add(-time.Duration(offset) * time.Second), true
}

func acctSplitDate(s string) (year, month, day int, ok bool) {
	parts := strings.Split(s, "-")
	if len(parts) != 3 || len(parts[0]) != 4 || len(parts[1]) == 0 || len(parts[1]) > 2 || len(parts[2]) == 0 || len(parts[2]) > 2 {
		return 0, 0, 0, false
	}
	year, ok1 := acctDigits(parts[0])
	month, ok2 := acctDigits(parts[1])
	day, ok3 := acctDigits(parts[2])
	if !ok1 || !ok2 || !ok3 || month < 1 || month > 12 || day < 1 || day > 31 {
		return 0, 0, 0, false
	}
	return year, month, day, true
}

// acctSplitClock reads HH:MM:SS and returns what follows.
func acctSplitClock(s string) (hour, minute, second int, rest string, ok bool) {
	if len(s) < 8 || s[2] != ':' || s[5] != ':' {
		return 0, 0, 0, "", false
	}
	hour, ok1 := acctDigits(s[0:2])
	minute, ok2 := acctDigits(s[3:5])
	second, ok3 := acctDigits(s[6:8])
	if !ok1 || !ok2 || !ok3 {
		return 0, 0, 0, "", false
	}
	return hour, minute, second, s[8:], true
}

// acctSplitFraction reads an optional ".digits" and returns nanoseconds cut to
// the millisecond.
func acctSplitFraction(s string) (nanos int, rest string, ok bool) {
	if !strings.HasPrefix(s, ".") {
		return 0, s, true
	}
	end := 1
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	digits := s[1:end]
	if digits == "" {
		return 0, "", false
	}
	if len(digits) > 9 {
		// Foundation overflows on such fractions and returns noise; a
		// zero fraction is the closest stable reading.
		return 0, s[end:], true
	}
	for len(digits) < 3 {
		digits += "0"
	}
	millis, _ := acctDigits(digits[:3])
	return millis * int(time.Millisecond), s[end:], true
}

// acctSplitZone reads Z or ±HH, ±HHMM, ±HH:MM and returns the offset east of
// UTC in seconds.
func acctSplitZone(s string) (seconds int, ok bool) {
	if s == "Z" {
		return 0, true
	}
	if len(s) < 3 || (s[0] != '+' && s[0] != '-') {
		return 0, false
	}
	hours, ok := acctDigits(s[1:3])
	if !ok {
		return 0, false
	}
	minutes := 0
	switch tail := s[3:]; {
	case tail == "":
	case len(tail) == 3 && tail[0] == ':':
		minutes, ok = acctDigits(tail[1:])
	case len(tail) == 2:
		minutes, ok = acctDigits(tail)
	default:
		ok = false
	}
	if !ok || minutes > 59 {
		return 0, false
	}
	total := hours*3600 + minutes*60
	if s[0] == '-' {
		total = -total
	}
	return total, true
}

func acctDigits(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, false
		}
		n = n*10 + int(s[i]-'0')
	}
	return n, true
}
