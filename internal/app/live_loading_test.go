package app

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestLocalScanPublishesLoadingUntilResult(t *testing.T) {
	g := grokSpeedBridge(t, true)
	c, _ := claudeSpeedBridge(t)
	for _, tc := range []struct {
		name     string
		refresh  func(context.Context) error
		onChange func(func())
		state    func() (bool, bool)
	}{
		{"grok", g.RefreshLocal, g.SetOnChange, func() (bool, bool) { s := g.State(time.Now()); return s.Refreshing, s.Speed != nil }},
		{"claude", c.RefreshLocal, c.SetOnChange, func() (bool, bool) { s := c.State(time.Now()); return s.Refreshing, s.Speed != nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			started := false
			tc.onChange(func() {
				busy, ready := tc.state()
				if busy && !ready {
					started = true
				}
			})
			if err := tc.refresh(context.Background()); err != nil {
				t.Fatal(err)
			}
			busy, ready := tc.state()
			if !started || busy || !ready {
				t.Fatalf("loading published=%v, final busy=%v ready=%v", started, busy, ready)
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if err := tc.refresh(ctx); !errors.Is(err, context.Canceled) {
				t.Fatalf("cancelled scan returned %v", err)
			}
			if busy, _ := tc.state(); busy {
				t.Fatal("cancelled scan left the loading indicator running")
			}
		})
	}
}

func TestOverlappingRefreshesKeepLoadingUntilBothFinish(t *testing.T) {
	g := grokSpeedBridge(t, true)
	c, _ := claudeSpeedBridge(t)
	for _, tc := range []struct {
		name         string
		local        func(context.Context) error
		quota        func(context.Context) error
		lock, unlock func()
		busy         func() bool
	}{
		{"grok", g.RefreshLocal, g.Refresh, g.refreshMu.Lock, g.refreshMu.Unlock, func() bool { return g.State(time.Now()).Refreshing }},
		{"claude", c.RefreshLocal, func(ctx context.Context) error { return c.Refresh(ctx, false) }, c.refreshMu.Lock, c.refreshMu.Unlock, func() bool { return c.State(time.Now()).Refreshing }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.lock()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- tc.quota(ctx) }()
			released := false
			defer func() {
				if !released {
					cancel()
					tc.unlock()
					<-done
				}
			}()
			waitFor(t, tc.busy)
			if err := tc.local(context.Background()); err != nil {
				t.Fatal(err)
			}
			if !tc.busy() {
				t.Fatal("local completion stopped the still-pending quota indicator")
			}
			cancel()
			tc.unlock()
			released = true
			if err := <-done; !errors.Is(err, context.Canceled) {
				t.Fatalf("quota cancellation = %v", err)
			}
			if tc.busy() {
				t.Fatal("loading remained after both operations completed")
			}
		})
	}
}
