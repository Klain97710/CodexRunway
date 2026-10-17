package app

import (
	"slices"
	"strings"
	"time"

	"github.com/licoy/quosparr/internal/store"
)

// foregroundPoll is how often the app in front is looked up while the
// floating window follows it.
const foregroundPoll = 250 * time.Millisecond

// foregroundKind is what is in front of the other windows.
type foregroundKind int

const (
	// fgPassing is no app for a moment, the system's own interface (the
	// Dock, the taskbar, the app switcher) or Quosparr itself: the window
	// stays as it is.
	fgPassing foregroundKind = iota
	fgDesktop
	fgApp
)

// foreground is what is in front. keys identify its app, lowercased, as
// floatingAppKeys identifies an app of the list.
type foreground struct {
	kind foregroundKind
	keys []string
}

// floatingAppMatcher decides from the app in front whether the window shows.
type floatingAppMatcher struct {
	filter store.FloatingAppFilter
	keys   map[string]bool
}

// newFloatingAppMatcher looks up the keys of the listed apps, which may read
// the disk: call it off the main thread.
func newFloatingAppMatcher(f store.FloatingWindow) floatingAppMatcher {
	m := floatingAppMatcher{filter: f.AppFilter, keys: map[string]bool{}}
	for _, a := range f.Apps {
		for _, k := range floatingAppKeys(a) {
			m.keys[k] = true
		}
	}
	return m
}

// visible reports whether the window shows with fg in front; known is false
// while fg leaves it as it is.
func (m floatingAppMatcher) visible(fg foreground) (visible, known bool) {
	switch {
	case m.filter == store.FloatingAppFilterOff, fg.kind == fgDesktop:
		return true, true
	case fg.kind == fgPassing:
		return false, false
	}
	listed := slices.ContainsFunc(fg.keys, func(k string) bool { return m.keys[k] })
	return listed == (m.filter == store.FloatingAppFilterAllow), true
}

// floatingFollow steadies the window: it shows at once for an app, but hides,
// or shows for the desktop, only once two looks agree, so that switching
// through other apps or past the desktop does not make it blink.
type floatingFollow struct {
	started, visible bool
	candidate        bool
	seen             int
}

// step takes a look; it reports whether the window changes.
func (d *floatingFollow) step(visible, immediate bool) bool {
	if !d.started {
		d.started, d.visible = true, visible
		return true
	}
	if visible == d.visible {
		d.seen = 0
		return false
	}
	if !immediate {
		if d.seen == 0 || d.candidate != visible {
			d.candidate, d.seen = visible, 1
			return false
		}
	}
	d.visible, d.seen = visible, 0
	return true
}

// watchForeground reports on the main thread whether the window shows by
// the app in front, at once and then as it changes, until the returned func
// is called.
func watchForeground(f store.FloatingWindow, report func(visible bool)) func() {
	stop := make(chan struct{})
	go func() {
		probe, done := newForegroundProbe()
		defer done()
		m := newFloatingAppMatcher(f)
		t := time.NewTicker(foregroundPoll)
		defer t.Stop()
		var follow floatingFollow
		for {
			fg := probe()
			if v, known := m.visible(fg); known && follow.step(v, v && fg.kind == fgApp) {
				runOnMain(func() {
					select {
					case <-stop:
					default:
						report(v)
					}
				})
			}
			select {
			case <-stop:
				return
			case <-t.C:
			}
		}
	}()
	return func() { close(stop) }
}

// waitOnMain runs fn on the main thread and waits for it, from another
// goroutine.
func waitOnMain(fn func()) {
	done := make(chan struct{})
	runOnMain(func() { defer close(done); fn() })
	<-done
}

// lowerKeys lowercases the keys that are not empty.
func lowerKeys(keys ...string) []string {
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		if k != "" {
			out = append(out, strings.ToLower(k))
		}
	}
	return out
}
