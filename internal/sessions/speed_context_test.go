package sessions

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/speed"
)

func TestRestoreSpeedContextRejectsOversizedLatestMetadata(t *testing.T) {
	data := `{"type":"turn_context","payload":{"model":"old"}}` + "\n" +
		`{"type":"turn_context","payload":{"model":"new","padding":"` +
		strings.Repeat("x", speedMetadataLineLimit) + `"}}` + "\n"
	var c speedCursor
	err := restoreSpeedContext(context.Background(), strings.NewReader(data), int64(len(data)), &c)
	if !errors.Is(err, errSpeedMetadataLineTooLong) {
		t.Fatalf("err = %v, context = %q", err, c.model)
	}
}

func TestRestoreSpeedContextRejectsMalformedLatestContext(t *testing.T) {
	data := `{"type":"turn_context","payload":{"model":"old"}}` + "\n" +
		`{"type":"turn_context",broken}` + "\n"
	var c speedCursor
	err := restoreSpeedContext(context.Background(), strings.NewReader(data), int64(len(data)), &c)
	if !errors.Is(err, errInvalidSpeedMetadata) || strings.Contains(err.Error(), "broken") {
		t.Fatalf("err = %v", err)
	}
}

func TestRestoreSpeedContextSkipsNestedMetadataMarkers(t *testing.T) {
	data := `{"type":"turn_context","payload":{"model":"gpt-test","effort":"high"}}` + "\n" +
		`{"type":"event_msg","payload":{"type":"thread_settings_applied","thread_settings":{"service_tier":"priority"}}}` + "\n" +
		`{"type":"event_msg","payload":{"type":"note","nested":{"type":"turn_context"}}}` + "\n" +
		`{"type":"event_msg","payload":{"type":"note","nested":{"type":"thread_settings_applied"}}}` + "\n"
	var c speedCursor
	if err := restoreSpeedContext(context.Background(), strings.NewReader(data), int64(len(data)), &c); err != nil {
		t.Fatal(err)
	}
	if c.model != "gpt-test" || c.effort != "high" || c.tier != "fast" {
		t.Fatalf("context = %q/%q tier=%q", c.model, c.effort, c.tier)
	}
}

func TestRestoreSpeedContextHonoursCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	data := strings.Repeat("x", speedMetadataBlock*2)
	if err := restoreSpeedContext(ctx, strings.NewReader(data), int64(len(data)), &speedCursor{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
}

func TestRestoreSpeedContextFindsMarkerAcrossBlockBoundary(t *testing.T) {
	contextLine := `{"type":"turn_context","payload":{"model":"gpt-test","effort":"high"}}` + "\n"
	data := contextLine + strings.Repeat("x", speedMetadataBlock+10-len(contextLine))
	var c speedCursor
	if err := restoreSpeedContext(context.Background(), strings.NewReader(data), int64(len(data)), &c); err != nil {
		t.Fatal(err)
	}
	if c.model != "gpt-test" || c.effort != "high" {
		t.Fatalf("context = %q/%q", c.model, c.effort)
	}
}

func TestScanSpeedReturnsErrorWhenAnotherSessionSucceeds(t *testing.T) {
	home := t.TempDir()
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	writeSpeedSession(t, home, "sessions", uuid(1), now, "good")

	l := &speedLog{base: now.Add(-2 * time.Minute)}
	l.meta(uuid(2), "/work/bad")
	l.turnContext(0, "old", "low")
	l.sb.WriteString(`{"type":"turn_context",broken}` + "\n")
	l.sb.WriteString(fmt.Sprintf(`{"type":"event_msg","payload":{"type":"exec_command_output_delta","chunk":%q}}`+"\n", strings.Repeat("x", speed.TailBytes+1)))
	l.toolOutput(100, "function_call_output")
	l.item(102, "Reasoning", 101)
	l.record(103, 100)
	path := filepath.Join(home, "sessions", "2026", "10", "16", "rollout-2026-10-16T00-00-00-"+uuid(2)+".jsonl")
	l.write(t, path)
	if err := os.Chtimes(path, now.Add(-time.Minute), now.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}

	got, err := (Scanner{CodexHome: home}).ScanSpeed(context.Background(), now)
	if !errors.Is(err, errInvalidSpeedMetadata) || got != nil {
		t.Fatalf("sessions = %+v, err = %v", got, err)
	}
}
