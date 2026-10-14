// Package runwayfmt adapts internal/runway's duration and schedule formatting
// to the resettoday.Formatter the detail copy needs. It lives in its own
// package so internal/resettoday stays free of an internal/runway dependency.
package runwayfmt

import (
	"time"

	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/resettoday"
	"github.com/licoy/quosparr/internal/runway"
)

// Formatter is the resettoday.Formatter backed by internal/runway.
type Formatter struct{}

var _ resettoday.Formatter = Formatter{}

// RelativePastSingleUnit implements resettoday.Formatter.
func (Formatter) RelativePastSingleUnit(date, now time.Time, lang l10n.Language) (string, bool) {
	return runway.RelativePastSingleUnit(date, now, lang)
}

// ScheduledLabel implements resettoday.Formatter.
func (Formatter) ScheduledLabel(w resettoday.ScheduleWindow, _ l10n.Language, loc *time.Location) string {
	return runway.ScheduledLabel(runway.ScheduleWindow{StartAt: w.StartAt, EndAt: w.EndAt}, loc)
}
