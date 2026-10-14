//go:build darwin

package grok

import (
	"bytes"
	"errors"
	"os"
	"syscall"
	"testing"
)

// failIndexCommits makes every write of grok-index.json fail by flagging the
// file immutable; the returned function undoes it.
func failIndexCommits(t *testing.T, s *Store) func() {
	t.Helper()
	if err := syscall.Chflags(s.IndexPath(), ufImmutable); err != nil {
		t.Skipf("cannot make the index immutable: %v", err)
	}
	return func() { _ = syscall.Chflags(s.IndexPath(), 0) }
}

func TestFailedIndexCommitRestoresOriginalOfficialAuth(t *testing.T) {
	rig := newModuleRig(t)
	apiKeyOnly := []byte(`{"xai::api_key":{"auth_mode":"api_key","key":"official-api-key-for-tests"}}`)
	writeOfficial(t, rig.store, apiKeyOnly)
	first := mustUpsert(t, rig.store, cred("first@example.com", "first-user"), true)
	second := mustUpsert(t, rig.store, cred("second@example.com", "second-user"), false)
	defer failIndexCommits(t, rig.store)()

	_, err := rig.apply(Command{Kind: CommandMakeCurrent, ID: second.ID, AllowWhileRunning: true})
	if err == nil {
		t.Fatal("expected the immutable index commit to fail")
	}
	var partial *PartialWriteError
	if errors.As(err, &partial) {
		t.Errorf("a successful rollback must report the original failure, got %v", err)
	}
	if !bytes.Equal(readOfficial(t, rig.store), apiKeyOnly) {
		t.Error("the official file was not restored byte for byte")
	}
	if idx, _ := rig.store.LoadIndex(); idx.CurrentAccountID != first.ID {
		t.Errorf("current = %q, want %q", idx.CurrentAccountID, first.ID)
	}
}

func TestFailedIndexCommitRestoresMissingOfficialAuth(t *testing.T) {
	rig := newModuleRig(t)
	first := mustUpsert(t, rig.store, cred("first@example.com", "first-user"), true)
	second := mustUpsert(t, rig.store, cred("second@example.com", "second-user"), false)
	defer failIndexCommits(t, rig.store)()

	if _, err := rig.apply(Command{Kind: CommandMakeCurrent, ID: second.ID, AllowWhileRunning: true}); err == nil {
		t.Fatal("expected the immutable index commit to fail")
	}
	if _, err := os.Stat(rig.store.OfficialAuthPath()); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("official auth.json should be absent again, stat err = %v", err)
	}
	if idx, _ := rig.store.LoadIndex(); idx.CurrentAccountID != first.ID {
		t.Errorf("current = %q, want %q", idx.CurrentAccountID, first.ID)
	}
}
