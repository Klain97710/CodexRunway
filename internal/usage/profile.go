package usage

import (
	"errors"
	"math"
	"strconv"
	"strings"
	"time"
)

// ProfileTokenUsage is the daily token activity of the signed-in account as
// reported by the Codex profile statistics endpoint.
type ProfileTokenUsage struct {
	// DailyTokens maps "yyyy-MM-dd" (UTC days) to token counts.
	DailyTokens map[string]int64
	// StatsAsOf is the day the statistics are current to; empty when absent.
	StatsAsOf string
	// GeneratedAt is when the server produced the statistics; the zero time
	// when absent.
	GeneratedAt time.Time
}

// ErrInvalidProfile reports a profile statistics response that cannot be used.
var ErrInvalidProfile = errors.New("usage: invalid Codex profile statistics")

type profileBucket struct {
	startDate *string
	tokens    *int64
}

// DecodeProfileTokenUsage parses a profile statistics response with the same
// acceptance rules as the Swift decoder: a missing, null or negative value, a
// duplicate or impossible date, a reported stats error, or an unparseable
// timestamp rejects the whole response.
func DecodeProfileTokenUsage(data []byte) (ProfileTokenUsage, error) {
	var (
		buckets                            []profileBucket
		haveBuckets, haveStats             bool
		statsError, statsAsOf, generatedAt *string
	)
	d := &jsonDec{b: data}
	if len(d.b) >= 3 && d.b[0] == 0xEF && d.b[1] == 0xBB && d.b[2] == 0xBF {
		d.i = 3
	}
	err := d.object(func(key string, first bool) error {
		if !first {
			return d.skip()
		}
		switch key {
		case "stats":
			haveStats = true
			return d.object(func(k string, first bool) error {
				if k != "daily_usage_buckets" || !first {
					return d.skip()
				}
				if d.peek() == 'n' {
					if d.literal("null") {
						return nil
					}
					return errMalformed
				}
				haveBuckets = true
				return d.elements(func() error {
					var b profileBucket
					err := d.object(func(bk string, first bool) error {
						if !first {
							return d.skip()
						}
						switch bk {
						case "start_date":
							s, err := d.requiredString()
							b.startDate = &s
							return err
						case "tokens":
							v, err := d.flex()
							if err != nil {
								return err
							}
							if v.kind != flexNumber {
								return errMalformed
							}
							n, ok := parseIntegralJSON(v.tok)
							if !ok {
								return errMalformed
							}
							b.tokens = &n
							return nil
						}
						return d.skip()
					})
					if err != nil {
						return err
					}
					if b.startDate == nil || b.tokens == nil {
						return errMalformed
					}
					buckets = append(buckets, b)
					return nil
				})
			})
		case "metadata":
			return d.optObject(func(k string, first bool) error {
				if !first {
					return d.skip()
				}
				switch k {
				case "stats_error":
					return d.optString(&statsError)
				case "stats_as_of":
					return d.optString(&statsAsOf)
				case "generated_at":
					return d.optString(&generatedAt)
				}
				return d.skip()
			})
		}
		return d.skip()
	})
	if err == nil {
		d.skipSpace()
		if d.i != len(d.b) || !haveStats {
			err = errMalformed
		}
	}
	if err != nil {
		return ProfileTokenUsage{}, ErrInvalidProfile
	}
	if s := trimmed(statsError); s != "" || !haveBuckets {
		return ProfileTokenUsage{}, ErrInvalidProfile
	}
	out := ProfileTokenUsage{DailyTokens: make(map[string]int64, len(buckets))}
	for _, b := range buckets {
		if !isValidProfileDay(*b.startDate) || *b.tokens < 0 {
			return ProfileTokenUsage{}, ErrInvalidProfile
		}
		if _, dup := out.DailyTokens[*b.startDate]; dup {
			return ProfileTokenUsage{}, ErrInvalidProfile
		}
		out.DailyTokens[*b.startDate] = *b.tokens
	}
	if s := trimmed(statsAsOf); s != "" {
		if !isValidProfileDay(s) {
			return ProfileTokenUsage{}, ErrInvalidProfile
		}
		out.StatsAsOf = s
	}
	if s := trimmed(generatedAt); s != "" {
		secs, ok := parseISOFallback(s)
		if !ok {
			return ProfileTokenUsage{}, ErrInvalidProfile
		}
		// ISO8601DateFormatter keeps milliseconds only.
		ms := int64(math.Round(secs * 1000))
		out.GeneratedAt = time.UnixMilli(ms).UTC()
	}
	return out, nil
}

func trimmed(s *string) string {
	if s == nil {
		return ""
	}
	return strings.TrimFunc(*s, isSwiftWhitespace)
}

// isValidProfileDay accepts "YYYY-MM-DD" whose parts are 4, 2 and 2 characters
// long, parse as integers (a sign counts as a character) and name a real
// calendar day, using the Julian rules before 1582-10-15 like the system
// Gregorian calendar.
func isValidProfileDay(v string) bool {
	parts := strings.Split(v, "-")
	if len(parts) != 3 || len(parts[0]) != 4 || len(parts[1]) != 2 || len(parts[2]) != 2 {
		return false
	}
	var n [3]int
	for i, p := range parts {
		x, err := strconv.ParseInt(p, 10, 32)
		if err != nil {
			return false
		}
		n[i] = int(x)
	}
	y, m, d := n[0], n[1], n[2]
	if y < 1 || m < 1 || m > 12 || d < 1 {
		return false
	}
	if y == 1582 && m == 10 && d >= 5 && d <= 14 {
		return false
	}
	dim := daysInMonth(y, m)
	if y < 1582 || (y == 1582 && m < 10) || (y == 1582 && m == 10 && d < 5) {
		if m == 2 && y%4 == 0 {
			dim = 29
		} else if m == 2 {
			dim = 28
		}
	}
	return d <= dim
}
