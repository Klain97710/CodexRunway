package usage

import (
	"math"
	"time"
	"unicode"
	"unicode/utf8"
)

// distantPast is Foundation's Date.distantPast in Unix seconds. Records whose
// timestamp cannot be parsed get this value, which keeps them outside every
// realistic window.
const distantPast = -62135769600.0

// parseTimestamp converts a log timestamp to Unix seconds. The fast path
// handles the exact "YYYY-MM-DDTHH:MM:SS[.f{1,9}]Z" form Codex writes; anything
// else goes through a lenient ISO-8601 parser that mimics Foundation's.
func parseTimestamp(text string) (float64, bool) {
	if v, ok := parseZulu(text); ok {
		return swiftDate(v), true
	}
	if v, ok := parseISOFallback(text); ok {
		return swiftDate(v), true
	}
	return 0, false
}

func digit2(b string, at int) (int, bool) {
	d1, d2 := b[at], b[at+1]
	if d1 < '0' || d1 > '9' || d2 < '0' || d2 > '9' {
		return 0, false
	}
	return int(d1-'0')*10 + int(d2-'0'), true
}

// parseZulu is the strict fast path. It reproduces the C implementation it
// replaces, quirks included: a year before 1900 yields timegm's -1 error value
// without failing the normalization check, and lower-case "z" is not accepted.
func parseZulu(b string) (float64, bool) {
	if len(b) < 20 || b[4] != '-' || b[7] != '-' || b[10] != 'T' || b[13] != ':' || b[16] != ':' {
		return 0, false
	}
	var year int
	for i := 0; i < 4; i++ {
		if b[i] < '0' || b[i] > '9' {
			return 0, false
		}
		year = year*10 + int(b[i]-'0')
	}
	month, ok1 := digit2(b, 5)
	day, ok2 := digit2(b, 8)
	hour, ok3 := digit2(b, 11)
	minute, ok4 := digit2(b, 14)
	second, ok5 := digit2(b, 17)
	if !(ok1 && ok2 && ok3 && ok4 && ok5) {
		return 0, false
	}
	fraction := 0.0
	if len(b) == 20 {
		if b[19] != 'Z' {
			return 0, false
		}
	} else {
		if b[19] != '.' || b[len(b)-1] != 'Z' {
			return 0, false
		}
		digits := len(b) - 21
		if digits < 1 || digits > 9 {
			return 0, false
		}
		scale := 0.1
		for i := 20; i < len(b)-1; i++ {
			if b[i] < '0' || b[i] > '9' {
				return 0, false
			}
			fraction += float64(float64(b[i]-'0') * scale)
			scale = float64(scale * 0.1)
		}
	}
	var seconds float64
	if year < 1900 {
		seconds = -1
	} else {
		if month < 1 || month > 12 || day < 1 || day > daysInMonth(year, month) ||
			hour > 23 || minute > 59 || second > 59 {
			return 0, false
		}
		seconds = float64(civilSeconds(year, month, day, hour, minute, second))
	}
	return float64(seconds + fraction), true
}

func isLeap(y int) bool { return y%4 == 0 && (y%100 != 0 || y%400 == 0) }

func daysInMonth(y, m int) int {
	switch m {
	case 2:
		if isLeap(y) {
			return 29
		}
		return 28
	case 4, 6, 9, 11:
		return 30
	}
	return 31
}

// daysFromCivil returns days since 1970-01-01 for a proleptic Gregorian date.
// Days beyond the end of the month roll into following months.
func daysFromCivil(y, m, d int) int64 {
	yy := int64(y)
	if m <= 2 {
		yy--
	}
	era := yy / 400
	if yy < 0 {
		era = (yy - 399) / 400
	}
	yoe := yy - era*400
	mp := int64((m + 9) % 12)
	doy := (153*mp+2)/5 + 0
	doe := yoe*365 + yoe/4 - yoe/100 + doy
	return era*146097 + doe - 719468 + int64(d-1)
}

func civilSeconds(y, m, d, h, mi, s int) int64 {
	return daysFromCivil(y, m, d)*86400 + int64(h)*3600 + int64(mi)*60 + int64(s)
}

// ---- lenient ISO-8601 fallback -------------------------------------------

// fallbackSpace reports the blanks Foundation tolerates around the zone.
func fallbackSpace(r rune) bool { return r == ' ' || r == 0xA0 }

// uniDigit returns the decimal value of a Unicode decimal digit.
func uniDigit(r rune) (int, bool) {
	if r >= '0' && r <= '9' {
		return int(r - '0'), true
	}
	if r < 0x80 || !unicode.IsDigit(r) {
		return 0, false
	}
	// Decimal digits come in aligned runs of ten starting at a zero.
	z := r
	for unicode.IsDigit(z - 1) {
		z--
	}
	return int((r - z) % 10), true
}

type isoScanner struct {
	r []rune
	p int
}

func (s *isoScanner) peek() rune {
	if s.p < len(s.r) {
		return s.r[s.p]
	}
	return -1
}

func (s *isoScanner) number(maxDigits int) (int, int, bool) {
	n, count := 0, 0
	for s.p < len(s.r) && (maxDigits <= 0 || count < maxDigits) {
		d, ok := uniDigit(s.r[s.p])
		if !ok {
			break
		}
		if n < 1_000_000_000 {
			n = n*10 + d
		}
		count++
		s.p++
	}
	return n, count, count > 0
}

func (s *isoScanner) expect(c rune) bool {
	if s.peek() == c {
		s.p++
		return true
	}
	return false
}

// parseISOFallback accepts what Foundation's ISO8601DateFormatter accepts for
// the internet date-time format, with or without fractional seconds: padding
// is optional, the day may exceed the month length (it rolls over), the time
// may be 24:mm:ss, fractions are truncated to milliseconds, and text after the
// zone is ignored.
func parseISOFallback(text string) (float64, bool) {
	if !utf8.ValidString(text) {
		return 0, false
	}
	s := &isoScanner{r: []rune(text)}
	for fallbackSpace(s.peek()) {
		s.p++
	}
	year, _, ok := s.number(0)
	if !ok || !s.expect('-') {
		return 0, false
	}
	year %= 10000
	month, _, ok := s.number(2)
	if !ok || month < 1 || month > 12 || !s.expect('-') {
		return 0, false
	}
	day, _, ok := s.number(2)
	if !ok || day < 1 || day > 31 || !s.expect('T') {
		return 0, false
	}
	hour, _, ok := s.number(2)
	if !ok || hour > 24 || !s.expect(':') {
		return 0, false
	}
	minute, _, ok := s.number(2)
	if !ok || minute > 59 || !s.expect(':') {
		return 0, false
	}
	second, _, ok := s.number(2)
	if !ok || second > 59 {
		return 0, false
	}
	millis := 0
	if s.peek() == '.' {
		s.p++
		ms, ok := s.fractionMillis()
		if !ok {
			return 0, false
		}
		millis = ms
	}
	for fallbackSpace(s.peek()) {
		s.p++
	}
	offset, ok := s.zone()
	if !ok {
		return 0, false
	}
	base := civilSeconds(year, month, day, hour, minute, second) - int64(offset)
	return float64(base) + float64(millis)/1000, true
}

// fractionMillis reads the fraction digits and returns them as whole
// milliseconds (truncated). The system parser keeps the digits in a 32-bit
// integer and divides by a 32-bit power of ten, so fractions longer than nine
// digits wrap around; this reproduces that, because the results are
// observable (a 12-digit fraction yields -1 ms).
func (s *isoScanner) fractionMillis() (int, bool) {
	var acc uint32
	count := 0
	for {
		d, ok := uniDigit(s.peek())
		if !ok {
			break
		}
		acc = acc*10 + uint32(d)
		count++
		s.p++
	}
	if count == 0 {
		return 0, false
	}
	v := int32(acc)
	switch {
	case count <= 3:
		ms := int(v)
		for i := count; i < 3; i++ {
			ms *= 10
		}
		return ms, true
	default:
		var div uint32 = 1
		for i := 0; i < count-3; i++ {
			div *= 10
		}
		if div == 0 {
			return 0, false
		}
		return int(v / int32(div)), true
	}
}

// zone parses "Z", "GMT", "UTC" or a numeric offset and returns seconds east of
// UTC.
func (s *isoScanner) zone() (int, bool) {
	c := s.peek()
	switch c {
	case 'Z', 'z':
		s.p++
		return 0, true
	case 'G':
		if s.hasWord("GMT") {
			return 0, true
		}
		return 0, false
	case 'U':
		if s.hasWord("UTC") {
			return 0, true
		}
		return 0, false
	case '+', '-':
		s.p++
		sign := 1
		if c == '-' {
			sign = -1
		}
		return sign * s.offsetFields(), true
	}
	return 0, false
}

func (s *isoScanner) hasWord(w string) bool {
	rs := []rune(w)
	if s.p+len(rs) > len(s.r) {
		return false
	}
	for i, r := range rs {
		if s.r[s.p+i] != r {
			return false
		}
	}
	s.p += len(rs)
	return true
}

// offsetFields reads the hh[:mm] / hhmm part of a numeric offset. An hour above
// 23 keeps only its tens digit, and minutes above 59 or with a single digit
// are ignored, as in the system parser.
func (s *isoScanner) offsetFields() int {
	start := s.p
	hours, hd, ok := s.number(2)
	if !ok {
		return 0
	}
	secs := 0
	if hours > 23 {
		hours /= 10
		_ = hd
	}
	secs = hours * 3600
	save := s.p
	hasColon := s.expect(':')
	n, count, ok := s.number(2)
	if ok && count == 2 && n <= 59 {
		secs += n * 60
	} else if hasColon {
		s.p = save
	}
	_ = start
	return secs
}

// ---- day keys ------------------------------------------------------------

// dayKey formats the calendar day of ts (Unix seconds) in loc as yyyy-MM-dd.
// Years at or before 0 are printed as era years like the system formatter.
func dayKey(ts float64, loc *time.Location) string {
	if math.IsNaN(ts) || math.IsInf(ts, 0) {
		ts = distantPast
	}
	secs := int64(math.Floor(ts))
	t := time.Unix(secs, 0).In(loc)
	if _, off := t.Zone(); secs+int64(off) < gregorianCutoverSeconds {
		y, m, d := julianDate(floorDiv(secs+int64(off), 86400))
		return formatDay(y, m, d)
	}
	y, m, d := t.Date()
	return formatDay(y, int(m), d)
}

// gregorianCutoverSeconds is 1582-10-15T00:00 local, the day the system
// calendar switches from the Julian to the Gregorian rules.
const gregorianCutoverSeconds = -12219292800

func floorDiv(a, b int64) int64 {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}

// julianDate converts days since 1970-01-01 to a proleptic Julian calendar
// date (astronomical year numbering).
func julianDate(days int64) (year, month, day int) {
	jdn := days + 2440588
	c := jdn + 32082
	d := (4*c + 3) / 1461
	e := c - 1461*d/4
	m := (5*e + 2) / 153
	day = int(e - (153*m+2)/5 + 1)
	month = int(m + 3 - 12*(m/10))
	year = int(d - 4800 + m/10)
	return
}

func formatDay(y, m, d int) string {
	if y <= 0 {
		y = 1 - y
	}
	b := make([]byte, 0, 11)
	if y >= 10000 {
		b = appendInt(b, y)
	} else {
		b = append(b, byte('0'+y/1000), byte('0'+y/100%10), byte('0'+y/10%10), byte('0'+y%10))
	}
	b = append(b, '-', byte('0'+m/10), byte('0'+m%10), '-', byte('0'+d/10), byte('0'+d%10))
	return string(b)
}

func appendInt(b []byte, v int) []byte {
	var tmp [20]byte
	i := len(tmp)
	for v > 0 {
		i--
		tmp[i] = byte('0' + v%10)
		v /= 10
	}
	return append(b, tmp[i:]...)
}
