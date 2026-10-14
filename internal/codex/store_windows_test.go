//go:build windows

package codex

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/licoy/quosparr/internal/platform"
)

func TestWindowsAccountAndOfficialCredentialsArePrivate(t *testing.T) {
	e := newEnv(t)
	auth := sampleAuth(t, sampleOpt{accountID: "windows-private", email: "test@example.com", refresh: "synthetic-refresh"})
	account := upsert(t, e.store, auth, true)
	e.writeOfficial(t, auth)
	for _, path := range []string{
		e.store.Root(), e.store.IndexPath(), e.store.AccountDir(account.ID), e.store.CredentialPath(account.ID),
		filepath.Dir(e.officialPath()), e.officialPath(),
	} {
		if err := platform.VerifyOwnerOnly(path, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestWindowsSwitchRollbackDoesNotUseReportedPOSIXModeForACL(t *testing.T) {
	f := newSwitchFixture(t, false, false)
	sw := f.switcher(SwitchHooks{})
	sw.failpoint = func(stage string) error {
		if stage == "credential" {
			return errors.New("synthetic write failure")
		}
		return nil
	}
	if _, err := sw.SwitchTo(context.Background(), f.target.ID); err == nil {
		t.Fatal("expected the switch to roll back")
	}
	for _, path := range []string{f.officialPath(), f.store.CredentialPath(f.target.ID), f.store.IndexPath()} {
		if err := platform.VerifyOwnerOnly(path, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}
