package sessions

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListManagedIncludesAllAndArchived(t *testing.T) {
	home := t.TempDir()
	for i := 0; i < 20; i++ {
		folder := "sessions"
		if i == 19 {
			folder = "archived_sessions"
		}
		id := fmt.Sprintf("00000000-0000-4000-8000-%012d", i)
		path := filepath.Join(home, folder, "2026", "10", "18", "rollout-"+id+".jsonl")
		writeManagedFixture(t, path, fmt.Sprintf(`{"timestamp":"2026-10-18T00:00:%02dZ","type":"session_meta","payload":{"id":%q,"cwd":"/tmp/demo"}}
{"timestamp":"2026-10-18T00:01:%02dZ","type":"response_item","payload":{"type":"message","role":"user","content":"hello"}}
{"timestamp":"2026-10-18T00:02:%02dZ","type":"event_msg","payload":{"type":"token_count"},"turn_context":{"model":"gpt-test"}}
`, i%60, id, i%60, i%60))
	}
	items, err := (Scanner{CodexHome: home}).ListManaged(context.Background())
	if err != nil || len(items) != 20 {
		t.Fatalf("ListManaged() = %d, %v", len(items), err)
	}
	archived := 0
	for _, item := range items {
		if item.Provider != ProviderCodex || item.Root == "" || len(item.Files) != 1 || item.Files[0].Info == nil || item.Model == nil || *item.Model != "gpt-test" {
			t.Fatalf("bad identity metadata: %#v", item)
		}
		if item.Archived {
			archived++
		}
	}
	if archived != 1 {
		t.Fatalf("archived = %d", archived)
	}
}

func TestManagedCodexProblemAndLargeProbe(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, "sessions")
	bad := filepath.Join(root, "bad.jsonl")
	writeManagedFixture(t, bad, "not json\n")
	large := filepath.Join(root, "large.jsonl")
	id := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	writeManagedFixture(t, large, fmt.Sprintf(`{"timestamp":"2026-10-18T00:00:00Z","type":"session_meta","payload":{"id":%q}}
{"timestamp":"2026-10-18T00:00:01Z","type":"event_msg","payload":{"type":"message","role":"user","content":%q}}
`, id, strings.Repeat("x", headProbeBytes)))
	items, err := (Scanner{CodexHome: home}).ListManaged(context.Background())
	if err != nil || len(items) != 2 {
		t.Fatalf("ListManaged() = %#v, %v", items, err)
	}
	for _, item := range items {
		if item.Path == bad && item.Problem == "" {
			t.Fatal("malformed file has no problem")
		}
		if item.Path == large && item.Problem != "" {
			t.Fatalf("large valid file problem = %q", item.Problem)
		}
	}
}

func TestReadManagedDetailMessagesTruncationAndIdentity(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "one.jsonl")
	writeManagedFixture(t, path, `{"timestamp":"2026-10-18T00:00:00Z","type":"response_item","payload":{"type":"message","role":"user","content":"question"}}
{"timestamp":"2026-10-18T00:00:01Z","type":"event_msg","payload":{"type":"agent_message","message":"answer"}}
{"timestamp":"2026-10-18T00:00:02Z","type":"response_item","payload":{"type":"message","role":"assistant","content":"answer"}}
{"timestamp":"2026-10-18T00:00:03Z","type":"event_msg","payload":{"type":"exec_command_output_delta","chunk":"secret tool output"}}
{"timestamp":"2026-10-18T00:00:04Z","type":"response_item","payload":{"type":"message","role":"assistant","content":"repeat"}}
{"timestamp":"2026-10-18T00:00:05Z","type":"response_item","payload":{"type":"message","role":"assistant","content":"repeat"}}
`)
	file, err := SnapshotManagedFile(root, path)
	if err != nil {
		t.Fatal(err)
	}
	session := ManagedSession{Provider: ProviderCodex, Root: root, Path: path, Files: []ManagedFile{file}}
	detail, err := ReadManagedDetail(context.Background(), session)
	if err != nil || len(detail.Messages) != 4 || detail.Messages[0].Text != "question" || detail.Messages[1].Text != "answer" || detail.Messages[2].Text != "repeat" || detail.Messages[3].Text != "repeat" {
		t.Fatalf("ReadManagedDetail() = %#v, %v", detail, err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"timestamp":"2026-10-18T00:00:04Z","type":"response_item","payload":{"type":"message","role":"user","content":"grown"}}` + "\n"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if grown, err := ReadManagedDetail(context.Background(), session); err != nil || len(grown.Messages) != 5 {
		t.Fatalf("grown detail = %#v, %v", grown, err)
	}

	if err := os.Rename(path, path+".replaced"); err != nil {
		t.Fatal(err)
	}
	writeManagedFixture(t, path, `{"type":"response_item","payload":{"type":"message","role":"user","content":"replacement"}}`)
	if _, err := ReadManagedDetail(context.Background(), session); err == nil || !strings.Contains(err.Error(), "identity changed") {
		t.Fatalf("replacement error = %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.jsonl")
	writeManagedFixture(t, outside, "{}\n")
	if err := os.Symlink(outside, path); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadManagedDetail(context.Background(), session); err == nil {
		t.Fatal("symlink replacement was read")
	}

	big := filepath.Join(root, "big.jsonl")
	writeManagedFixture(t, big, `{"type":"response_item","payload":{"type":"message","role":"user","content":`+fmt.Sprintf("%q", strings.Repeat("x", managedTextBytes+10))+`}}
`)
	bigFile, _ := SnapshotManagedFile(root, big)
	detail, err = ReadManagedDetail(context.Background(), ManagedSession{Provider: ProviderCodex, Root: root, Path: big, Files: []ManagedFile{bigFile}})
	if err != nil || !detail.Truncated || len(detail.Messages) != 1 || !strings.HasSuffix(detail.Messages[0].Text, "…") {
		t.Fatalf("large detail = %#v, %v", detail, err)
	}

	longLine := filepath.Join(root, "long-line.jsonl")
	writeManagedFixture(t, longLine, strings.Repeat("x", managedLineBytes+1)+"\n"+`{"type":"response_item","payload":{"type":"message","role":"user","content":"after"}}`+"\n")
	longFile, _ := SnapshotManagedFile(root, longLine)
	detail, err = ReadManagedDetail(context.Background(), ManagedSession{Provider: ProviderCodex, Root: root, Path: longLine, Files: []ManagedFile{longFile}})
	if err != nil || !detail.Truncated || len(detail.Messages) != 1 || detail.Messages[0].Text != "after" {
		t.Fatalf("long-line detail = %#v, %v", detail, err)
	}
}

func TestSnapshotManagedFileFreezesIdentity(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "one.jsonl")
	writeManagedFixture(t, path, "original\n")
	file, err := SnapshotManagedFile(root, path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path, path+".replaced"); err != nil {
		t.Fatal(err)
	}
	writeManagedFixture(t, path, "replaced\n")
	if err := os.Chtimes(path, file.ModTime, file.ModTime); err != nil {
		t.Fatal(err)
	}
	if err := VerifyManagedFileIdentity(root, file); err == nil || !strings.Contains(err.Error(), "identity changed") {
		t.Fatalf("replacement error = %v", err)
	}
}

func TestManagedPathsRejectParentSymlinkAndCancellation(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	writeManagedFixture(t, filepath.Join(outside, "one.jsonl"), "{}\n")
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Fatal(err)
	}
	if _, err := SnapshotManagedFile(root, filepath.Join(root, "linked", "one.jsonl")); err == nil {
		t.Fatal("parent symlink accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := (Scanner{CodexHome: root}).ListManaged(ctx); err == nil {
		t.Fatal("cancelled scan succeeded")
	}
	path := filepath.Join(root, "bad.jsonl")
	writeManagedFixture(t, path, "not json\n")
	file, err := SnapshotManagedFile(root, path)
	if err != nil {
		t.Fatal(err)
	}
	session := ManagedSession{Provider: ProviderCodex, Root: root, Path: path, Files: []ManagedFile{file}}
	if _, err := ReadManagedDetail(context.Background(), session); err == nil {
		t.Fatal("malformed detail succeeded")
	}
	if _, err := ReadManagedDetail(ctx, session); err == nil {
		t.Fatal("cancelled detail succeeded")
	}
}

func writeManagedFixture(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}
