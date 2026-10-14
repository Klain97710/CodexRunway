package resettoday

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ReactionDelta is a "+n" the UI floats once. ID changes for every
// announcement so an equal amount can be shown twice.
type ReactionDelta struct {
	ID     uint64
	Amount int
}

// ReactionState is what the UI renders.
type ReactionState struct {
	Reaction *ReactionSnapshot
	Busy     bool
	Loading  bool
	// Fresh: a fetch of the current session has completed.
	Fresh bool
	Delta ReactionDelta
}

// ReactionController owns the reaction polling lifecycle and the optimistic
// click (the RunwayModel reaction logic without any UI).
//
// Polling runs only while the panel is open: one fetch on open, then one every
// pollMs. A "disabled" answer stops it until polling is switched off and on
// again. A click is ignored while busy, while the session's first fetch is in
// flight, and while polling is wanted but no fresh state exists. Results that
// arrive after a click started (write generation) or after the session ended
// are discarded.
type ReactionController struct {
	FetchFunc func(ctx context.Context) (ReactionSnapshot, error)
	ClickFunc func(ctx context.Context) (ReactionPostResult, error)
	// OnChange is called after every state change, never concurrently, from any
	// goroutine. It must not block for long and may call State.
	OnChange func(ReactionState)
	// Sleep waits d or until ctx ends and reports whether the full time
	// passed; nil uses a real timer. Tests inject their own.
	Sleep func(ctx context.Context, d time.Duration) bool

	emitMu sync.Mutex

	mu         sync.Mutex
	root       context.Context
	rootCancel context.CancelFunc
	wg         sync.WaitGroup
	state      ReactionState
	desired    bool
	disabled   bool
	pollCancel context.CancelFunc
	generation int
	deltaSeq   uint64
}

func (c *ReactionController) ensureRoot() {
	if c.root == nil {
		c.root, c.rootCancel = context.WithCancel(context.Background())
	}
}

// State returns a copy of the current state.
func (c *ReactionController) State() ReactionState {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.copyState()
}

func (c *ReactionController) copyState() ReactionState {
	s := c.state
	if s.Reaction != nil {
		r := *s.Reaction
		s.Reaction = &r
	}
	return s
}

func (c *ReactionController) emit() {
	if c.OnChange == nil {
		return
	}
	c.emitMu.Lock()
	defer c.emitMu.Unlock()
	c.OnChange(c.State())
}

// SetPollingEnabled follows the panel: enabled is true while it is open;
// allowed is false when the section is not on screen at all (another provider
// is selected, or the section is hidden). Call it again with the same enabled
// when allowed changes.
func (c *ReactionController) SetPollingEnabled(enabled, allowed bool) {
	c.mu.Lock()
	c.ensureRoot()
	if enabled && !c.desired {
		c.disabled = false
	}
	c.desired = enabled
	if enabled && allowed && !c.disabled {
		if c.pollCancel == nil {
			c.beginSessionLocked()
		}
	} else {
		c.endSessionLocked()
	}
	c.mu.Unlock()
	c.emit()
}

func (c *ReactionController) beginSessionLocked() {
	c.state.Fresh = false
	c.state.Loading = true
	c.state.Delta = ReactionDelta{}
	ctx, cancel := context.WithCancel(c.root)
	c.pollCancel = cancel
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.poll(ctx)
	}()
}

func (c *ReactionController) endSessionLocked() {
	if c.pollCancel != nil {
		c.pollCancel()
		c.pollCancel = nil
	}
	c.state.Fresh = false
	c.state.Loading = false
	c.state.Delta = ReactionDelta{}
}

// Close stops polling and waits for the background work to end.
func (c *ReactionController) Close() {
	c.mu.Lock()
	c.ensureRoot()
	c.desired = false
	c.endSessionLocked()
	c.rootCancel()
	c.mu.Unlock()
	c.wg.Wait()
}

func (c *ReactionController) sleep(ctx context.Context, d time.Duration) bool {
	if c.Sleep != nil {
		return c.Sleep(ctx, d)
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}

func (c *ReactionController) wanted(ctx context.Context) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return ctx.Err() == nil && c.desired
}

func (c *ReactionController) busy() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state.Busy
}

// poll: one fetch on open, then wait the service's interval before each next.
func (c *ReactionController) poll(ctx context.Context) {
	for c.busy() && c.wanted(ctx) {
		if !c.sleep(ctx, 50*time.Millisecond) {
			return
		}
	}
	if !c.wanted(ctx) {
		return
	}
	c.load(ctx)
	for c.wanted(ctx) {
		c.mu.Lock()
		interval := 5 * time.Second
		if c.state.Reaction != nil {
			interval = time.Duration(c.state.Reaction.PollInterval() * float64(time.Second))
		}
		c.mu.Unlock()
		if !c.sleep(ctx, interval) || !c.wanted(ctx) {
			return
		}
		if c.busy() {
			continue
		}
		c.load(ctx)
	}
}

func (c *ReactionController) load(ctx context.Context) {
	c.mu.Lock()
	if c.state.Busy {
		c.mu.Unlock()
		return
	}
	generation := c.generation
	var prevCount *int
	var prevEpoch *string
	if r := c.state.Reaction; r != nil {
		prevCount, prevEpoch = r.Count, r.EpochID
	}
	announce := c.state.Fresh
	if !announce {
		c.state.Loading = true
	}
	c.mu.Unlock()
	if !announce {
		c.emit()
	}

	snapshot, err := c.FetchFunc(ctx)
	if ctx.Err() != nil {
		return
	}
	c.mu.Lock()
	switch {
	case err == nil:
		if generation != c.generation || c.state.Busy {
			c.mu.Unlock()
			return
		}
		s := snapshot
		c.state.Reaction = &s
		if announce {
			c.publishDeltaLocked(PositiveDelta(prevCount, s.Count, prevEpoch, s.EpochID))
		}
		c.state.Fresh = true
		c.state.Loading = false
	case errors.Is(err, ErrReactionDisabled):
		if generation != c.generation {
			c.mu.Unlock()
			return
		}
		c.disabled = true
		c.state.Reaction = nil
		c.state.Loading = false
		c.state.Fresh = false
		if c.pollCancel != nil {
			c.pollCancel()
			c.pollCancel = nil
		}
	default:
		c.state.Loading = false
	}
	c.mu.Unlock()
	c.emit()
}

func (c *ReactionController) publishDeltaLocked(amount int) {
	if amount <= 0 {
		return
	}
	c.deltaSeq++
	c.state.Delta = ReactionDelta{ID: c.deltaSeq, Amount: amount}
}

// Click counts one reaction optimistically and posts it in the background.
// It reports whether the click was accepted.
func (c *ReactionController) Click() bool {
	c.mu.Lock()
	c.ensureRoot()
	if c.state.Busy || c.state.Loading || (c.desired && !c.state.Fresh) {
		c.mu.Unlock()
		return false
	}
	current := c.state.Reaction
	if current == nil || !current.IsVisible() || current.IsExhausted() {
		c.mu.Unlock()
		return false
	}
	before := *current
	c.state.Busy = true
	c.generation++
	optimistic := OptimisticClick(before)
	c.state.Reaction = &optimistic
	c.publishDeltaLocked(1)
	root := c.root
	c.wg.Add(1)
	c.mu.Unlock()
	c.emit()
	go func() {
		defer c.wg.Done()
		c.finishClick(root, before)
	}()
	return true
}

func (c *ReactionController) finishClick(ctx context.Context, current ReactionSnapshot) {
	local := ReactionLocal{EpochID: current.EpochID, PreviousRemaining: current.Remaining}
	if current.Count != nil {
		local.PreviousCount = *current.Count
	}
	var parsed *ReactionPostResult
	if res, err := c.ClickFunc(ctx); err == nil {
		parsed = &res
	}
	next := ReconcileAfterPost(local, parsed)
	updated := current
	if next.Payload != nil {
		updated = *next.Payload
	}
	count := next.Count
	updated.Count = &count
	if next.Exhausted && updated.DailyLimit > 0 {
		zero := 0
		updated.Remaining = &zero
	} else {
		updated.Remaining = next.Remaining
	}
	c.mu.Lock()
	c.state.Reaction = &updated
	c.state.Busy = false
	if count > local.PreviousCount {
		// +1 was already published locally; only float extra live clicks.
		prev := local.PreviousCount + 1
		c.publishDeltaLocked(PositiveDelta(&prev, updated.Count, local.EpochID, updated.EpochID))
	} else {
		c.state.Delta = ReactionDelta{}
	}
	c.mu.Unlock()
	c.emit()
}
