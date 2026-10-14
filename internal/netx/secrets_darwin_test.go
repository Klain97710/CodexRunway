//go:build darwin

package netx

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestKeychainRoundTrip uses the real login Keychain with a throwaway service
// name, so it only runs when asked for: QUOSPARR_KEYCHAIN_TEST=1.
func TestKeychainRoundTrip(t *testing.T) {
	if os.Getenv("QUOSPARR_KEYCHAIN_TEST") != "1" {
		t.Skip("set QUOSPARR_KEYCHAIN_TEST=1 to exercise the real Keychain")
	}
	service := "com.quosparr.app.network-proxy.test-" + strings.ToLower(testUUID)
	s := NewCredentialStore(service)
	t.Cleanup(func() { _ = s.Delete(testUUID) })

	if _, err := s.Load(testUUID, false); !errors.Is(err, ErrCredentialsUnavailable) {
		t.Fatalf("missing item: %v", err)
	}
	want := Credentials{Username: "fixture-user", Password: "fixture-password"}
	if err := s.Save(want, testUUID); err != nil {
		t.Fatal(err)
	}
	// Saving again goes through SecItemUpdate.
	want.Password = "fixture-password-2"
	if err := s.Save(want, testUUID); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load(testUUID, false)
	if err != nil || got != want {
		t.Fatalf("load: %v", err)
	}
	// The stored attributes and raw value, as the security tool reports them.
	out, err := exec.Command("/usr/bin/security", "find-generic-password", "-s", service, "-a", testUUID).CombinedOutput()
	if err != nil {
		t.Fatalf("security: %v", err)
	}
	attrs := string(out)
	for _, expect := range []string{`class: "genp"`, `"svce"<blob>="` + service + `"`, `"acct"<blob>="` + testUUID + `"`} {
		if !strings.Contains(attrs, expect) {
			t.Logf("attributes:\n%s", attrs)
			t.Fatalf("missing %s", expect)
		}
	}
	if err := s.Delete(testUUID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load(testUUID, false); !errors.Is(err, ErrCredentialsUnavailable) {
		t.Fatalf("deleted item still loads: %v", err)
	}
	if err := s.Delete(testUUID); err != nil {
		t.Fatalf("deleting a missing item must succeed: %v", err)
	}
}
