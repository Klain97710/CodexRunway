package app

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"sync"
	"time"

	"github.com/licoy/quosparr/internal/export"
)

type widgetConfiguration struct {
	Kind   export.Kind   `json:"kind"`
	Family export.Family `json:"family"`
}

type widgetNative struct {
	configurations func(context.Context) ([]widgetConfiguration, error)
	reload         func([]string) error
}

type widgetCoordinator struct {
	mu               sync.Mutex
	ctx              context.Context
	cancel           context.CancelFunc
	wg               sync.WaitGroup
	store            export.SnapshotStore
	native           *widgetNative
	unavailable      error
	active           []string
	lastReload       time.Time
	lastSnapshot     *export.Snapshot
	querying, closed bool
	onRequirements   func(export.Requirements)
	onError          func(error)
	interval         func() time.Duration
}

func newWidgetCoordinator(home string, enabled bool) *widgetCoordinator {
	ctx, cancel := context.WithCancel(context.Background())
	w := &widgetCoordinator{ctx: ctx, cancel: cancel, store: export.SnapshotStore{Path: export.LocalSnapshotPath(home)}}
	if enabled {
		for _, kind := range export.Kinds() {
			w.active = append(w.active, string(kind))
		}
		w.native, w.unavailable = loadWidgetNative()
		if w.unavailable != nil {
			log.Println("widgets:", w.unavailable)
		}
	}
	return w
}

func (w *widgetCoordinator) publish(snap export.Snapshot) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return context.Canceled
	}
	interval := time.Duration(0)
	if w.interval != nil {
		interval = w.interval()
	}
	due := w.native != nil && len(w.active) > 0 && (w.lastReload.IsZero() || !snap.GeneratedAt.Before(w.lastReload.Add(interval)))
	if w.lastSnapshot != nil {
		if snap.GeneratedAt.Before(w.lastSnapshot.GeneratedAt) {
			return nil
		}
		previous := *w.lastSnapshot
		previous.GeneratedAt = snap.GeneratedAt
		if reflect.DeepEqual(previous, snap) && !due {
			return nil
		}
	}
	if err := w.store.Save(snap); err != nil {
		return err
	}
	w.lastSnapshot = &snap
	if w.unavailable != nil {
		return w.unavailable
	}
	if w.native == nil || len(w.active) == 0 || !due {
		return nil
	}
	if err := w.native.reload(w.active); err != nil {
		return err
	}
	w.lastReload = snap.GeneratedAt
	return nil
}

// Query after startup, app activation and each full refresh, as the Swift
// host does; failed inspection keeps local mode's initial all-data demand.
func (w *widgetCoordinator) refresh() {
	w.mu.Lock()
	if w.closed || w.querying || w.native == nil {
		w.mu.Unlock()
		return
	}
	w.querying = true
	w.wg.Add(1)
	w.mu.Unlock()
	go func() {
		defer w.wg.Done()
		ctx, cancel := context.WithTimeout(w.ctx, 15*time.Second)
		defer cancel()
		configurations, err := w.native.configurations(ctx)
		w.mu.Lock()
		w.querying = false
		if w.closed {
			w.mu.Unlock()
			return
		}
		onError, onRequirements := w.onError, w.onRequirements
		if err == nil {
			w.active = nil
			seen := map[string]bool{}
			for _, cfg := range configurations {
				if export.RequirementsFor(cfg.Kind, cfg.Family) != 0 && !seen[string(cfg.Kind)] {
					w.active = append(w.active, string(cfg.Kind))
					seen[string(cfg.Kind)] = true
				}
			}
		}
		w.mu.Unlock()
		if err != nil {
			if onError != nil {
				onError(fmt.Errorf("WidgetKit configuration query: %w", err))
			}
			return
		}
		var reqs export.Requirements
		for _, cfg := range configurations {
			reqs |= export.RequirementsFor(cfg.Kind, cfg.Family)
		}
		if onRequirements != nil {
			onRequirements(reqs)
		}
	}()
}

func (w *widgetCoordinator) close() {
	w.mu.Lock()
	w.closed = true
	w.cancel()
	w.mu.Unlock()
	w.wg.Wait()
}

// RefreshWidgetConfigurations rechecks which widgets are installed after
// activation; it does not launch a second query while one is pending.
func RefreshWidgetConfigurations() {
	if rt := currentRuntime(); rt != nil && rt.widgets != nil {
		rt.widgets.refresh()
	}
}
