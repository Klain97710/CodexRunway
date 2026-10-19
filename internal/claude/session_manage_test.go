package claude

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/licoy/quosparr/internal/sessions"
)

func TestListManagedClaudeGroupsFilesKeepsMetadataAndProjectBoundary(t *testing.T) {
	home := t.TempDir()
	config := filepath.Join(home, "custom-claude")
	root := filepath.Join(config, "projects")
	id := "same-session"
	main := filepath.Join(root, "-work-one", id+".jsonl")
	sidecar := filepath.Join(root, "-work-one", id, "subagents", "agent-a.jsonl")
	other := filepath.Join(root, "-work-two", id+".jsonl")
	writeClaudeManaged(t, main,
		`{"type":"ai-title","sessionId":"same-session","aiTitle":"User only","timestamp":"2026-10-18T01:00:00Z","cwd":"/work/one"}`,
		`{"type":"user","sessionId":"same-session","uuid":"u1","timestamp":"2026-10-18T01:00:01Z","message":{"content":"hello"}}`,
		`{"type":"user","sessionId":"same-session","uuid":"u1","timestamp":"2026-10-18T01:00:01Z","message":{"content":"hello"}}`)
	writeClaudeManaged(t, sidecar,
		`{"type":"assistant","sessionId":"same-session","uuid":"r1","timestamp":"2026-10-18T01:00:02Z","message":{"id":"a1","content":[{"type":"text","text":"answer"}]}}`,
		`{"type":"assistant","sessionId":"same-session","uuid":"r1","timestamp":"2026-10-18T01:00:03Z","message":{"id":"a1","content":[{"type":"text","text":"answer expanded"}]}}`,
		`{"type":"assistant","sessionId":"same-session","uuid":"r2","timestamp":"2026-10-18T01:00:04Z","message":{"id":"a1","content":[{"type":"text","text":"second paragraph"}]}}`)
	writeClaudeManaged(t, other,
		`{"type":"user","sessionId":"same-session","uuid":"u2","timestamp":"2026-10-18T02:00:00Z","cwd":"/work/two","message":{"content":"other"}}`)

	items, err := (SessionScanner{ConfigDir: config, Home: home}).ListManaged(context.Background())
	if err != nil || len(items) != 2 {
		t.Fatalf("ListManaged() = %#v, %v", items, err)
	}
	for _, item := range items {
		if item.Path == sidecar {
			t.Fatalf("sidecar selected as primary: %#v", item)
		}
		if item.Path == main {
			if len(item.Files) != 2 || item.Title != "User only" || item.Cwd == nil || *item.Cwd != "/work/one" || item.State != sessions.StateRecent {
				t.Fatalf("main group = %#v", item)
			}
			detail, err := sessions.ReadManagedDetail(context.Background(), item)
			if err != nil || len(detail.Messages) != 3 || detail.Messages[0].Text != "hello" || detail.Messages[1].Text != "answer expanded" || detail.Messages[2].Text != "second paragraph" {
				t.Fatalf("detail = %#v, %v", detail, err)
			}
		}
	}
}

func TestListManagedClaudeCustomExtraRootMalformedAndCancel(t *testing.T) {
	home := t.TempDir()
	extra := filepath.Join(home, ".config", "claude", "projects", "-extra", "bad.jsonl")
	writeClaudeManaged(t, extra, "not json")
	items, err := (SessionScanner{ConfigDir: filepath.Join(home, "custom"), Home: home}).ListManaged(context.Background())
	if err != nil || len(items) != 1 || items[0].Root != filepath.Join(home, ".config", "claude", "projects") || items[0].Problem == "" {
		t.Fatalf("ListManaged() = %#v, %v", items, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := (SessionScanner{ConfigDir: filepath.Join(home, "custom"), Home: home}).ListManaged(ctx); err == nil {
		t.Fatal("cancelled scan succeeded")
	}
}

func TestManagedClaudeRejectsReplacementAfterSnapshot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "projects")
	path := filepath.Join(root, "-work", "one.jsonl")
	writeClaudeManaged(t, path, `{"type":"user","sessionId":"old","timestamp":"2026-10-18T00:00:00Z"}`)
	snapshot, err := sessions.SnapshotManagedFile(root, path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path, path+".replaced"); err != nil {
		t.Fatal(err)
	}
	writeClaudeManaged(t, path, `{"type":"user","sessionId":"new","timestamp":"2026-10-18T00:00:00Z"}`)
	if err := os.Chtimes(path, snapshot.ModTime, snapshot.ModTime); err != nil {
		t.Fatal(err)
	}
	replacement, err := os.Lstat(path)
	if err != nil || replacement.Size() != snapshot.Size || !replacement.ModTime().Equal(snapshot.ModTime) {
		t.Fatalf("replacement metadata = %#v, %v", replacement, err)
	}
	if _, err := readManagedClaudePart(context.Background(), root, path, snapshot); err == nil {
		t.Fatal("replacement accepted with old snapshot")
	}
}

func writeClaudeManaged(t *testing.T, path string, lines ...string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	text := ""
	for _, line := range lines {
		text += line + "\n"
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}
