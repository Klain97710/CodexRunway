package codex

import (
	"errors"
	"os"
	"testing"
)

func TestRefreshDoesNotUndoLogout(t *testing.T) {
	e := newEnv(t)
	auth := sampleAuth(t, sampleOpt{accountID: "one", email: "one@example.com", refresh: "before-refresh-credential"})
	acc := upsert(t, e.store, auth, true)
	e.writeOfficial(t, auth)
	noErr(t, os.Remove(e.officialPath()))
	auth = must(e.store.LoadCredential(acc.ID))
	auth.Tokens.RefreshToken = "rotated-managed-credential"
	result, err := e.store.WriteRefreshedCredential(acc.ID, auth, testNow)
	noErr(t, err)
	if !result.Managed || result.Official || !result.OfficialSkipped {
		t.Fatalf("refresh result = %+v", result)
	}
	if _, err := os.Stat(e.officialPath()); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("refresh recreated logged-out auth")
	}
	if got := must(e.store.LoadCredential(acc.ID)); !got.Equal(auth) {
		t.Fatal("rotated token was lost")
	}
}
