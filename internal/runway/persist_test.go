package runway

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/codex"
	"github.com/licoy/quosparr/internal/codexapi"
)

var persistNow = time.Unix(1_800_000_000, 0).UTC()

func persistFixture(t *testing.T) (st *codex.Store, cur, oth codex.ManagedAccount) {
	t.Helper()
	st = tempStore(t)
	curAuth := testAuth(t, "acct-cur", "cur@example.com", "refresh-cur")
	othAuth := testAuth(t, "acct-oth", "oth@example.com", "refresh-oth")
	var err error
	if oth, err = st.Upsert(othAuth, codex.UpsertOptions{Now: persistNow}); err != nil {
		t.Fatal(err)
	}
	if cur, err = st.Upsert(curAuth, codex.UpsertOptions{MakeActive: true, Now: persistNow}); err != nil {
		t.Fatal(err)
	}
	writeOfficialAuth(t, st, curAuth)
	return st, cur, oth
}

func TestPersisterCurrentAccountWritesOfficialAndManaged(t *testing.T) {
	st, cur, _ := persistFixture(t)
	p := TokenPersister{Store: st}
	access := jwtOf(t, map[string]any{"exp": farFutureExp, "n": 1})
	err := p.SaveRefreshed(context.Background(), cur.ID, codexapi.TokenSet{AccessToken: access, RefreshToken: "new-refresh-token-0123456789"}, persistNow)
	if err != nil {
		t.Fatal(err)
	}
	managed, _ := st.LoadCredential(cur.ID)
	official, _ := st.LoadOfficialAuth()
	for name, a := range map[string]codex.Auth{"managed": managed, "official": official} {
		if a.Tokens.AccessToken != access || a.Tokens.RefreshToken != "new-refresh-token-0123456789" {
			t.Errorf("%s copy not updated", name)
		}
	}
}

func TestPersisterOtherAccountNeverTouchesOfficial(t *testing.T) {
	st, cur, oth := persistFixture(t)
	before, _ := st.LoadOfficialAuth()
	p := TokenPersister{Store: st}
	rotated := jwtOf(t, map[string]any{"exp": farFutureExp, "n": 2})
	if err := p.SaveRefreshed(context.Background(), oth.ID, codexapi.TokenSet{AccessToken: rotated}, persistNow); err != nil {
		t.Fatal(err)
	}
	managed, _ := st.LoadCredential(oth.ID)
	if managed.Tokens.AccessToken != rotated {
		t.Error("managed copy not updated")
	}
	// A response without a new refresh token keeps the old one.
	if managed.Tokens.RefreshToken == "" {
		t.Error("refresh token lost")
	}
	after, _ := st.LoadOfficialAuth()
	if !before.Equal(after) {
		t.Error("official auth.json changed for a non-current account")
	}
	if cm, _ := st.LoadCredential(cur.ID); cm.Tokens.AccessToken == rotated {
		t.Error("current account leaked the other's token")
	}
}

func TestPersisterErrorsDoNotLeakTokens(t *testing.T) {
	st, _, _ := persistFixture(t)
	p := TokenPersister{Store: st}
	err := p.SaveRefreshed(context.Background(), "missing", codexapi.TokenSet{AccessToken: "secret-access-token"}, persistNow)
	if err == nil {
		t.Fatal("expected error")
	}
	if got := err.Error(); containsAny(got, "secret-access-token") {
		t.Errorf("error leaks token: %q", got)
	}
	if err := (TokenPersister{}).SaveRefreshed(context.Background(), "x", codexapi.TokenSet{}, persistNow); !errors.Is(err, ErrNoAccount) {
		t.Errorf("nil store: %v", err)
	}
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
	}
	return false
}
