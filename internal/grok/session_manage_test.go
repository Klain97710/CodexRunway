package grok

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/licoy/quosparr/internal/sessions"
)

func TestListManagedGrokIsUnlimitedAndReadsDetail(t *testing.T) {
	home := t.TempDir()
	for i := 0; i < 301; i++ {
		dir := filepath.Join(home, "sessions", "project", fmt.Sprintf("session-%03d", i))
		writeGrokManaged(t, filepath.Join(dir, "summary.json"), fmt.Sprintf(`{
  "info": {"id": "session-%03d", "cwd": "/work/demo"},
  "generated_title": "Session %03d",
  "current_model_id": "grok-4",
  "updated_at": "2026-10-18T01:00:00Z"
}`, i, i))
		if i == 300 {
			writeGrokManaged(t, filepath.Join(dir, "events.jsonl"), `{"timestamp":1792285200.25,"params":{"prompt":"question one"}}
{"timestamp":1792285204,"params":{"prompt":"question two"}}
`)
			writeGrokManaged(t, filepath.Join(dir, "updates.jsonl"), `{"timestamp":1792285201,"params":{"update":{"sessionUpdate":"agent_message_chunk","messageId":"a1","content":{"type":"text","text":"hello "}}}}
{"timestamp":1792285202,"params":{"update":{"sessionUpdate":"agent_message_chunk","messageId":"a1","content":{"type":"text","text":"world"}}}}
{"timestamp":1792285203,"params":{"update":{"sessionUpdate":"agent_message_chunk","messageId":"a2","content":{"type":"text","text":"separate"}}}}
{"timestamp":1792285203.5,"params":{"update":{"sessionUpdate":"turn_completed"}}}
{"timestamp":1792285205,"params":{"update":{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"ha"}}}}
{"timestamp":1792285206,"params":{"update":{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"ha"}}}}
{"timestamp":1792285207,"params":{"update":{"sessionUpdate":"turn_completed"}}}
`)
		}
	}
	items, err := (SessionScanner{GrokHome: home}).ListManaged(context.Background())
	if err != nil || len(items) != 301 {
		t.Fatalf("ListManaged() = %d, %v", len(items), err)
	}
	var target sessions.ManagedSession
	for _, item := range items {
		if item.ID == "session-300" {
			target = item
		}
	}
	if target.Path == "" || target.Root != filepath.Join(home, "sessions") || len(target.Files) != 3 || target.Model == nil || *target.Model != "grok-4" {
		t.Fatalf("target = %#v", target)
	}
	detail, err := sessions.ReadManagedDetail(context.Background(), target)
	want := []string{"question one", "hello world", "separate", "question two", "haha"}
	if err != nil || len(detail.Messages) != len(want) {
		t.Fatalf("detail = %#v, %v", detail, err)
	}
	for i, text := range want {
		if detail.Messages[i].Text != text {
			t.Fatalf("detail[%d] = %#v", i, detail.Messages[i])
		}
	}
	if detail.Messages[0].At.Nanosecond() != 250_000_000 {
		t.Fatalf("fractional timestamp = %v", detail.Messages[0].At)
	}
}

func TestListManagedGrokMalformedAndCancelled(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "sessions", "project", "bad")
	writeGrokManaged(t, filepath.Join(dir, "summary.json"), "{bad")
	items, err := (SessionScanner{GrokHome: home}).ListManaged(context.Background())
	if err != nil || len(items) != 1 || items[0].Problem == "" || items[0].State != sessions.StateNeedsAttention {
		t.Fatalf("ListManaged() = %#v, %v", items, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := (SessionScanner{GrokHome: home}).ListManaged(ctx); err == nil {
		t.Fatal("cancelled scan succeeded")
	}
}

func TestManagedGrokRejectsReplacementAfterSnapshot(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, "sessions")
	dir := filepath.Join(root, "project", "one")
	summary := filepath.Join(dir, "summary.json")
	writeGrokManaged(t, summary, `{"info":{"id":"old"}}`)
	files, err := grokManagedSnapshots(root, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("snapshots = %#v", files)
	}
	if err := os.Rename(summary, summary+".replaced"); err != nil {
		t.Fatal(err)
	}
	writeGrokManaged(t, summary, `{"info":{"id":"new"}}`)
	if err := os.Chtimes(summary, files[0].ModTime, files[0].ModTime); err != nil {
		t.Fatal(err)
	}
	replacement, err := os.Lstat(summary)
	if err != nil || replacement.Size() != files[0].Size || !replacement.ModTime().Equal(files[0].ModTime) {
		t.Fatalf("replacement metadata = %#v, %v", replacement, err)
	}
	if _, err := (SessionScanner{GrokHome: home}).managedFromCapturedSummary(root, summary, files); err == nil {
		t.Fatal("replacement accepted with old snapshot")
	}
}

func writeGrokManaged(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}
