package app

import (
	"context"
	"errors"
	"time"

	"github.com/licoy/quosparr/internal/resettoday"
)

type panelReactionState struct {
	initialized bool
	generation  uint64
	cancel      context.CancelFunc
	task        Task
	snapshot    *resettoday.ReactionSnapshot
	delta       int
	deltaAt     time.Time
}

// Reactions only post after an explicit click; merely opening details reads state.
func (a *App) startReaction(acts PanelActions, click bool) {
	s := &a.panelState.reaction
	if s.task.Running {
		return
	}
	if acts.QueryReaction == nil || (click && acts.React == nil) {
		s.task = Task{Err: errors.New(tx(a.L(), xDetailUnavailable))}
		return
	}
	before := s.snapshot
	if click {
		if before == nil || !before.IsVisible() || before.IsExhausted() {
			return
		}
		optimistic := resettoday.OptimisticClick(*before)
		s.snapshot = &optimistic
		s.delta, s.deltaAt = 1, time.Now()
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.generation++
	gen := s.generation
	s.task = Task{Running: true}
	go func() {
		var snap resettoday.ReactionSnapshot
		var err error
		if click {
			var result resettoday.ReactionPostResult
			result, err = acts.React(ctx)
			if err == nil {
				if result.Data != nil {
					snap = *result.Data
				}
				if !result.OK || result.Data == nil || !result.Data.IsVisible() {
					err = reactionResultError(result)
				}
			}
		} else {
			snap, err = acts.QueryReaction(ctx)
		}
		runOnMain(func() {
			if ctx.Err() != nil || a.panelState.reaction.generation != gen {
				return
			}
			s.finish(before, snap, click, err)
			a.redrawPanel()
		})
	}()
}

func (s *panelReactionState) finish(before *resettoday.ReactionSnapshot, snap resettoday.ReactionSnapshot, click bool, err error) {
	s.task = Task{Err: err}
	if err == nil {
		if click && before != nil {
			optimistic := resettoday.OptimisticClick(*before)
			if resettoday.PositiveDelta(before.Count, snap.Count, before.EpochID, snap.EpochID) == 0 {
				s.delta = 0
			} else if extra := resettoday.PositiveDelta(optimistic.Count, snap.Count, before.EpochID, snap.EpochID); extra > 0 {
				s.delta, s.deltaAt = extra, time.Now()
			}
		}
		s.snapshot = &snap
	} else if click {
		s.snapshot, s.delta = before, 0
	}
	if errors.Is(err, resettoday.ErrReactionRateLimited) && s.snapshot != nil {
		limited := *s.snapshot
		zero := 0
		limited.Remaining, limited.DailyLimit = &zero, max(1, limited.DailyLimit)
		s.snapshot = &limited
	}
}

func reactionResultError(r resettoday.ReactionPostResult) error {
	if r.Error != nil {
		switch *r.Error {
		case "daily_limit", "rate_limited":
			return resettoday.ErrReactionRateLimited
		case "forbidden":
			return resettoday.ErrReactionForbidden
		case "disabled":
			return resettoday.ErrReactionDisabled
		}
	}
	return resettoday.ErrReactionUnavailable
}
