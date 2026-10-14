//go:build windows

package grok

import (
	"path/filepath"
	"testing"

	"github.com/licoy/quosparr/internal/platform"
)

func TestWindowsGrokCredentialsAndPendingLoginArePrivate(t *testing.T) {
	s := newTestStore(t)
	data := cred("test@example.com", "windows-private")
	account, err := s.UpsertCredentialData(data, UpsertOptions{Now: testClock})
	if err != nil {
		t.Fatal(err)
	}
	pending, err := s.EnsurePendingHome("test-login")
	if err != nil {
		t.Fatal(err)
	}
	if err := writeAuthJSON(data, pending); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveOfficialAuthDataAtomically(data); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		s.Root(), s.IndexPath(), s.AccountDir(account.ID), s.CredentialPath(account.ID),
		pending, filepath.Join(pending, "auth.json"), s.OfficialHome(), s.OfficialAuthPath(),
	} {
		if err := platform.VerifyOwnerOnly(path, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}
