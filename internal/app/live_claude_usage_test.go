package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/claude"
	"github.com/licoy/quosparr/internal/store"
)

func TestClaudeCostWindowUsesSevenDayReset(t *testing.T) {
	reset := time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	snap := &claude.UsageSnapshot{Windows: []claude.UsageWindow{{
		Kind: claude.WindowSevenDay, ResetsAt: &reset,
	}}}
	got := claudeCostWindow(store.CostRangeCurrent, snap, now)
	if !got.Start.Equal(reset.Add(-7*24*time.Hour)) || !got.End.Equal(now) {
		t.Fatalf("window %v %v", got.Start, got.End)
	}
	fallback := claudeCostWindow(store.CostRangeCurrent, nil, now)
	if !fallback.Start.Equal(now.Add(-7*24*time.Hour)) || !fallback.End.Equal(now) {
		t.Fatalf("fallback %v %v", fallback.Start, fallback.End)
	}
}

func TestClaudeGatewayReadsKeyNamesOnly(t *testing.T) {
	dir := t.TempDir()
	if claudeGatewayConfigured(dir) {
		t.Fatal("missing settings")
	}
	body := []byte(`{"env":{"ANTHROPIC_BASE_URL":"secret-value","OTHER":"x"}}`)
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), body, 0o600); err != nil {
		t.Fatal(err)
	}
	if !claudeGatewayConfigured(dir) {
		t.Fatal("gateway key not seen")
	}
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(`{"env":{"OTHER":"x"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if claudeGatewayConfigured(dir) {
		t.Fatal("unrelated key")
	}
}
