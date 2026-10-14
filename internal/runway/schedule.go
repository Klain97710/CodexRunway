package runway

import "time"

// farFuture stands for Swift's Date.distantFuture.
var farFuture = time.Date(4001, 1, 1, 0, 0, 0, 0, time.UTC)

// Schedule decides when the next automatic refresh is due (Swift
// RefreshSchedule). While a refresh is in flight no deadline is armed; the
// deadline only re-arms when the refresh completes, so a slow refresh never
// stacks a second one behind it. The zero value is not armed.
type Schedule struct {
	deadline         time.Time
	latestCompletion time.Time
	inFlight         bool
	armed            bool
}

// NewSchedule returns a schedule with nothing due.
func NewSchedule() Schedule { return Schedule{deadline: farFuture, armed: true} }

func (s *Schedule) init() {
	if !s.armed {
		s.deadline = farFuture
		s.armed = true
	}
}

// RefreshStarted marks a refresh as running and disarms the deadline.
func (s *Schedule) RefreshStarted() {
	s.init()
	s.inFlight = true
	s.deadline = farFuture
}

// RefreshCompleted arms the next deadline interval after completion.
func (s *Schedule) RefreshCompleted(at time.Time, interval time.Duration) {
	s.init()
	s.inFlight = false
	s.latestCompletion = at
	s.deadline = at.Add(interval)
}

// IntervalChanged re-arms the deadline for a new interval, measured from the
// last completion (or now when nothing completed yet). It is a no-op while a
// refresh is running: the completion arms the deadline with the new interval.
func (s *Schedule) IntervalChanged(interval time.Duration, now time.Time) {
	s.init()
	if s.inFlight {
		return
	}
	base := now
	if !s.latestCompletion.IsZero() {
		base = s.latestCompletion
	}
	s.deadline = base.Add(interval)
}

// IsDue reports whether a refresh should start at now.
func (s *Schedule) IsDue(now time.Time) bool {
	s.init()
	return !s.inFlight && !now.Before(s.deadline)
}

// InFlight reports whether a refresh is running.
func (s *Schedule) InFlight() bool { return s.inFlight }
