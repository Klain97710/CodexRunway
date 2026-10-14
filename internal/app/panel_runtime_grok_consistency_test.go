package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/grok"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/runway"
	"github.com/licoy/quosparr/internal/store"
	"github.com/licoy/quosparr/internal/usage"
)

func TestPanelRuntimeGrokRangeChangeDoesNotRelabelOldCost(t *testing.T) {
	detailMainQueue(t)
	p := quietLivePreferences()
	p.Language = store.LanguageEnglish
	p.SelectedProvider = store.ProviderGrok
	p.ShowsCostSummary, p.ShowsTokenUsageHeatmap, p.ShowsRecentSessions = true, true, true
	home := t.TempDir()
	dir := filepath.Join(home, "sessions", "synthetic-range")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"summary.json":  `{"info":{"id":"one","cwd":"/synthetic/project"},"current_model_id":"grok-4.5"}`,
		"updates.jsonl": `{"params":{"update":{"sessionUpdate":"turn_completed","usage":{"inputTokens":100,"outputTokens":10,"modelUsage":{"grok-4.5":{"inputTokens":100,"outputTokens":10}}}}}}`,
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	mod := grok.NewModule(grok.NewStore(home, filepath.Join(t.TempDir(), "runway")))
	g := newGrokBridge(mod, p, nil)
	if err := g.scanLocal(context.Background()); err != nil {
		t.Fatal(err)
	}
	g.reload(false)
	before := g.State(time.Now())
	if before.Cost == nil || before.Cost.Totals.TotalTokens != 110 {
		t.Fatal("temporary Grok fixture did not scan")
	}
	svc := runway.New(runway.Options{Preferences: p, Grok: g})
	defer svc.Close()
	svc.ReloadAccounts()
	rt := &liveApp{svc: svc, grok: g, reset: newResetBridge(nil, p, nil)}
	a, tt := newPanel(t, func(prefs *store.Preferences) { *prefs = p }, svc.Snapshot(), PanelActions{})
	a.settings.OnChange(func() { rt.pushPreferences(a.settings) })
	g.refreshMu.Lock()
	var released bool
	defer func() {
		if !released {
			g.refreshMu.Unlock()
		}
		rt.closePreferences()
	}()
	a.settings.SetAPICostRange(store.CostRangeThisMonth)
	rt.preferenceWork.idle.Wait()
	pending := svc.Snapshot()
	a.snapMu.Lock()
	a.snap = pending
	a.snapMu.Unlock()
	tt.Frame()
	if pending.Grok.Cost != nil {
		t.Fatalf("new range retained old window %s", pending.Grok.Cost.Window.Start)
	}
	if len(pending.Grok.Heatmap.Local) == 0 || len(pending.Grok.RecentSessions) == 0 {
		t.Fatal("range change cleared unrelated local data")
	}
	if !tt.HasText(a.L().Text(l10n.KeyNotLoaded)) {
		t.Fatal("cost did not show the waiting state for the new range")
	}
	g.refreshMu.Unlock()
	released = true
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := svc.WaitIdle(ctx); err != nil {
		t.Fatal(err)
	}
	fresh := svc.Snapshot()
	want := usage.ThisMonth(time.Now(), time.Local).Window.Start
	if fresh.Grok.Cost == nil || !fresh.Grok.Cost.Window.Start.Equal(want) {
		t.Fatal("new requested Grok range did not arrive")
	}
	a.snapMu.Lock()
	a.snap = fresh
	a.snapMu.Unlock()
	tt.Frame()
	a.settings.Flush()
}
