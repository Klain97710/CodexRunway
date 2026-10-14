package runway

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/licoy/quosparr/internal/codex"
)

// Every token below is synthetic; nothing here touches a real ~/.codex or
// ~/.quosparr.

const farFutureExp = 4_100_000_000

func jwtOf(t testing.TB, payload map[string]any) string {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	enc := base64.RawURLEncoding.EncodeToString
	return enc([]byte(`{"alg":"none"}`)) + "." + enc(body) + "."
}

// testAuth is an OAuth login for accountID whose tokens never expire.
func testAuth(t testing.TB, accountID, email, refresh string) codex.Auth {
	t.Helper()
	claims := map[string]any{"chatgpt_account_id": accountID, "chatgpt_plan_type": "plus"}
	id := jwtOf(t, map[string]any{"email": email, "https://api.openai.com/auth": claims})
	if len(refresh) < 20 {
		refresh += "xxxxxxxxxxxxxxxxxxxxxxxx"
	}
	return codex.Auth{
		AuthMode: "chatgpt",
		Tokens: codex.Tokens{
			IDToken: id, AccessToken: jwtOf(t, map[string]any{"exp": farFutureExp}),
			RefreshToken: refresh, AccountID: accountID,
		},
		PlanType: "plus",
	}
}

func writeOfficialAuth(t testing.TB, st *codex.Store, a codex.Auth) {
	t.Helper()
	data, err := a.MarshalFile(nil)
	if err != nil {
		t.Fatal(err)
	}
	p := st.OfficialAuthPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func tempStore(t testing.TB) *codex.Store {
	t.Helper()
	base := t.TempDir()
	return codex.NewStore(filepath.Join(base, "codex"), filepath.Join(base, "runway"))
}
