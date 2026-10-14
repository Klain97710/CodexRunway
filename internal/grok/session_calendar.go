package grok

import (
	"fmt"
	"math"
	"strconv"
	"time"
)

// The day keys of the session scan are written and read by a DateFormatter
// on the system Gregorian calendar, which is the Julian calendar before
// 1582-10-15, shows years before the common era as positive era years and
// stops at about +-5.8 million years. These helpers reproduce that.

// The calendar represents the instants (seconds from 1970) from
// sessionCalendarMin to sessionCalendarMax; later or earlier ones are clamped.
const (
	sessionCalendarMin = -184303902528000
	sessionCalendarMax = 183882168921600
)

// sessionGregorianStartJDN is the Julian day number of 1582-10-15.
const sessionGregorianStartJDN = 2299161

func sessionFloorDiv(a, b int64) int64 {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}

func sessionGregorianToJDN(y, m, d int64) int64 {
	a := sessionFloorDiv(14-m, 12)
	yy := y + 4800 - a
	mm := m + 12*a - 3
	return d + sessionFloorDiv(153*mm+2, 5) + 365*yy + sessionFloorDiv(yy, 4) - sessionFloorDiv(yy, 100) + sessionFloorDiv(yy, 400) - 32045
}

func sessionJulianToJDN(y, m, d int64) int64 {
	a := sessionFloorDiv(14-m, 12)
	yy := y + 4800 - a
	mm := m + 12*a - 3
	return d + sessionFloorDiv(153*mm+2, 5) + 365*yy + sessionFloorDiv(yy, 4) - 32083
}

func sessionJDNToGregorian(jdn int64) (y, m, d int64) {
	a := jdn + 32044
	b := sessionFloorDiv(4*a+3, 146097)
	c := a - sessionFloorDiv(146097*b, 4)
	dd := sessionFloorDiv(4*c+3, 1461)
	e := c - sessionFloorDiv(1461*dd, 4)
	mm := sessionFloorDiv(5*e+2, 153)
	d = e - sessionFloorDiv(153*mm+2, 5) + 1
	m = mm + 3 - 12*sessionFloorDiv(mm, 10)
	y = 100*b + dd - 4800 + sessionFloorDiv(mm, 10)
	return
}

func sessionJDNToJulian(jdn int64) (y, m, d int64) {
	c := jdn + 32082
	dd := sessionFloorDiv(4*c+3, 1461)
	e := c - sessionFloorDiv(1461*dd, 4)
	mm := sessionFloorDiv(5*e+2, 153)
	d = e - sessionFloorDiv(153*mm+2, 5) + 1
	m = mm + 3 - 12*sessionFloorDiv(mm, 10)
	y = dd - 4800 + sessionFloorDiv(mm, 10)
	return
}

// sessionDayLabel is the yyyy-MM-dd key of a civil date given in the
// proleptic Gregorian calendar.
func sessionDayLabel(y, m, d int) string {
	jdn := sessionGregorianToJDN(int64(y), int64(m), int64(d))
	yy, mm, dd := int64(y), int64(m), int64(d)
	if jdn < sessionGregorianStartJDN {
		yy, mm, dd = sessionJDNToJulian(jdn)
		if yy <= 0 {
			yy = 1 - yy
		}
	}
	return fmt.Sprintf("%04d-%02d-%02d", yy, mm, dd)
}

// sessionDayOf is the key of the local day an instant falls on.
func sessionDayOf(t time.Time, loc *time.Location) string {
	y, m, d := t.In(loc).Date()
	return sessionDayLabel(y, int(m), d)
}

// sessionDayString is the local calendar day key of Unix seconds.
func sessionDayString(seconds float64, loc *time.Location) string {
	if math.IsNaN(seconds) {
		seconds = 0
	}
	seconds = math.Max(sessionCalendarMin, math.Min(sessionCalendarMax, seconds))
	whole := math.Floor(seconds)
	return sessionDayOf(time.Unix(int64(whole), int64((seconds-whole)*1e9)), loc)
}

// sessionDayDate is the local midnight of a day key (always read as a date
// of the common era).
func sessionDayDate(day string, loc *time.Location) (time.Time, bool) {
	if len(day) < 10 {
		return time.Time{}, false
	}
	y, e1 := strconv.ParseInt(day[:len(day)-6], 10, 64)
	m, e2 := strconv.ParseInt(day[len(day)-5:len(day)-3], 10, 64)
	d, e3 := strconv.ParseInt(day[len(day)-2:], 10, 64)
	if e1 != nil || e2 != nil || e3 != nil {
		return time.Time{}, false
	}
	if jdn := sessionJulianToJDN(y, m, d); jdn < sessionGregorianStartJDN {
		y, m, d = sessionJDNToGregorian(jdn)
	}
	return sessionDayStart(int(y), int(m), int(d), loc), true
}
