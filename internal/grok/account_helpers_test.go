package grok

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Everything below uses synthetic tokens in t.TempDir; nothing touches the
// real ~/.grok, ~/.codex or ~/.quosparr.

var testClock = time.Unix(1_785_499_200, 0)

type credSpec struct {
	email, user, principal string
	access, refresh        string
	expiresAt              string
	team                   string
	extra                  map[string]any
}

func (c credSpec) json() []byte {
	if c.principal == "" {
		c.principal = "principal-" + c.user
	}
	if c.access == "" {
		c.access = "access-" + c.user
	}
	if c.refresh == "" {
		c.refresh = "refresh-" + c.user
	}
	if c.expiresAt == "" {
		c.expiresAt = "2099-08-01T12:00:00Z"
	}
	body := map[string]any{
		"auth_mode": "oidc", "email": c.email, "expires_at": c.expiresAt, "key": c.access,
		"oidc_client_id": "desktop-client", "oidc_issuer": "https://auth.x.ai", "principal_id": c.principal,
		"principal_type": "User", "refresh_token": c.refresh, "user_id": c.user,
	}
	if c.team != "" {
		body["team_id"], body["team_name"] = c.team, "Team "+c.team
	}
	for k, v := range c.extra {
		body[k] = v
	}
	data, err := acctMarshalJSON(map[string]any{OAuthScopePrefix + "desktop-client": body}, acctSerialization, true)
	if err != nil {
		panic(err)
	}
	return data
}

func cred(email, user string) []byte { return credSpec{email: email, user: user}.json() }

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	return NewStoreAt(filepath.Join(dir, "accounts"), filepath.Join(dir, "official"))
}

func writeOfficial(t *testing.T, s *Store, data []byte) {
	t.Helper()
	if err := os.MkdirAll(s.OfficialHome(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.OfficialAuthPath(), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func readOfficial(t *testing.T, s *Store) []byte {
	t.Helper()
	data, err := os.ReadFile(s.OfficialAuthPath())
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func mustUpsert(t *testing.T, s *Store, data []byte, current bool) ManagedAccount {
	t.Helper()
	acc, err := s.UpsertCredentialData(data, UpsertOptions{MakeCurrent: current, Now: testClock})
	if err != nil {
		t.Fatal(err)
	}
	return acc
}

func fieldOf(t *testing.T, data []byte, field string) string {
	t.Helper()
	doc, err := ParseAuthDocument(data)
	if err != nil {
		t.Fatal(err)
	}
	root, err := authRoot(data)
	if err != nil {
		t.Fatal(err)
	}
	object, _ := root[doc.Identity.Scope].(map[string]any)
	return authString(object[field])
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
