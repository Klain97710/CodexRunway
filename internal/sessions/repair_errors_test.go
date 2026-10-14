package sessions

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRepairRefusesInvalidSessionRootWithoutWriting(t *testing.T) {
	for _, folder := range []string{"sessions", "archived_sessions"} {
		t.Run(folder, func(t *testing.T) {
			f := newFixture(t)
			f.write(folder, "not a directory")
			assertRepairFailsWithoutWriting(t, f.home)
		})
	}
}

func assertRepairFailsWithoutWriting(t *testing.T, home string) {
	t.Helper()
	index := filepath.Join(home, IndexFileName)
	original := []byte("{\"id\":\"keep\",\"thread_name\":\"Keep\",\"updated_at\":\"2026-01-01T00:00:00Z\"}\n")
	if err := os.WriteFile(index, original, 0o600); err != nil {
		t.Fatal(err)
	}
	r := Repairer{CodexHome: home}
	if _, err := r.DryRun(); err == nil {
		t.Fatal("dry run ignored scan failure")
	}
	if _, err := r.Repair(); err == nil {
		t.Fatal("repair ignored scan failure")
	}
	data, err := os.ReadFile(index)
	if err != nil || string(data) != string(original) {
		t.Fatal("failed repair changed index")
	}
	backups, err := filepath.Glob(filepath.Join(home, "session_index.backup-*"))
	if err != nil || len(backups) != 0 {
		t.Fatal("failed scan reached backup/write stage")
	}
}
