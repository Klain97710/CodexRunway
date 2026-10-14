package app

import (
	"sync"

	"github.com/licoy/quosparr/internal/runway"
)

// snapshotDispatch keeps one UI hop outstanding; RunOnMain can wait on the
// event loop, so the service's publisher must never call it synchronously.
type snapshotDispatch struct {
	mu              sync.Mutex
	latest          runway.Snapshot
	pending, closed bool
	post            func(func())
	apply           func(runway.Snapshot)
}

func (d *snapshotDispatch) publish(s runway.Snapshot) {
	d.mu.Lock()
	d.latest = s
	if d.closed || d.pending {
		d.mu.Unlock()
		return
	}
	d.pending = true
	d.mu.Unlock()
	go d.drain()
}

func (d *snapshotDispatch) drain() {
	for {
		var applied uint64
		d.post(func() {
			d.mu.Lock()
			latest, closed := d.latest, d.closed
			d.mu.Unlock()
			applied = latest.Seq
			if !closed {
				d.apply(latest)
			}
		})
		d.mu.Lock()
		if d.closed || d.latest.Seq == applied {
			d.pending = false
			d.mu.Unlock()
			return
		}
		d.mu.Unlock()
	}
}

func (d *snapshotDispatch) close() {
	d.mu.Lock()
	d.closed = true
	d.mu.Unlock()
}
