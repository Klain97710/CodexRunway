package grok

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestRemoveDeletesAccountAndDirectory(t *testing.T) {
	store := newTestStore(t)
	current := mustUpsert(t, store, cred("current@example.com", "current"), true)
	other := mustUpsert(t, store, cred("other@example.com", "other"), false)
	if err := store.Remove(other.ID); err != nil {
		t.Fatal(err)
	}
	idx, _ := store.LoadIndex()
	if _, ok := idx.Account(other.ID); ok || idx.CurrentAccountID != current.ID {
		t.Errorf("index = %+v", idx)
	}
	if _, err := os.Stat(store.AccountDir(other.ID)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("directory still there: %v", err)
	}
	entries, _ := os.ReadDir(store.Root())
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".grok-remove-") {
			t.Errorf("tombstone %s left behind", e.Name())
		}
	}
}

func TestRemoveRefusesCurrentAndUnknown(t *testing.T) {
	store := newTestStore(t)
	current := mustUpsert(t, store, cred("current@example.com", "current"), true)
	var cannot *CurrentAccountCannotBeRemovedError
	if err := store.Remove(current.ID); !errors.As(err, &cannot) {
		t.Errorf("err = %v", err)
	}
	var missing *AccountNotFoundError
	if err := store.Remove("nope"); !errors.As(err, &missing) {
		t.Errorf("err = %v", err)
	}
	if _, err := os.Stat(store.CredentialPath(current.ID)); err != nil {
		t.Errorf("current credential gone: %v", err)
	}
}
