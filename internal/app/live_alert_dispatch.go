package app

import (
	"sync"

	"github.com/licoy/quosparr/internal/alerts"
	"github.com/licoy/quosparr/internal/runway"
)

// alertDispatch batches alerts behind one UI hop; the service can finish
// while the UI thread is waiting for shutdown, and stopped alerts are dropped.
type alertDispatch struct {
	mu              sync.Mutex
	alerts          []alerts.Alert
	pending, closed bool
	post            func(func())
	apply           func(runway.Event)
}

func (d *alertDispatch) publish(e runway.Event) {
	if e.Kind != runway.EventAlerts || len(e.Alerts) == 0 {
		return
	}
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return
	}
	d.alerts = append(d.alerts, e.Alerts...)
	if d.pending {
		d.mu.Unlock()
		return
	}
	d.pending = true
	d.mu.Unlock()
	go d.post(func() {
		d.mu.Lock()
		batch, closed := d.alerts, d.closed
		d.alerts, d.pending = nil, false
		d.mu.Unlock()
		if !closed {
			d.apply(runway.Event{Kind: runway.EventAlerts, Alerts: batch})
		}
	})
}

func (d *alertDispatch) close() {
	d.mu.Lock()
	d.closed = true
	d.alerts = nil
	d.mu.Unlock()
}
