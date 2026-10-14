//go:build darwin

package grok

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

const ufImmutable = 0x2 // UF_IMMUTABLE

// An immutable file makes the final cleanup of the moved credential directory
// fail after the index was saved, which must roll everything back.
func TestRemoveRollsBackAfterCleanupFailure(t *testing.T) {
	store := newTestStore(t)
	current := mustUpsert(t, store, cred("current@example.com", "current"), true)
	removableData := cred("removable@example.com", "removable")
	removable := mustUpsert(t, store, removableData, false)

	blocker := filepath.Join(store.AccountDir(removable.ID), "zz-immutable-blocker")
	if err := os.WriteFile(blocker, []byte("blocker"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Chflags(blocker, ufImmutable); err != nil {
		t.Skipf("cannot set the immutable flag: %v", err)
	}
	defer func() {
		_ = syscall.Chflags(blocker, 0)
		entries, _ := os.ReadDir(store.Root())
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), ".grok-remove-") {
				_ = syscall.Chflags(filepath.Join(store.Root(), e.Name(), "zz-immutable-blocker"), 0)
			}
		}
	}()

	err := store.Remove(removable.ID)
	var ioErr *AccountIOError
	if !errors.As(err, &ioErr) || ioErr.Message != "Unable to remove the Grok account." {
		t.Fatalf("err = %v", err)
	}
	idx, _ := store.LoadIndex()
	if idx.CurrentAccountID != current.ID {
		t.Error("current account changed")
	}
	if _, ok := idx.Account(removable.ID); !ok {
		t.Error("account row not restored")
	}
	if got, err := store.LoadCredentialData(removable.ID); err != nil || !bytes.Equal(got, removableData) {
		t.Errorf("credential not restored (%v)", err)
	}
	if info, err := os.Stat(store.CredentialPath(removable.ID)); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("credential mode = %v (%v)", info, err)
	}
}
