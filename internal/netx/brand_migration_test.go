package netx

import (
	"bytes"
	"errors"
	"testing"
)

func TestBrandCredentialMigrationKeepsPermissionErrors(t *testing.T) {
	data, err := encodeCredentials(Credentials{Username: "fixture", Password: "synthetic"})
	if err != nil {
		t.Fatal(err)
	}
	previous := &fakeRaw{items: map[string][]byte{testUUID: data}}
	current := &fakeRaw{}
	s := renamedSecrets{current, []rawSecrets{previous}}
	got, found, err := s.get(testUUID, false)
	if err != nil || !found || !bytes.Equal(got, data) || !bytes.Equal(current.items[testUUID], data) || !bytes.Equal(previous.items[testUUID], data) {
		t.Fatal("credential was not copied and preserved")
	}
	if previous.interactive[0] {
		t.Fatal("migration enabled a background permission prompt")
	}
	current.getErr = errors.New("denied")
	if _, _, err := s.get(testUUID, false); err == nil || len(previous.interactive) != 1 {
		t.Fatal("permission error was bypassed by a legacy lookup")
	}
}

func TestBrandCredentialsPreferLatestPreviewService(t *testing.T) {
	data, err := encodeCredentials(Credentials{Username: "preview", Password: "synthetic"})
	if err != nil {
		t.Fatal(err)
	}
	preview := &fakeRaw{items: map[string][]byte{testUUID: data}}
	older := &fakeRaw{getErr: errors.New("must not read older credentials")}
	s := renamedSecrets{&fakeRaw{}, []rawSecrets{preview, older}}
	got, found, err := s.get(testUUID, false)
	if err != nil || !found || !bytes.Equal(got, data) || len(older.interactive) != 0 {
		t.Fatal("latest preview credentials were not preferred")
	}
	preview.getErr = errors.New("denied")
	s.current = &fakeRaw{}
	if _, _, err := s.get(testUUID, false); err == nil || len(older.interactive) != 0 {
		t.Fatal("preview permission error was bypassed")
	}
}
