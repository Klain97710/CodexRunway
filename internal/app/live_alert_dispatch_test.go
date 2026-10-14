package app

import (
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/alerts"
	"github.com/licoy/quosparr/internal/runway"
)

func TestAlertDispatchBatchesOnceAndDropsStoppedEvents(t *testing.T) {
	for _, stop := range []bool{false, true} {
		t.Run(map[bool]string{false: "deliver", true: "stop"}[stop], func(t *testing.T) {
			entered, release := make(chan struct{}), make(chan struct{})
			delivered := make(chan runway.Event, 1)
			d := &alertDispatch{post: func(fn func()) { close(entered); <-release; fn() }, apply: func(e runway.Event) { delivered <- e }}
			d.publish(runway.Event{Kind: runway.EventAlerts, Alerts: []alerts.Alert{{ID: "a"}}})
			<-entered
			d.publish(runway.Event{Kind: runway.EventAlerts, Alerts: []alerts.Alert{{ID: "b"}}})
			if stop {
				d.close()
			}
			close(release)
			select {
			case e := <-delivered:
				if stop {
					t.Fatal("stopped alerts delivered")
				}
				if len(e.Alerts) != 2 || e.Alerts[0].ID != "a" || e.Alerts[1].ID != "b" {
					t.Fatal("alerts dropped or reordered")
				}
			case <-time.After(30 * time.Millisecond):
				if !stop {
					t.Fatal("alerts not delivered")
				}
			}
		})
	}
}
