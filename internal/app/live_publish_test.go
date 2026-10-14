package app

import (
	"context"
	"errors"
	"github.com/licoy/quosparr/internal/codex"
	"github.com/licoy/quosparr/internal/personal"
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/runway"
	"github.com/licoy/quosparr/internal/store"
)

func TestServiceCloseDoesNotWaitOnBlockedUIDispatch(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	delivered := make(chan uint64, 1)
	d := &snapshotDispatch{post: func(fn func()) { close(entered); <-release; fn() }, apply: func(s runway.Snapshot) { delivered <- s.Seq }}
	svc := runway.New(runway.Options{Features: personal.Features{Grok: true}, Preferences: store.DefaultPreferences(), OnSnapshot: d.publish, Sources: runway.Sources{LoadValidAuth: func(context.Context, bool, *codex.Auth) (codex.Auth, error) {
		return codex.Auth{}, errors.New("synthetic auth unavailable")
	}}})
	svc.Start(context.Background())
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("UI dispatch did not start")
	}
	for n := uint64(2); n < 1000; n++ {
		d.publish(runway.Snapshot{Seq: n})
	}
	d.close()
	closed := make(chan struct{})
	go func() { svc.Close(); close(closed) }()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("service close waited for UI")
	}
	close(release)
	select {
	case <-delivered:
		t.Fatal("closed dispatcher applied stale snapshot")
	case <-time.After(20 * time.Millisecond):
	}
	if err := svc.WaitIdle(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestSnapshotDispatchConflatesPendingSnapshots(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	delivered := make(chan uint64, 1)
	d := &snapshotDispatch{post: func(fn func()) { close(entered); <-release; fn() }, apply: func(s runway.Snapshot) { delivered <- s.Seq }}
	d.publish(runway.Snapshot{Seq: 1})
	<-entered
	d.publish(runway.Snapshot{Seq: 2})
	d.publish(runway.Snapshot{Seq: 3})
	close(release)
	select {
	case seq := <-delivered:
		if seq != 3 {
			t.Fatalf("delivered %d", seq)
		}
	case <-time.After(time.Second):
		t.Fatal("no snapshot")
	}
}
