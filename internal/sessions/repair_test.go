package sessions

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func repairSession(id, title, timestamp string) string {
	return fmt.Sprintf(`{"timestamp":%q,"type":"session_meta","payload":{"id":%q,"cwd":"/tmp"}}
{"timestamp":%q,"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":%q}]}}`,
		timestamp, id, timestamp, title)
}

func TestDryRunReportsIndexProblems(t *testing.T) {
	f := newFixture(t)
	f.write("sessions/2026/06/29/rollout-s1.jsonl", repairSession("s1", "First", "2026-06-29T01:00:00Z"))
	f.write("sessions/2026/06/29/rollout-s2.jsonl", repairSession("s2", "Second", "2026-06-29T02:00:00Z"))
	index := `{"id":"s1","thread_name":"First","updated_at":"2026-06-29T01:00:00Z"}
{"id":"s1","thread_name":"First copy","updated_at":"2026-06-29T01:00:00Z"}
{"id":"old","thread_name":"Gone","updated_at":"2026-06-28T01:00:00Z"}`
	f.write("session_index.jsonl", index)

	report, err := Repairer{CodexHome: f.home}.DryRun()
	if err != nil {
		t.Fatal(err)
	}
	want := Report{
		MissingIndexIDs:   []string{"s2"},
		OrphanIndexIDs:    []string{"old"},
		DuplicateIndexIDs: []string{"s1"},
		PlannedEntries:    2,
	}
	if !reflect.DeepEqual(report, want) {
		t.Fatalf("report %+v, want %+v", report, want)
	}
	if got, _ := os.ReadFile(filepath.Join(f.home, "session_index.jsonl")); string(got) != index {
		t.Fatal("dry run must not touch the index")
	}
	if entries, _ := filepath.Glob(filepath.Join(f.home, "session_index.backup-*")); len(entries) != 0 {
		t.Fatal("dry run must not write a backup")
	}
}

func TestDryRunTracksSessionIndexUpdates(t *testing.T) {
	f := newFixture(t)
	r := Repairer{CodexHome: f.home}
	checkMissing := func(want int) {
		t.Helper()
		report, err := r.DryRun()
		if err != nil {
			t.Fatal(err)
		}
		if len(report.MissingIndexIDs) != want {
			t.Fatalf("missing IDs = %v, want %d", report.MissingIndexIDs, want)
		}
	}
	f.write("sessions/rollout-s1.jsonl", repairSession("s1", "First", "2026-06-29T01:00:00Z"))
	checkMissing(1)
	// A later index write resolves the missing entry without a repair.
	f.write("session_index.jsonl", `{"id":"s1","thread_name":"First","updated_at":"2026-06-29T01:00:00Z"}`)
	checkMissing(0)
	f.write("sessions/rollout-s2.jsonl", repairSession("s2", "Second", "2026-06-29T02:00:00Z"))
	checkMissing(1)
	if _, err := r.Repair(); err != nil {
		t.Fatal(err)
	}
	checkMissing(0)
}

func TestStaleTitles(t *testing.T) {
	f := newFixture(t)
	f.write("sessions/2026/06/29/rollout-s1.jsonl", repairSession("s1", "Real title", "2026-06-29T01:00:00Z"))
	f.write("session_index.jsonl", `{"id":"s1","thread_name":"Renamed","updated_at":"2026-06-29T01:00:00Z"}`)
	report, err := Repairer{CodexHome: f.home}.DryRun()
	if err != nil || !reflect.DeepEqual(report.StaleTitleIDs, []string{"s1"}) {
		t.Fatalf("%+v %v", report, err)
	}
}

func TestRepairBacksUpAndRewritesIndex(t *testing.T) {
	f := newFixture(t)
	session := f.write("sessions/2026/06/29/rollout-s1.jsonl", repairSession("s1", "First", "2026-06-29T01:00:00Z"))
	archived := f.write("archived_sessions/rollout-s0.jsonl", repairSession("s0", "Archived", "2026-06-20T01:00:00Z"))
	oldIndex := `{"id":"old","thread_name":"Gone","updated_at":"2026-06-28T01:00:00Z"}`
	f.write("session_index.jsonl", oldIndex)
	sessionBefore, _ := os.ReadFile(session)
	archivedBefore, _ := os.ReadFile(archived)

	now := time.Unix(1_782_710_400, 0)
	result, err := Repairer{CodexHome: f.home, Now: func() time.Time { return now }}.Repair()
	if err != nil {
		t.Fatal(err)
	}
	wantBackup := filepath.Join(f.home, "session_index.backup-1782710400.jsonl")
	if result.BackupPath != wantBackup {
		t.Fatalf("backup %q, want %q", result.BackupPath, wantBackup)
	}
	if got, _ := os.ReadFile(wantBackup); string(got) != oldIndex {
		t.Fatalf("backup content %q", got)
	}
	repaired, _ := os.ReadFile(filepath.Join(f.home, "session_index.jsonl"))
	wantIndex := `{"id":"s1","thread_name":"First","updated_at":"2026-06-29T01:00:00Z"}
{"id":"s0","thread_name":"Archived","updated_at":"2026-06-20T01:00:00Z"}
`
	if string(repaired) != wantIndex {
		t.Fatalf("index %q", repaired)
	}
	if !reflect.DeepEqual(result.MissingIndexIDs, []string{"s0", "s1"}) || !reflect.DeepEqual(result.OrphanIndexIDs, []string{"old"}) {
		t.Fatalf("report %+v", result)
	}
	// Only session_index.jsonl and its backup changed: session files are intact.
	if b, _ := os.ReadFile(session); !bytes.Equal(b, sessionBefore) {
		t.Fatal("session file modified")
	}
	if b, _ := os.ReadFile(archived); !bytes.Equal(b, archivedBefore) {
		t.Fatal("archived session file modified")
	}
	entries, _ := os.ReadDir(f.home)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if want := []string{"archived_sessions", "session_index.backup-1782710400.jsonl", "session_index.jsonl", "sessions"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("home entries %v, want %v (no stray temp files)", names, want)
	}
}

func TestRepairWithoutExistingIndex(t *testing.T) {
	f := newFixture(t)
	f.write("sessions/2026/06/29/rollout-s1.jsonl", repairSession("s1", "First", "2026-06-29T01:00:00Z"))
	result, err := Repairer{CodexHome: f.home}.Repair()
	if err != nil || result.BackupPath != "" {
		t.Fatalf("%+v %v", result, err)
	}
	if got, _ := os.ReadFile(filepath.Join(f.home, "session_index.jsonl")); !strings.Contains(string(got), `"id":"s1"`) {
		t.Fatalf("index %q", got)
	}
}

func TestRepairKeepsNewestDuplicateAndEscapesTitle(t *testing.T) {
	f := newFixture(t)
	f.write("sessions/a/rollout-s1.jsonl", repairSession("s1", `old "quoted" <b>`, "2026-06-29T01:00:00Z"))
	f.write("archived_sessions/rollout-s1.jsonl", repairSession("s1", "new / title", "2026-06-29T05:00:00Z"))
	if _, err := (Repairer{CodexHome: f.home}).Repair(); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(f.home, "session_index.jsonl"))
	if want := `{"id":"s1","thread_name":"new / title","updated_at":"2026-06-29T05:00:00Z"}` + "\n"; string(got) != want {
		t.Fatalf("index %q", got)
	}
	f2 := newFixture(t)
	f2.write("sessions/rollout-s1.jsonl", repairSession("s1", `a "q" <b> \ x`, "2026-06-29T01:00:00Z"))
	_, _ = Repairer{CodexHome: f2.home}.Repair()
	got, _ = os.ReadFile(filepath.Join(f2.home, "session_index.jsonl"))
	if want := `{"id":"s1","thread_name":"a \"q\" <b> \\ x","updated_at":"2026-06-29T01:00:00Z"}` + "\n"; string(got) != want {
		t.Fatalf("index %q", got)
	}
}

func TestDryRunOnlyNeedsValidHeadAndTail(t *testing.T) {
	f := newFixture(t)
	var data bytes.Buffer
	data.WriteString(`{"timestamp":"2026-06-29T01:00:00Z","type":"session_meta","payload":{"id":"s1","cwd":"/tmp"}}` + "\n")
	data.Write([]byte{0xff, 0xfe, 0xfd, '\n'})
	data.WriteString(`{"timestamp":"2026-06-29T01:01:00Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"Tail title"}]}}`)
	f.write("sessions/2026/06/29/rollout-s1.jsonl", data.String())
	report, err := Repairer{CodexHome: f.home}.DryRun()
	if err != nil || report.PlannedEntries != 1 {
		t.Fatalf("%+v %v", report, err)
	}
}

func TestRepairOnlyReadsHeadAndTailOfBigFiles(t *testing.T) {
	f := newFixture(t)
	var b strings.Builder
	b.WriteString(`{"timestamp":"2026-06-29T01:00:00Z","type":"session_meta","payload":{"id":"big","cwd":"/tmp"}}` + "\n")
	for i := 0; i < 20000; i++ {
		b.WriteString(`{"timestamp":"2026-06-29T01:00:00Z","type":"event_msg","payload":{"type":"agent_message","text":"padding padding padding"}}` + "\n")
	}
	b.WriteString(`{"timestamp":"2026-06-30T09:00:00Z","type":"event_msg","payload":{"type":"task_complete"}}` + "\n")
	f.write("sessions/rollout-big.jsonl", b.String())
	if _, err := (Repairer{CodexHome: f.home}).Repair(); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(f.home, "session_index.jsonl"))
	if want := `{"id":"big","thread_name":"Untitled","updated_at":"2026-06-30T09:00:00Z"}` + "\n"; string(got) != want {
		t.Fatalf("index %q", got)
	}
}

func TestIndexLineDecoding(t *testing.T) {
	for _, c := range []struct {
		line string
		ok   bool
		name string
	}{
		{`{"id":"a","updated_at":"2026-06-29T01:00:00Z"}`, true, "Untitled"},
		{`{"id":"a","thread_name":null,"updated_at":"2026-06-29T01:00:00Z"}`, true, "Untitled"},
		{`{"id":"a","thread_name":"T","updated_at":"not a date"}`, true, "T"},
		{`{"id":"a","thread_name":5,"updated_at":"2026-06-29T01:00:00Z"}`, false, ""},
		{`{"thread_name":"T","updated_at":"2026-06-29T01:00:00Z"}`, false, ""},
		{`{"id":"a","thread_name":"T"}`, false, ""},
		{`not json`, false, ""},
	} {
		e, ok := decodeIndexLine(c.line)
		if ok != c.ok || (ok && e.ThreadName != c.name) {
			t.Errorf("%s: ok=%v name=%q", c.line, ok, e.ThreadName)
		}
	}
}

func TestBackupNeverOverwrites(t *testing.T) {
	f := newFixture(t)
	f.write("sessions/rollout-s1.jsonl", repairSession("s1", "First", "2026-06-29T01:00:00Z"))
	f.write("session_index.jsonl", "original\n")
	f.write("session_index.backup-100.jsonl", "earlier backup\n")
	r := Repairer{CodexHome: f.home, Now: func() time.Time { return time.Unix(100, 0) }}
	if _, err := r.Repair(); err == nil {
		t.Fatal("a same-second backup must not be overwritten")
	}
	if got, _ := os.ReadFile(filepath.Join(f.home, "session_index.backup-100.jsonl")); string(got) != "earlier backup\n" {
		t.Fatal("existing backup changed")
	}
	if got, _ := os.ReadFile(filepath.Join(f.home, "session_index.jsonl")); string(got) != "original\n" {
		t.Fatal("index must stay untouched when the backup failed")
	}
}
