package resettoday

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// tick is the injected Sleep: every wait lasts until the test releases it.
type tick struct {
	ch      chan struct{}
	sleeps  atomic.Int32
	lastDur atomic.Int64
}

func newTick() *tick { return &tick{ch: make(chan struct{}, 16)} }

func (k *tick) Sleep(ctx context.Context, d time.Duration) bool {
	k.sleeps.Add(1)
	k.lastDur.Store(int64(d))
	select {
	case <-k.ch:
		return true
	case <-ctx.Done():
		return false
	}
}

func (k *tick) release() { k.ch <- struct{}{} }

func waitUntil(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("timed out waiting for state")
}

type gate struct {
	once sync.Once
	ch   chan struct{}
}

func newGate() *gate  { return &gate{ch: make(chan struct{})} }
func (g *gate) open() { g.once.Do(func() { close(g.ch) }) }
func (g *gate) wait(ctx context.Context) {
	select {
	case <-g.ch:
	case <-ctx.Done():
	}
}

func countIs(c *ReactionController, n int) func() bool {
	return func() bool {
		r := c.State().Reaction
		return r != nil && r.Count != nil && *r.Count == n
	}
}

func snapshotWith(count, pollMs int) ReactionSnapshot {
	s := reactionSnap("abc", count, ptr(2), 5, PolarityNo)
	s.PollMs = ClampedPollMs(pollMs)
	return s
}

func newController(fetch func(context.Context) (ReactionSnapshot, error), click func(context.Context) (ReactionPostResult, error)) (*ReactionController, *tick) {
	k := newTick()
	return &ReactionController{FetchFunc: fetch, ClickFunc: click, Sleep: k.Sleep}, k
}

func TestPollingFollowsPanelVisibility(t *testing.T) {
	var fetches atomic.Int32
	snap := snapshotWith(20, 5000)
	c, k := newController(func(context.Context) (ReactionSnapshot, error) { fetches.Add(1); return snap, nil }, nil)
	defer c.Close()

	c.SetPollingEnabled(true, true)
	waitUntil(t, func() bool { return fetches.Load() >= 1 })
	waitUntil(t, func() bool { return k.sleeps.Load() >= 1 })
	if d := time.Duration(k.lastDur.Load()); d != 5*time.Second {
		t.Errorf("poll wait = %v, want the service's 5s", d)
	}
	if fetches.Load() != 1 {
		t.Fatalf("fetches = %d before the interval elapsed", fetches.Load())
	}
	c.SetPollingEnabled(false, true)
	k.release()
	time.Sleep(30 * time.Millisecond)
	if fetches.Load() != 1 {
		t.Fatalf("fetches = %d after hiding", fetches.Load())
	}
	s := c.State()
	if s.Reaction == nil || *s.Reaction.Count != 20 || s.Delta.Amount != 0 || s.Loading {
		t.Errorf("state = %+v", s)
	}
}

func TestOpeningShowsLoadingUntilFetch(t *testing.T) {
	var started atomic.Int32
	g := newGate()
	defer g.open()
	c, _ := newController(func(ctx context.Context) (ReactionSnapshot, error) {
		started.Add(1)
		g.wait(ctx)
		return snapshotWith(20, 5000), nil
	}, nil)
	defer c.Close()

	c.SetPollingEnabled(true, true)
	waitUntil(t, func() bool { return started.Load() >= 1 })
	if s := c.State(); !s.Loading || s.Fresh || s.Reaction != nil {
		t.Fatalf("while loading: %+v", s)
	}
	g.open()
	waitUntil(t, func() bool { return !c.State().Loading })
	if s := c.State(); !s.Fresh || *s.Reaction.Count != 20 {
		t.Errorf("after fetch: %+v", s)
	}
}

func counts(values ...int) func(context.Context) (ReactionSnapshot, error) {
	var mu sync.Mutex
	return func(context.Context) (ReactionSnapshot, error) {
		mu.Lock()
		defer mu.Unlock()
		v := values[0]
		if len(values) > 1 {
			values = values[1:]
		}
		return snapshotWith(v, 1000), nil
	}
}

func TestReopenDoesNotAnnounceCatchUpDelta(t *testing.T) {
	c, _ := newController(counts(10, 50), nil)
	defer c.Close()
	c.SetPollingEnabled(true, true)
	waitUntil(t, countIs(c, 10))
	if c.State().Delta.Amount != 0 {
		t.Fatal("delta on first fetch")
	}
	c.SetPollingEnabled(false, true)
	c.SetPollingEnabled(true, true)
	if !c.State().Loading {
		t.Error("reopen is not loading")
	}
	waitUntil(t, countIs(c, 50))
	waitUntil(t, func() bool { return !c.State().Loading })
	if c.State().Delta.Amount != 0 {
		t.Errorf("catch-up delta = %d", c.State().Delta.Amount)
	}
}

func TestLivePollAnnouncesIncrease(t *testing.T) {
	c, k := newController(counts(10, 13), nil)
	defer c.Close()
	c.SetPollingEnabled(true, true)
	waitUntil(t, countIs(c, 10))
	if c.State().Delta.Amount != 0 {
		t.Fatal("delta on first fetch")
	}
	waitUntil(t, func() bool { return k.sleeps.Load() >= 1 })
	k.release()
	waitUntil(t, countIs(c, 13))
	if c.State().Delta.Amount != 3 || c.State().Delta.ID == 0 {
		t.Errorf("delta = %+v", c.State().Delta)
	}
}

func TestClickIgnoredWhileFirstFetchInFlight(t *testing.T) {
	var fetches, clicks atomic.Int32
	g := newGate()
	defer g.open()
	c, _ := newController(func(ctx context.Context) (ReactionSnapshot, error) {
		if fetches.Add(1) == 1 {
			return snapshotWith(10, 1000), nil
		}
		g.wait(ctx)
		return snapshotWith(50, 1000), nil
	}, func(context.Context) (ReactionPostResult, error) {
		clicks.Add(1)
		s := snapshotWith(11, 1000)
		return ReactionPostResult{OK: true, Data: &s}, nil
	})
	defer c.Close()

	c.SetPollingEnabled(true, true)
	waitUntil(t, countIs(c, 10))
	c.SetPollingEnabled(false, true)
	c.SetPollingEnabled(true, true)
	waitUntil(t, func() bool { return fetches.Load() >= 2 })
	if s := c.State(); !s.Loading || *s.Reaction.Count != 10 {
		t.Fatalf("state = %+v", s)
	}
	if c.Click() {
		t.Error("click accepted while loading")
	}
	if clicks.Load() != 0 || *c.State().Reaction.Count != 10 {
		t.Error("ignored click had an effect")
	}
	g.open()
	waitUntil(t, countIs(c, 50))
	if c.State().Delta.Amount != 0 {
		t.Errorf("delta = %+v", c.State().Delta)
	}
}

func TestClickFloatsImmediately(t *testing.T) {
	g := newGate()
	defer g.open()
	c, _ := newController(func(context.Context) (ReactionSnapshot, error) { return snapshotWith(10, 1000), nil },
		func(ctx context.Context) (ReactionPostResult, error) {
			g.wait(ctx)
			s := snapshotWith(11, 1000)
			return ReactionPostResult{OK: true, Data: &s}, nil
		})
	defer c.Close()
	c.SetPollingEnabled(true, true)
	waitUntil(t, countIs(c, 10))
	c.SetPollingEnabled(false, true)
	if !c.Click() {
		t.Fatal("click rejected")
	}
	if s := c.State(); *s.Reaction.Count != 11 || s.Delta.Amount != 1 || !s.Busy {
		t.Fatalf("optimistic state = %+v", s)
	}
	g.open()
	waitUntil(t, func() bool { return !c.State().Busy })
	if s := c.State(); *s.Reaction.Count != 11 || s.Delta.Amount != 1 {
		t.Errorf("settled state = %+v", s)
	}
}

func TestClickUpdatesCount(t *testing.T) {
	two, one := 2, 1
	fetch := reactionSnap("abc", 10, &two, 5, PolarityNo)
	fetch.PollMs = 1000
	posted := reactionSnap("abc", 11, &one, 5, PolarityNo)
	c, _ := newController(func(context.Context) (ReactionSnapshot, error) { return fetch, nil },
		func(context.Context) (ReactionPostResult, error) {
			return ReactionPostResult{OK: true, Data: &posted}, nil
		})
	defer c.Close()
	c.SetPollingEnabled(true, true)
	waitUntil(t, countIs(c, 10))
	c.SetPollingEnabled(false, true)
	c.Click()
	if *c.State().Reaction.Count != 11 {
		t.Fatal("not optimistic")
	}
	waitUntil(t, func() bool { return !c.State().Busy })
	s := c.State()
	if *s.Reaction.Count != 11 || s.Delta.Amount != 1 || s.Reaction.Remaining == nil || *s.Reaction.Remaining != 1 {
		t.Errorf("state = %+v", s)
	}
}

func TestDailyLimitKeepsPreviousCount(t *testing.T) {
	one, zero := 1, 0
	fetch := reactionSnap("abc", 12, &one, 2, PolarityNo)
	capped := reactionSnap("abc", 12, &zero, 2, PolarityNo)
	e := "daily_limit"
	c, _ := newController(func(context.Context) (ReactionSnapshot, error) { return fetch, nil },
		func(context.Context) (ReactionPostResult, error) {
			return ReactionPostResult{OK: false, Error: &e, Data: &capped}, nil
		})
	defer c.Close()
	c.SetPollingEnabled(true, true)
	waitUntil(t, countIs(c, 12))
	c.SetPollingEnabled(false, true)
	c.Click()
	waitUntil(t, func() bool { return !c.State().Busy })
	s := c.State()
	if *s.Reaction.Count != 12 || !s.Reaction.IsExhausted() || s.Delta.Amount != 0 {
		t.Errorf("state = %+v", s)
	}
	if c.Click() {
		t.Error("click accepted when exhausted")
	}
}

func TestDisabledFetchHidesButtonAndStopsPolling(t *testing.T) {
	var fetches atomic.Int32
	c, k := newController(func(context.Context) (ReactionSnapshot, error) {
		fetches.Add(1)
		return ReactionSnapshot{}, ErrReactionDisabled
	}, nil)
	defer c.Close()
	c.SetPollingEnabled(true, true)
	waitUntil(t, func() bool { return fetches.Load() >= 1 })
	time.Sleep(20 * time.Millisecond)
	k.release()
	k.release()
	time.Sleep(30 * time.Millisecond)
	if s := c.State(); s.Reaction != nil || s.Loading || fetches.Load() != 1 {
		t.Fatalf("state = %+v fetches = %d", s, fetches.Load())
	}
	// Closing and reopening the panel retries.
	c.SetPollingEnabled(false, true)
	c.SetPollingEnabled(true, true)
	waitUntil(t, func() bool { return fetches.Load() >= 2 })
}

func TestTransientFetchErrorKeepsLastCount(t *testing.T) {
	var n atomic.Int32
	c, k := newController(func(context.Context) (ReactionSnapshot, error) {
		if n.Add(1) == 1 {
			return snapshotWith(10, 1000), nil
		}
		return ReactionSnapshot{}, ErrReactionUnavailable
	}, nil)
	defer c.Close()
	c.SetPollingEnabled(true, true)
	waitUntil(t, countIs(c, 10))
	waitUntil(t, func() bool { return k.sleeps.Load() >= 1 })
	k.release()
	waitUntil(t, func() bool { return n.Load() >= 2 })
	waitUntil(t, func() bool { return k.sleeps.Load() >= 2 })
	if s := c.State(); s.Reaction == nil || *s.Reaction.Count != 10 || s.Loading || !s.Fresh {
		t.Errorf("state = %+v", s)
	}
}

func TestDisallowedSectionStopsPolling(t *testing.T) {
	var fetches atomic.Int32
	c, _ := newController(func(context.Context) (ReactionSnapshot, error) { fetches.Add(1); return snapshotWith(1, 1000), nil }, nil)
	defer c.Close()
	c.SetPollingEnabled(true, false)
	time.Sleep(30 * time.Millisecond)
	if fetches.Load() != 0 {
		t.Fatal("fetched while not allowed")
	}
	c.SetPollingEnabled(true, true)
	waitUntil(t, func() bool { return fetches.Load() == 1 })
}
