package codex

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// All tokens in these tests are synthetic; nothing here touches a real
// ~/.codex or ~/.quosparr.

var testNow = time.Unix(1_800_000_000, 0).UTC()

const farFuture = 4_100_000_000

func b64url(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

// jwtOf builds an unsigned JWT with the given payload claims.
func jwtOf(t testing.TB, payload map[string]any) string {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return b64url([]byte(`{"alg":"none"}`)) + "." + b64url(body) + "."
}

func jwtExp(t testing.TB, exp int) string { return jwtOf(t, map[string]any{"exp": exp}) }

type sampleOpt struct {
	accountID, email, refresh string
	plan, userID, subject     string
	claimAccountID            string
	workspaceName             string
}

// sampleAuth mirrors the Swift tests' fixture: an OAuth login whose id token
// carries the identity claims and whose access token is valid for decades.
func sampleAuth(t testing.TB, o sampleOpt) Auth {
	t.Helper()
	if o.plan == "" {
		o.plan = "plus"
	}
	claimed := o.claimAccountID
	if claimed == "" {
		claimed = o.accountID
	}
	authClaims := map[string]any{"chatgpt_account_id": claimed, "chatgpt_plan_type": o.plan}
	if o.userID != "" {
		authClaims["chatgpt_user_id"] = o.userID
	}
	if o.workspaceName != "" {
		authClaims["organizations"] = []any{map[string]any{
			"id": "organization-" + o.accountID, "is_default": true, "title": o.workspaceName,
		}}
	}
	payload := map[string]any{"email": o.email, "https://api.openai.com/auth": authClaims}
	if o.subject != "" {
		payload["sub"] = o.subject
	}
	refresh := o.refresh
	if len([]rune(refresh)) < 20 {
		refresh += "xxxxxxxxxxxxxxxxxxxxxxxx"
	}
	return Auth{
		AuthMode: "chatgpt",
		Tokens: Tokens{
			IDToken: jwtOf(t, payload), AccessToken: jwtExp(t, farFuture),
			RefreshToken: refresh, AccountID: o.accountID,
		},
		PlanType: o.plan,
	}
}

// testEnv is an isolated codex home and runway home.
type testEnv struct {
	codexHome, runwayHome string
	store                 *Store
}

func newEnv(t testing.TB) testEnv {
	t.Helper()
	base := t.TempDir()
	e := testEnv{codexHome: filepath.Join(base, "codex"), runwayHome: filepath.Join(base, "runway")}
	e.store = NewStore(e.codexHome, e.runwayHome)
	return e
}

func (e testEnv) officialPath() string { return e.store.OfficialAuthPath() }

func (e testEnv) writeOfficial(t testing.TB, a Auth) {
	t.Helper()
	if err := e.store.writeOfficial(a, testNow); err != nil {
		t.Fatal(err)
	}
}

func (e testEnv) officialBytes(t testing.TB) []byte {
	t.Helper()
	data, err := os.ReadFile(e.officialPath())
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// must returns v or fails the test by panicking, which `go test` reports
// with a stack that names the call site.
func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func noErr(t testing.TB, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func upsert(t testing.TB, s *Store, a Auth, active bool) ManagedAccount {
	t.Helper()
	return must(s.Upsert(a, UpsertOptions{MakeActive: active, Now: testNow}))
}

func ctxBG() context.Context { return context.Background() }
