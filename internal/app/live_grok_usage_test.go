package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/export"
	"github.com/licoy/quosparr/internal/grok"
	"github.com/licoy/quosparr/internal/store"
)

func TestGrokLocalScanPublishesTypedPanelAndWidgetData(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "sessions", "synthetic")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	summary := `{"info":{"id":"one","cwd":"/synthetic/project"},"generated_title":"Synthetic session","current_model_id":"grok-4.5","num_chat_messages":2}`
	if err := os.WriteFile(filepath.Join(dir, "summary.json"), []byte(summary), 0600); err != nil {
		t.Fatal(err)
	}
	turn := `{"params":{"update":{"sessionUpdate":"turn_completed","usage":{"inputTokens":100,"outputTokens":10,"modelUsage":{"grok-4.5":{"inputTokens":100,"outputTokens":10}}}}}}`
	if err := os.WriteFile(filepath.Join(dir, "updates.jsonl"), []byte(turn), 0600); err != nil {
		t.Fatal(err)
	}
	p := store.DefaultPreferences()
	p.SelectedProvider = store.ProviderGrok
	p.ShowsRecentSessions = true
	st := grok.NewStore(home, filepath.Join(t.TempDir(), "runway"))
	b := newGrokBridge(grok.NewModule(st), p, nil)
	if err := b.scanLocal(context.Background()); err != nil {
		t.Fatal(err)
	}
	b.reload(false)
	got := b.State(time.Now())
	if got.Cost == nil || got.Cost.Totals.TotalTokens != 110 || len(got.Heatmap.Local) != 1 || len(got.RecentSessions) != 1 {
		t.Fatalf("Grok local data missing: cost=%v heat=%d recent=%d", got.Cost, len(got.Heatmap.Local), len(got.RecentSessions))
	}
	if got.Widget == nil || len(got.Widget.DailyTokens) != 1 || got.Widget.APIEquivalentCostUSD == nil {
		t.Fatal("widget local data missing")
	}
}

func TestInitialWidgetRequirementsFollowSupportedLocalStorage(t *testing.T) {
	s := NewSettings(nil, System{Widgets: true}, nil)
	if got := initialWidgetRequirements(s); got != export.RequireAll {
		t.Fatalf("requirements = %d", got)
	}
	s.sys.Widgets = false
	if initialWidgetRequirements(s) != 0 {
		t.Fatal("unsupported platform requested widgets")
	}
}
