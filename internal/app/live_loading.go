package app

// Count queued and active refreshes so overlapping quota and local scans keep
// the indicator visible until both finish; callbacks run outside the lock.
func (g *grokBridge) changeRefreshing(delta int) {
	g.mu.Lock()
	g.refreshes += delta
	g.state.Refreshing = g.refreshes > 0
	fn := g.onChange
	g.mu.Unlock()
	if fn != nil {
		fn()
	}
}

func (g *claudeBridge) changeRefreshing(delta int) {
	g.mu.Lock()
	g.refreshes += delta
	g.state.Refreshing = g.refreshes > 0
	fn := g.onChange
	g.mu.Unlock()
	if fn != nil {
		fn()
	}
}
