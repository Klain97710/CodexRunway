package resettoday

import (
	"strconv"
	"strings"
	"time"

	"github.com/licoy/quosparr/internal/export"
	"github.com/licoy/quosparr/internal/l10n"
)

// Formatter supplies the duration and schedule formatting of the detail copy
// (Swift DurationFormatter and ResetLabelFormatter). internal/resettoday/runwayfmt
// adapts internal/runway to it.
type Formatter interface {
	// RelativePastSingleUnit is "5 minutes" for a date in the past; ok is
	// false for a future date.
	RelativePastSingleUnit(date, now time.Time, lang l10n.Language) (string, bool)
	// ScheduledLabel is "yyyy/M/d HH:mm", or "start~end" for a range, in loc.
	ScheduledLabel(w ScheduleWindow, lang l10n.Language, loc *time.Location) string
}

// FormatterFuncs implements Formatter from plain functions.
type FormatterFuncs struct {
	Relative func(date, now time.Time, lang l10n.Language) (string, bool)
	Label    func(w ScheduleWindow, lang l10n.Language, loc *time.Location) string
}

// RelativePastSingleUnit implements Formatter.
func (f FormatterFuncs) RelativePastSingleUnit(date, now time.Time, lang l10n.Language) (string, bool) {
	return f.Relative(date, now, lang)
}

// ScheduledLabel implements Formatter.
func (f FormatterFuncs) ScheduledLabel(w ScheduleWindow, lang l10n.Language, loc *time.Location) string {
	return f.Label(w, lang, loc)
}

// TokenKind is the kind of a detail token.
type TokenKind int

// Token kinds.
const (
	TokenText TokenKind = iota
	TokenResetType
	TokenPercent
	TokenTime
)

// DetailToken is one piece of a detail line; Text is set for TokenText.
type DetailToken struct {
	Kind TokenKind
	Text string
}

// DetailPresentation is the explanatory line under the verdict.
type DetailPresentation struct {
	Tokens      []DetailToken
	ResetType   *ResetType
	TypeLabel   string
	PercentText *string
	TimeText    *string
	PlainText   string
}

type copyValue struct{ key, value string }

// replaceCopy substitutes {key} placeholders in the given order.
func replaceCopy(template string, values ...copyValue) string {
	for _, v := range values {
		template = strings.ReplaceAll(template, "{"+v.key+"}", v.value)
	}
	return template
}

var copyMarks = []struct {
	needle string
	token  TokenKind
}{
	{"{type}", TokenResetType}, {"{percent}", TokenPercent}, {"{probability}", TokenPercent},
	{"{when}", TokenTime}, {"{date}", TokenTime}, {"{ago}", TokenTime},
}

// Tokens splits a template into text and placeholder tokens.
func Tokens(template string) []DetailToken {
	var out []DetailToken
	rest := template
	for rest != "" {
		best, kind, length := -1, TokenText, 0
		for _, m := range copyMarks {
			if i := strings.Index(rest, m.needle); i >= 0 && (best < 0 || i < best) {
				best, kind, length = i, m.token, len(m.needle)
			}
		}
		if best < 0 {
			out = append(out, DetailToken{Kind: TokenText, Text: rest})
			break
		}
		if best > 0 {
			out = append(out, DetailToken{Kind: TokenText, Text: rest[:best]})
		}
		out = append(out, DetailToken{Kind: kind})
		rest = rest[best+length:]
	}
	if len(out) == 0 {
		return []DetailToken{{Kind: TokenText, Text: template}}
	}
	return out
}

// NoneSegment is a piece of the "no" hint template.
type NoneSegment struct {
	Kind NoneSegmentKind
	Text string
}

// NoneSegmentKind is the kind of a NoneSegment.
type NoneSegmentKind int

// Segment kinds.
const (
	SegmentText NoneSegmentKind = iota
	SegmentResetType
	SegmentAgo
)

// NoneHintSegments splits a "{type} ... {ago}" template (RateLimitResetNoneHint.segments).
func NoneHintSegments(template string) []NoneSegment {
	var out []NoneSegment
	rest := template
	for rest != "" {
		t, a := strings.Index(rest, "{type}"), strings.Index(rest, "{ago}")
		var idx, length int
		var kind NoneSegmentKind
		switch {
		case t >= 0 && (a < 0 || t <= a):
			idx, length, kind = t, len("{type}"), SegmentResetType
		case a >= 0:
			idx, length, kind = a, len("{ago}"), SegmentAgo
		default:
			out = append(out, NoneSegment{Kind: SegmentText, Text: rest})
			return out
		}
		if idx > 0 {
			out = append(out, NoneSegment{Kind: SegmentText, Text: rest[:idx]})
		}
		out = append(out, NoneSegment{Kind: kind})
		rest = rest[idx+length:]
	}
	if len(out) == 0 {
		return []NoneSegment{{Kind: SegmentText, Text: template}}
	}
	return out
}

// PlainNoneHint fills the "no" hint template.
func PlainNoneHint(template, typeName, ago string) string {
	return strings.ReplaceAll(strings.ReplaceAll(template, "{type}", typeName), "{ago}", ago)
}

// VerdictDetail is verdictDetail(l10n:now:calendar:); nil when there is nothing
// to say (a completion that cannot be dated).
func (s *Snapshot) VerdictDetail(l l10n.Localizer, f Formatter, now time.Time, loc *time.Location) *DetailPresentation {
	loc = locOrLocal(loc)
	p := s.VerdictPresentation(now, loc)
	if p.IsCompleted {
		return s.confirmedDetail(l, f, now, loc, p)
	}
	switch p.Reason {
	case export.ReasonUpcoming:
		return s.scheduledDetail(l, f, loc, p)
	case export.ReasonGrace:
		return s.graceDetail(l, f, now, loc, p)
	case export.ReasonExpiredUnconfirmed, export.ReasonNone:
		return s.noneDetail(l, f, now)
	case export.ReasonUnavailable:
		return s.unavailableDetail(l, now)
	}
	return nil
}

func strPtr(s string) *string { return &s }

func (s *Snapshot) confirmedDetail(l l10n.Localizer, f Formatter, now time.Time, loc *time.Location, p VerdictPresentation) *DetailPresentation {
	if p.CompletedAt == nil {
		return nil
	}
	resetAt := *p.CompletedAt
	ago, ok := f.RelativePastSingleUnit(resetAt, now, l.Language())
	if !ok {
		return nil
	}
	resetType := ResetGlobal
	if p.ResetType != nil {
		resetType = *p.ResetType
	}
	typeLabel := l.ResetTypeName(string(resetType))
	zone := l.Text(l10n.KeyRateLimitResetTodayClockLocal)
	label := f.ScheduledLabel(ScheduleWindow{StartAt: resetAt, EndAt: resetAt}, l.Language(), loc)
	when := l.Textf(l10n.KeyRateLimitResetTodayConfirmedWhen, label, ago)
	var percentText *string
	key := l10n.KeyRateLimitResetTodayConfirmedHintNoPercent
	values := []copyValue{{"type", typeLabel}, {"zone", zone}, {"when", when}}
	if p.Confidence != nil {
		percentText = strPtr(strconv.Itoa(DisplayedPercent(*p.Confidence)) + "%")
		key = l10n.KeyRateLimitResetTodayConfirmedHint
		values = append(values, copyValue{"percent", *percentText})
	}
	raw := l.Text(key)
	return &DetailPresentation{
		Tokens:      Tokens(replaceCopy(raw, copyValue{"zone", zone})),
		ResetType:   &resetType,
		TypeLabel:   typeLabel,
		PercentText: percentText,
		TimeText:    &when,
		PlainText:   replaceCopy(raw, values...),
	}
}

func optionalText(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (s *Snapshot) scheduledDetail(l l10n.Localizer, f Formatter, loc *time.Location, p VerdictPresentation) *DetailPresentation {
	if p.ScheduleWindow == nil || p.ResetType == nil {
		return nil
	}
	typeLabel := l.ResetTypeName(string(*p.ResetType))
	zone := l.Text(l10n.KeyRateLimitResetTodayClockLocal)
	date := f.ScheduledLabel(*p.ScheduleWindow, l.Language(), loc)
	percent := p.PercentText(l)
	key := l10n.KeyRateLimitResetTodayScheduledChanceHint
	if p.ScheduleBasis == BasisContextualInference {
		key = l10n.KeyRateLimitResetTodayScheduledContextualHint
	}
	values := []copyValue{{"type", typeLabel}, {"zone", zone}, {"date", date}}
	if percent != "" {
		values = append(values, copyValue{"percent", percent})
	}
	raw := l.Text(key)
	return &DetailPresentation{
		Tokens:      Tokens(replaceCopy(raw, copyValue{"zone", zone}, copyValue{"percent", percent})),
		ResetType:   p.ResetType,
		TypeLabel:   typeLabel,
		PercentText: optionalText(percent),
		TimeText:    &date,
		PlainText:   replaceCopy(raw, values...),
	}
}

func (s *Snapshot) graceDetail(l l10n.Localizer, f Formatter, now time.Time, loc *time.Location, p VerdictPresentation) *DetailPresentation {
	if p.ScheduleWindow == nil || p.ResetType == nil {
		return nil
	}
	typeLabel := l.ResetTypeName(string(*p.ResetType))
	zone := l.Text(l10n.KeyRateLimitResetTodayClockLocal)
	end := p.ScheduleWindow.PendingUntil()
	date := f.ScheduledLabel(ScheduleWindow{StartAt: end, EndAt: end}, l.Language(), loc)
	ago, _ := f.RelativePastSingleUnit(end, now, l.Language())
	when := l.Textf(l10n.KeyRateLimitResetTodayConfirmedWhen, date, ago)
	percent := p.PercentText(l)
	key := l10n.KeyRateLimitResetTodayScheduledGraceHint
	if p.ScheduleBasis == BasisContextualInference {
		key = l10n.KeyRateLimitResetTodayScheduledGraceContextualHint
	}
	values := []copyValue{{"type", typeLabel}, {"zone", zone}, {"when", when}}
	if percent != "" {
		values = append(values, copyValue{"probability", percent})
	}
	raw := l.Text(key)
	return &DetailPresentation{
		Tokens:      Tokens(replaceCopy(raw, copyValue{"zone", zone}, copyValue{"probability", percent})),
		ResetType:   p.ResetType,
		TypeLabel:   typeLabel,
		PercentText: optionalText(percent),
		TimeText:    &when,
		PlainText:   replaceCopy(raw, values...),
	}
}

func (s *Snapshot) noneDetail(l l10n.Localizer, f Formatter, now time.Time) *DetailPresentation {
	latest := s.NoneHintLastReset(l, f, now)
	if latest == nil {
		return plainDetail(l.Text(l10n.KeyRateLimitResetTodayNoHint))
	}
	raw := l.Text(l10n.KeyRateLimitResetTodayNoHintWithLast)
	rt := latest.ResetType
	ago := latest.Ago
	return &DetailPresentation{
		Tokens:    Tokens(raw),
		ResetType: &rt,
		TypeLabel: l.ResetTypeName(string(rt)),
		TimeText:  &ago,
		PlainText: latest.Text,
	}
}

func plainDetail(text string) *DetailPresentation {
	return &DetailPresentation{Tokens: []DetailToken{{Kind: TokenText, Text: text}}, PlainText: text}
}

func (s *Snapshot) unavailableDetail(l l10n.Localizer, now time.Time) *DetailPresentation {
	key := l10n.KeyRateLimitResetTodayUnavailableStaleHint
	switch {
	case s.Monitor.Status != MonitorOK:
		key = l10n.KeyRateLimitResetTodayUnavailableMonitorHint
	case now.Sub(s.FreshnessAt()) <= StaleAfter:
		key = l10n.KeyRateLimitResetTodayUnavailableHint
	}
	return plainDetail(l.Text(key))
}

// NoneHint is the hosted "no" copy naming the last reset.
type NoneHint struct {
	ResetType ResetType
	Ago       string
	Text      string
}

// NoneHintLastReset is the hosted "no" copy when nothing is completed or
// scheduled today but history exists; nil when there is no past reset.
func (s *Snapshot) NoneHintLastReset(l l10n.Localizer, f Formatter, now time.Time) *NoneHint {
	latest := s.LatestReset(now)
	if latest == nil {
		return nil
	}
	ago, ok := f.RelativePastSingleUnit(latest.At, now, l.Language())
	if !ok {
		return nil
	}
	typeName := l.ResetTypeName(string(latest.ResetType))
	template := l.Text(l10n.KeyRateLimitResetTodayNoHintWithLast)
	return &NoneHint{ResetType: latest.ResetType, Ago: ago, Text: PlainNoneHint(template, typeName, ago)}
}

// UnconfirmedScheduleHint is the hosted "no" copy for a schedule that expired
// without a completion; nil when there is none.
func (s *Snapshot) UnconfirmedScheduleHint(l l10n.Localizer, now time.Time, loc *time.Location) *NoneHint {
	e := s.UnconfirmedExpiredSchedule(now, loc)
	if e == nil {
		return nil
	}
	typeName := l.ResetTypeName(string(e.ResetType))
	template := l.Text(l10n.KeyRateLimitResetTodayNoHintUnconfirmedSchedule)
	return &NoneHint{ResetType: e.ResetType, Text: PlainNoneHint(template, typeName, "")}
}
