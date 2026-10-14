package grok

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"os"
	"testing"
)

// Exercise the real refresh request boundary, including a token rotation by
// another process for the same user, which an identity-only check would miss.
func TestRefreshPreservesOfficialChangesDuringRequest(t *testing.T) {
	for _, change := range []string{"switch", "logout", "same account rotation"} {
		t.Run(change, func(t *testing.T) {
			rig := newModuleRig(t)
			before := credSpec{email: "a@example.com", user: "a", refresh: "a-refresh", expiresAt: expired}.json()
			writeOfficial(t, rig.store, before)
			id := rig.load().CurrentAccountID
			external := cred("b@example.com", "b")
			if change == "same account rotation" {
				external = credSpec{email: "a@example.com", user: "a", refresh: "external-rotation"}.json()
			}
			requests := 0
			rig.mod.Refresher.HTTP = handlerClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if change == "logout" {
					if err := os.Remove(rig.store.OfficialAuthPath()); err != nil {
						t.Fatal(err)
					}
				} else {
					writeOfficial(t, rig.store, external)
				}
				rig.tokens.ServeHTTP(w, r)
			}))
			_, err := rig.mod.ensureValidCredential(context.Background(), id, true)
			if err == nil || requests != 1 {
				t.Fatalf("changed official credential must stop refresh write-back: err=%v requests=%d", err, requests)
			}
			if change == "logout" {
				if _, err := os.Stat(rig.store.OfficialAuthPath()); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("refresh restored logged-out account")
				}
			} else if !bytes.Equal(readOfficial(t, rig.store), external) {
				t.Fatal("refresh overwrote external credential")
			}
			managed, err := rig.store.LoadCredentialData(id)
			if err != nil || fieldOf(t, managed, "refresh_token") != "rotated-1" {
				t.Fatal("rotated managed credential was lost")
			}
			doc, err := parseCredential(managed)
			if err != nil || doc.Identity.UserID != "a" {
				t.Fatal("external account leaked into managed credential")
			}
		})
	}
}

func TestCurrentRefreshRejectsAlreadyChangedOfficialIdentity(t *testing.T) {
	rig := newModuleRig(t)
	writeOfficial(t, rig.store, cred("a@example.com", "a"))
	id := rig.load().CurrentAccountID
	before, err := rig.store.LoadCredentialData(id)
	if err != nil {
		t.Fatal(err)
	}
	external := credSpec{email: "b@example.com", user: "b", expiresAt: expired}.json()
	writeOfficial(t, rig.store, external)
	_, err = rig.mod.ensureValidCredential(context.Background(), id, true)
	var mismatch *CredentialIdentityMismatchError
	if !errors.As(err, &mismatch) || len(rig.tokens.calls()) != 0 {
		t.Fatalf("drift not detected before request: %v", err)
	}
	managed, err := rig.store.LoadCredentialData(id)
	if err != nil || !bytes.Equal(managed, before) || !bytes.Equal(readOfficial(t, rig.store), external) {
		t.Fatal("drift changed credentials")
	}
}
