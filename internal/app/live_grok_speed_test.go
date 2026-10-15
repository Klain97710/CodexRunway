package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/codex"
	"github.com/licoy/quosparr/internal/grok"
	"github.com/licoy/quosparr/internal/personal"
	"github.com/licoy/quosparr/internal/runway"
	"github.com/licoy/quosparr/internal/speed"
	"github.com/licoy/quosparr/internal/store"
)

// writeGrokSpeedSession stores one synthetic session whose logs are dated now:
// two model calls of one turn (1 s and 3 s to the first token) and 400 output
// tokens over a 14 s API time.
func writeGrokSpeedSession(t *testing.T, home string, now time.Time) {
	t.Helper()
	dir := filepath.Join(home, "sessions", "%2Fsynthetic", "speed-one")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	ts := func(sec int) string {
		return now.Add(time.Duration(sec-30) * time.Second).UTC().Format("2006-01-02T15:04:05.000Z")
	}
	events := fmt.Sprintf(`{"ts":%q,"type":"turn_started","model_id":"grok-synthetic"}`+"\n"+
		`{"ts":%q,"type":"phase_changed","phase":"waiting_for_model"}`+"\n"+
		`{"ts":%q,"type":"first_token"}`+"\n"+
		`{"ts":%q,"type":"phase_changed","phase":"waiting_for_model"}`+"\n"+
		`{"ts":%q,"type":"first_token"}`+"\n"+
		`{"ts":%q,"type":"turn_ended"}`+"\n",
		ts(0), ts(1), ts(2), ts(10), ts(13), ts(20))
	updates := `{"params":{"update":{"sessionUpdate":"turn_completed","usage":{"inputTokens":100,"outputTokens":400,"apiDurationMs":14000,"modelCalls":2}}}}` + "\n"
	summary := fmt.Sprintf(`{"info":{"id":"speed-one","cwd":"/synthetic"},"generated_title":"Synthetic","current_model_id":"grok-synthetic","reasoning_effort":"high","last_active_at":%q}`,
		now.UTC().Format(time.RFC3339))
	for name, content := range map[string]string{"summary.json": summary, "events.jsonl": events, "updates.jsonl": updates} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func grokSpeedBridge(t *testing.T, shows bool) *grokBridge {
	t.Helper()
	home := t.TempDir()
	writeGrokSpeedSession(t, home, time.Now())
	p := store.DefaultPreferences()
	p.SelectedProvider = store.ProviderGrok
	p.ShowsSpeedAnalysis = shows
	st := grok.NewStore(home, filepath.Join(t.TempDir(), "runway"))
	return newGrokBridge(grok.NewModule(st), p, nil)
}

func TestGrokLocalScanPublishesSpeedReport(t *testing.T) {
	b := grokSpeedBridge(t, true)
	if err := b.scanLocal(context.Background()); err != nil {
		t.Fatal(err)
	}
	b.reload(false)
	got := b.State(time.Now()).Speed
	if got == nil || len(got.Groups) != 1 || len(got.Sessions) != 1 {
		t.Fatalf("speed report = %+v", got)
	}
	g := got.Groups[0]
	if g.Model != "grok-synthetic" || g.Effort != "high" || g.Calls != 2 || g.TTFTP50 != time.Second || g.OutputTokens != 400 {
		t.Fatalf("group = %+v", g)
	}
	if !g.HasTPS() || g.TPS() != 400/(14.0-2.0*2) {
		t.Fatalf("tps = %v over %v", g.TPS(), g.Decode)
	}
}

func TestGrokLocalScanSkipsSpeedWhenHidden(t *testing.T) {
	b := grokSpeedBridge(t, false)
	if err := b.scanLocal(context.Background()); err != nil {
		t.Fatal(err)
	}
	b.reload(false)
	if b.State(time.Now()).Speed != nil {
		t.Fatal("a hidden module was scanned")
	}
}

func TestGrokSpeedFollowsThePreference(t *testing.T) {
	b := grokSpeedBridge(t, true)
	if err := b.scanLocal(context.Background()); err != nil {
		t.Fatal(err)
	}
	b.reload(false)
	off := b.prefs.Clone()
	off.ShowsSpeedAnalysis = false
	gen := b.generation
	b.setPreferences(off, nil)
	b.reload(false)
	if b.State(time.Now()).Speed != nil || b.generation == gen {
		t.Fatalf("hiding kept the report (generation %d -> %d)", gen, b.generation)
	}
	on := off.Clone()
	on.ShowsSpeedAnalysis = true
	b.setPreferences(on, nil)
	if err := b.scanLocal(context.Background()); err != nil {
		t.Fatal(err)
	}
	b.reload(false)
	if b.State(time.Now()).Speed == nil {
		t.Fatal("showing it again did not load the report")
	}
}

func TestGrokSpeedScanFailureIsNoReportNotAnError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p := store.DefaultPreferences()
	home := t.TempDir()
	writeGrokSpeedSession(t, home, time.Now())
	if r := scanGrokSpeed(ctx, grok.SessionScanner{GrokHome: home}, p, time.Now()); r != nil {
		t.Fatalf("report on a cancelled scan: %+v", r)
	}
	scanner := grok.SessionScanner{GrokHome: t.TempDir()}
	p.ShowsSpeedAnalysis = false
	if r := scanGrokSpeed(context.Background(), scanner, p, time.Now()); r != nil {
		t.Fatalf("report while hidden: %+v", r)
	}
	// A home without sessions is an empty report, not a failure.
	p.ShowsSpeedAnalysis = true
	if r := scanGrokSpeed(context.Background(), scanner, p, time.Now()); r == nil || !r.Empty() {
		t.Fatalf("empty home report = %+v", r)
	}
}

// countingGrok is a Grok provider that only counts quota refreshes and
// local scans.
type countingGrok struct{ refreshes, locals atomic.Int32 }

func (g *countingGrok) State(time.Time) runway.GrokState   { return runway.GrokState{} }
func (g *countingGrok) Refresh(context.Context) error      { g.refreshes.Add(1); return nil }
func (g *countingGrok) RefreshLocal(context.Context) error { g.locals.Add(1); return nil }
func (g *countingGrok) SetOnChange(func())                 {}

func speedService(t *testing.T, provider store.Provider, grokProvider runway.GrokProvider, scans *atomic.Int32) *runway.Service {
	t.Helper()
	p := store.DefaultPreferences()
	p.SelectedProvider = provider
	s := runway.New(runway.Options{Features: personal.Features{Grok: true, Claude: true},
		Preferences: p,
		Grok:        grokProvider,
		Sources: runway.Sources{
			// The full refresh also covers Codex; no one is signed in.
			LoadValidAuth: func(context.Context, bool, *codex.Auth) (codex.Auth, error) {
				return codex.Auth{}, &runway.AuthLoadError{}
			},
			ScanSpeed: func(context.Context, time.Time) (speed.Report, error) {
				scans.Add(1)
				return speed.Report{}, nil
			},
		},
		Location: time.UTC,
	})
	t.Cleanup(s.Close)
	return s
}

func waitSpeedCond(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestRefreshSectionSpeedForCodexScansSpeedOnly(t *testing.T) {
	var scans atomic.Int32
	g := &countingGrok{}
	s := speedService(t, store.ProviderCodex, g, &scans)
	refreshSection(s, runway.SectionSpeed)
	waitSpeedCond(t, "the speed scan", func() bool { return scans.Load() == 1 })
	if g.refreshes.Load() != 0 {
		t.Fatalf("grok refreshed %d times for a Codex speed refresh", g.refreshes.Load())
	}
}

func TestRefreshSectionSpeedForGrokRescansGrokOnly(t *testing.T) {
	var scans atomic.Int32
	g := &countingGrok{}
	s := speedService(t, store.ProviderGrok, g, &scans)
	refreshSection(s, runway.SectionSpeed)
	waitSpeedCond(t, "the grok local scan", func() bool { return g.locals.Load() == 1 })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.WaitIdle(ctx); err != nil {
		t.Fatal(err)
	}
	if g.refreshes.Load() != 0 || scans.Load() != 0 {
		t.Fatalf("a local section refreshed the quota %d times and Codex speed %d times", g.refreshes.Load(), scans.Load())
	}
}
