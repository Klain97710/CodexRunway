package codexapi

import "sync"

// Generations numbers refreshes so a result that arrives after its account was
// switched, removed or re-refreshed can be recognized and dropped. Begin hands
// out a generation when a request starts; the consumer applies a result only
// while Valid still holds.
type Generations struct {
	mu      sync.Mutex
	counter uint64
	floor   uint64
	latest  map[string]uint64
}

// Begin starts a new refresh of accountID and returns its generation. It
// supersedes any older, still running refresh of the same account.
func (g *Generations) Begin(accountID string) uint64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.counter++
	if g.latest == nil {
		g.latest = map[string]uint64{}
	}
	g.latest[accountID] = g.counter
	return g.counter
}

// Invalidate discards every in-flight refresh of accountID (switch, removal).
func (g *Generations) Invalidate(accountID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.counter++
	if g.latest != nil {
		delete(g.latest, accountID)
	}
}

// InvalidateAll discards every in-flight refresh (proxy change, shutdown).
func (g *Generations) InvalidateAll() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.counter++
	g.floor = g.counter
	g.latest = nil
}

// Valid reports whether gen is still the newest refresh of accountID.
func (g *Generations) Valid(accountID string, gen uint64) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return gen > g.floor && g.latest[accountID] == gen
}
