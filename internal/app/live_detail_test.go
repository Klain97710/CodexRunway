package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/grok"
	"github.com/licoy/quosparr/internal/store"
	"github.com/licoy/quosparr/internal/usage"
)

func TestGrokDetailQueryScansOnlySelectedRange(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "sessions", "fixture")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"summary.json":  `{"info":{"id":"one","cwd":"/synthetic/project"},"current_model_id":"grok-4.5"}`,
		"updates.jsonl": `{"timestamp":1785541392,"params":{"update":{"sessionUpdate":"turn_completed","usage":{"inputTokens":100,"outputTokens":10,"modelUsage":{"grok-4.5":{"inputTokens":100,"outputTokens":10}}}}}}`,
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	mod := grok.NewModule(grok.NewStore(home, filepath.Join(t.TempDir(), "runway")))
	rt := &liveApp{grokMod: mod, grok: newGrokBridge(mod, store.DefaultPreferences(), nil)}
	at := time.Unix(1785541392, 0)
	for _, test := range []struct {
		delta int
		want  int64
	}{{0, 110}, {1, 0}} {
		day := at.AddDate(0, 0, test.delta)
		r, ok := usage.CustomRange(day, day, time.UTC)
		if !ok {
			t.Fatal("custom fixture range invalid")
		}
		sum, err := rt.queryDetailCost(context.Background(), store.ProviderGrok, CostQuery{Custom: &r})
		if err != nil {
			t.Fatal(err)
		}
		if sum.Totals.TotalTokens != test.want || !sum.Window.Start.Equal(r.Window.Start) || !sum.Window.End.Equal(r.Window.End) {
			t.Fatalf("Grok scanner ignored range: tokens=%d window=%+v", sum.Totals.TotalTokens, sum.Window)
		}
	}
	if _, err := rt.queryDetailCost(context.Background(), store.ProviderGrok, CostQuery{Range: store.CostRangeCurrent}); !errors.Is(err, errGrokCycleUnavailable) {
		t.Fatalf("unknown cycle invented a range: %v", err)
	}
}
