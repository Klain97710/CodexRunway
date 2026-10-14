package grok

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestParseCurrentOAuthCredentialKeepsBytes(t *testing.T) {
	data := []byte(`{
  "https://auth.x.ai::desktop-client": {
    "auth_mode": "oidc",
    "create_time": "2026-07-30T12:00:00Z",
    "email": "person@example.com",
    "expires_at": "2099-08-01T12:00:00Z",
    "key": "test-access-token-not-a-secret",
    "oidc_client_id": "desktop-client",
    "oidc_issuer": "https://auth.x.ai",
    "principal_id": "principal-123",
    "principal_type": "User",
    "refresh_token": "test-refresh-token-not-a-secret",
    "team_id": "team-456",
    "team_name": "Example Team",
    "user_id": "user-789"
  }
}`)
	doc, err := ParseAuthDocument(data)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(doc.RawData, data) {
		t.Error("raw bytes changed")
	}
	id := doc.Identity
	if id.Kind != AuthKindOAuth || id.Email != "person@example.com" || id.PrincipalID != "principal-123" || id.TeamID != "team-456" {
		t.Errorf("identity = %+v", id)
	}
	if !strings.HasPrefix(doc.StableID(), "grok-") {
		t.Errorf("stable id %q", doc.StableID())
	}
	if doc.RequiresReauthentication(time.Unix(1_800_000_000, 0)) {
		t.Error("a far-future token must not need re-authentication")
	}
}

func TestParseLegacySessionScope(t *testing.T) {
	doc, err := ParseAuthDocument([]byte(`{"https://accounts.x.ai/sign-in":{"auth_mode":"grok","email":"legacy@example.com","key":"k","user_id":"legacy-user"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if doc.Identity.Kind != AuthKindLegacySession || doc.Identity.UserID != "legacy-user" ||
		!equalStrings(doc.ManagedScopeKeys, []string{LegacyScope}) {
		t.Errorf("document = %+v", doc)
	}
}

func TestStableIDSurvivesLegacyToOAuthUpgrade(t *testing.T) {
	legacy, err := ParseAuthDocument([]byte(`{"https://accounts.x.ai/sign-in":{"auth_mode":"web_login","email":"upgrade@example.com","key":"k","user_id":"upgrade-user"}}`))
	if err != nil {
		t.Fatal(err)
	}
	oauth, err := ParseAuthDocument(credSpec{email: "upgrade@example.com", user: "upgrade-user", principal: "newly-enriched-principal"}.json())
	if err != nil {
		t.Fatal(err)
	}
	if legacy.StableID() != oauth.StableID() {
		t.Errorf("ids differ: %s vs %s", legacy.StableID(), oauth.StableID())
	}
}

func TestAPIKeyOnlyIsNotManaged(t *testing.T) {
	_, err := ParseAuthDocument([]byte(`{"xai::api_key":{"auth_mode":"api_key","key":"k","user_id":""}}`))
	if !errors.Is(err, ErrAuthNoManagedCredential) {
		t.Fatalf("err = %v", err)
	}
}

func TestReplaceManagedScopesKeepsUnmanaged(t *testing.T) {
	official := []byte(`{
  "https://auth.x.ai::old-client": {"auth_mode":"oidc","email":"old@example.com","key":"old-session","user_id":"old-user"},
  "xai::api_key": {"auth_mode":"api_key","key":"official-api-key"},
  "https://idp.example.com::custom-client": {"auth_mode":"oidc","key":"official-custom-token","user_id":"custom-user"}
}`)
	target := []byte(`{
  "https://auth.x.ai::new-client": {"auth_mode":"oidc","email":"new@example.com","key":"new-session","user_id":"new-user"},
  "https://accounts.x.ai/sign-in": {"auth_mode":"web_login","email":"stale@example.com","key":"stale-legacy","user_id":"stale-user"},
  "xai::api_key": {"auth_mode":"api_key","key":"target-api-key"}
}`)
	merged, err := ReplaceManagedScopes(official, target)
	if err != nil {
		t.Fatal(err)
	}
	root, err := acctParseObject(merged)
	if err != nil {
		t.Fatal(err)
	}
	key := func(scope string) string {
		o, _ := root[scope].(map[string]any)
		return authString(o["key"])
	}
	if _, old := root["https://auth.x.ai::old-client"]; old {
		t.Error("old managed scope kept")
	}
	if _, stale := root[LegacyScope]; stale {
		t.Error("target's legacy scope copied although the OAuth scope was selected")
	}
	if key("https://auth.x.ai::new-client") != "new-session" || key(APIKeyScope) != "official-api-key" ||
		key("https://idp.example.com::custom-client") != "official-custom-token" {
		t.Errorf("merged = %s", merged)
	}
}

func TestStoreKeepsCredentialsApartFromIndex(t *testing.T) {
	store := newTestStore(t)
	if err := os.MkdirAll(store.Root(), 0o700); err != nil {
		t.Fatal(err)
	}
	codexIndex := filepath.Join(store.Root(), "index.json")
	sentinel := []byte(`{"version":1,"sentinel":"codex"}`)
	if err := os.WriteFile(codexIndex, sentinel, 0o600); err != nil {
		t.Fatal(err)
	}
	credential := cred("stored@example.com", "stored-user")
	acc := mustUpsert(t, store, credential, true)

	if !strings.HasPrefix(acc.ID, "grok-") || filepath.Base(store.IndexPath()) != "grok-index.json" {
		t.Errorf("id %q index %q", acc.ID, store.IndexPath())
	}
	if got, err := store.LoadCredentialData(acc.ID); err != nil || !bytes.Equal(got, credential) {
		t.Errorf("credential differs (%v)", err)
	}
	idx, _ := store.LoadIndex()
	if idx.CurrentAccountID != acc.ID || len(idx.Accounts) != 1 {
		t.Errorf("index = %+v", idx)
	}
	if got, _ := os.ReadFile(codexIndex); !bytes.Equal(got, sentinel) {
		t.Error("the Codex index was modified")
	}
	text, _ := os.ReadFile(store.IndexPath())
	for _, secret := range []string{"access-stored-user", "refresh-stored-user"} {
		if strings.Contains(string(text), secret) {
			t.Errorf("index leaks %q", secret)
		}
	}
	if runtime.GOOS == "windows" {
		return
	}
	for path, want := range map[string]os.FileMode{
		store.Root(): 0o700, store.AccountDir(acc.ID): 0o700, store.CredentialPath(acc.ID): 0o600, store.IndexPath(): 0o600,
	} {
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != want {
			t.Errorf("%s mode = %v (%v), want %v", filepath.Base(path), info.Mode().Perm(), err, want)
		}
	}
}

func TestStoreDeduplicatesRotatedTokens(t *testing.T) {
	store := newTestStore(t)
	first := mustUpsert(t, store, credSpec{email: "same@example.com", user: "same-user", access: "first-access"}.json(), false)
	rotatedData := credSpec{email: "same@example.com", user: "same-user", access: "rotated-access"}.json()
	rotated := mustUpsert(t, store, rotatedData, false)
	idx, _ := store.LoadIndex()
	if rotated.ID != first.ID || len(idx.Accounts) != 1 {
		t.Fatalf("ids %s/%s, %d accounts", first.ID, rotated.ID, len(idx.Accounts))
	}
	if got, _ := store.LoadCredentialData(first.ID); !bytes.Equal(got, rotatedData) {
		t.Error("rotated credential not stored")
	}
}

func TestStoreRejectsCredentialOfAnotherIdentity(t *testing.T) {
	store := newTestStore(t)
	a := mustUpsert(t, store, cred("a@example.com", "user-a"), false)
	b := mustUpsert(t, store, cred("b@example.com", "user-b"), false)
	err := store.SaveCredentialData(a.ID, cred("b@example.com", "user-b"))
	var mismatch *CredentialIdentityMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("err = %v", err)
	}
	if got, _ := store.LoadCredentialData(b.ID); !bytes.Equal(got, cred("b@example.com", "user-b")) {
		t.Error("the other account changed")
	}
}

func TestOfficialCredentialStatus(t *testing.T) {
	store := newTestStore(t)
	if st := store.LoadOfficialCredentialStatus(testClock); st.Kind != OfficialMissing {
		t.Fatalf("status = %v", st.Kind)
	}
	writeOfficial(t, store, []byte(`{"xai::api_key":{"auth_mode":"api_key","key":"api-key-for-tests-only"}}`))
	if st := store.LoadOfficialCredentialStatus(testClock); st.Kind != OfficialAPIKeyOnly {
		t.Fatalf("status = %v", st.Kind)
	}
	writeOfficial(t, store, []byte("not json"))
	if st := store.LoadOfficialCredentialStatus(testClock); st.Kind != OfficialMalformed {
		t.Fatalf("status = %v", st.Kind)
	}
	writeOfficial(t, store, []byte(`{"https://auth.x.ai::desktop-client":{"auth_mode":"oidc","email":"expired@example.com","expires_at":"2020-01-01T00:00:00Z","key":"expired-access","user_id":"expired-user"}}`))
	st := store.LoadOfficialCredentialStatus(time.Unix(1_800_000_000, 0))
	if st.Kind != OfficialRequiresReauthentication || st.Identity == nil ||
		st.Identity.Email != "expired@example.com" || !st.Identity.HasAccessToken || st.Identity.HasRefreshToken {
		t.Fatalf("status = %+v", st)
	}
	if !st.HasManagedLogin() {
		t.Error("an expired login is still a managed login")
	}
}

func TestResolveOfficialHome(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX paths")
	}
	for _, c := range []struct{ env, want string }{
		{"/Volumes/isolated-grok", "/Volumes/isolated-grok"},
		{"", "/Users/example/.grok"},
		{"~/alternate-grok", "/Users/example/alternate-grok"},
		{"~", "/Users/example"},
	} {
		if got := ResolveOfficialHome(c.env, "/Users/example"); got != c.want {
			t.Errorf("ResolveOfficialHome(%q) = %q, want %q", c.env, got, c.want)
		}
	}
}

func TestIndexKeepsUnknownFields(t *testing.T) {
	store := newTestStore(t)
	acc := mustUpsert(t, store, cred("keep@example.com", "keep"), true)
	text, _ := os.ReadFile(store.IndexPath())
	edited := strings.Replace(string(text), `"version" : 1`, `"future" : {"x":[1,2]},`+"\n  "+`"version" : 1`, 1)
	edited = strings.Replace(edited, `"requiresReauth"`, `"badge" : "gold",`+"\n      "+`"requiresReauth"`, 1)
	if err := os.WriteFile(store.IndexPath(), []byte(edited), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.SetCurrentAccountID(acc.ID, testClock.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	out, _ := os.ReadFile(store.IndexPath())
	if !strings.Contains(string(out), `"future"`) || !strings.Contains(string(out), `"badge"`) {
		t.Errorf("unknown fields dropped:\n%s", out)
	}
}

func TestUnsupportedIndexVersion(t *testing.T) {
	store := newTestStore(t)
	if err := os.MkdirAll(store.Root(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.IndexPath(), []byte(`{"version":9,"accounts":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var unsupported *UnsupportedIndexVersionError
	if _, err := store.LoadIndex(); !errors.As(err, &unsupported) || unsupported.Version != 9 {
		t.Fatalf("err = %v", err)
	}
}

func TestReorderAndAlias(t *testing.T) {
	store := newTestStore(t)
	a := mustUpsert(t, store, cred("a@example.com", "a"), true)
	b := mustUpsert(t, store, cred("b@example.com", "b"), false)
	c := mustUpsert(t, store, cred("c@example.com", "c"), false)
	if err := store.Reorder([]string{c.ID, "unknown", a.ID}); err != nil {
		t.Fatal(err)
	}
	idx, _ := store.LoadIndex()
	var got []string
	for _, acc := range idx.OrderedAccounts() {
		got = append(got, acc.ID)
	}
	if want := []string{c.ID, a.ID, b.ID}; !equalStrings(got, want) {
		t.Errorf("order = %v, want %v", got, want)
	}
}

func TestSanitizedAccountDirNeverEscapes(t *testing.T) {
	store := newTestStore(t)
	for _, id := range []string{"..", ".", "../x", "a/b", `a\b`, ""} {
		dir := store.AccountDir(id)
		rel, err := filepath.Rel(store.Root(), dir)
		if err != nil || rel == ".." || strings.HasPrefix(rel, "../") || strings.ContainsAny(rel, `/\`) {
			t.Errorf("AccountDir(%q) = %q escapes the root", id, dir)
		}
	}
}

func TestAtomicIndexRewriteSkippedWhenUnchanged(t *testing.T) {
	store := newTestStore(t)
	mustUpsert(t, store, cred("x@example.com", "x"), true)
	before, _ := os.Stat(store.IndexPath())
	idx, _ := store.LoadIndex()
	time.Sleep(10 * time.Millisecond)
	if err := store.SaveIndex(idx); err != nil {
		t.Fatal(err)
	}
	after, _ := os.Stat(store.IndexPath())
	if !after.ModTime().Equal(before.ModTime()) {
		t.Error("an identical index was rewritten")
	}
}
