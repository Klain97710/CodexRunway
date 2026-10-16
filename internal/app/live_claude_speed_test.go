package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/claude"
	"github.com/licoy/quosparr/internal/store"
)

func claudeSpeedBridge(t *testing.T) (*claudeBridge, string) {
	t.Helper()
	home := t.TempDir()
	config := filepath.Join(home, "cfg")
	path := filepath.Join(config, "projects", "demo", "speed.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	requestAt := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	data := []byte(fmt.Sprintf("{\"type\":\"user\",\"sessionId\":\"s\",\"timestamp\":%q}\n"+
		"{\"type\":\"assistant\",\"sessionId\":\"s\",\"timestamp\":%q,\"message\":{\"id\":\"m\",\"model\":\"claude-opus-4-8\",\"usage\":{\"output_tokens\":120}}}\n",
		requestAt.Format(time.RFC3339), requestAt.Add(12*time.Second).Format(time.RFC3339)))
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	p := store.DefaultPreferences()
	p.SelectedProvider = store.ProviderClaude
	p.ShowsSpeedAnalysis = true
	st := claude.NewStore(config, filepath.Join(home, "runway"), home, "", "")
	return newClaudeBridge(claude.NewModule(st), p, nil), path
}

func TestClaudeSpeedReadFailureKeepsReportAndSurfacesError(t *testing.T) {
	b, path := claudeSpeedBridge(t)
	if err := b.scanLocal(context.Background()); err != nil {
		t.Fatal(err)
	}
	b.reload(false)
	before := b.State(time.Now())
	if before.Speed == nil || before.Speed.Empty() {
		t.Fatalf("initial speed = %+v", before.Speed)
	}
	changedAt := time.Now().Add(time.Second)
	if err := os.Chtimes(path, changedAt, changedAt); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(path, 0o600)
	if f, err := os.Open(path); err == nil {
		f.Close()
		t.Skip("filesystem does not enforce unreadable file permissions")
	}
	if err := b.scanLocal(context.Background()); err == nil {
		t.Fatal("session read failure was swallowed")
	}
	b.reload(false)
	failed := b.State(time.Now())
	if failed.Speed != before.Speed || failed.SpeedError == "" {
		t.Fatalf("speed=%p want=%p error=%q", failed.Speed, before.Speed, failed.SpeedError)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := b.scanLocal(context.Background()); err != nil {
		t.Fatal(err)
	}
	b.reload(false)
	if got := b.State(time.Now()); got.Speed == nil || got.SpeedError != "" {
		t.Fatalf("recovered speed=%v error=%q", got.Speed != nil, got.SpeedError)
	}
}

func TestClaudeCancelledLocalScanKeepsSpeedState(t *testing.T) {
	b, _ := claudeSpeedBridge(t)
	if err := b.scanLocal(context.Background()); err != nil {
		t.Fatal(err)
	}
	before, beforeErr := b.speed, b.speedErr
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := b.scanLocal(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel error = %v", err)
	}
	if b.speed != before || b.speedErr != beforeErr {
		t.Fatal("cancelled scan changed speed state")
	}
}
