package codex

import (
	"bytes"
	"context"
	"errors"
	"os"
	"reflect"
	"testing"
	"time"
)

const rotatedRefresh = "rotated-refresh-token-for-tests-only"

// switchFixture is a library with a current and a target account; the
// official file holds the current login.
type switchFixture struct {
	testEnv
	current, target ManagedAccount
	currentAuth     Auth
	targetAuth      Auth
}

func fixtureAuth(t testing.TB, id string, expired bool) Auth {
	exp := farFuture
	if expired {
		exp = 1
	}
	return Auth{AuthMode: "chatgpt", Tokens: Tokens{
		IDToken:      jwtOf(t, map[string]any{"email": id + "@example.com", "https://api.openai.com/auth": map[string]any{"chatgpt_account_id": id}}),
		AccessToken:  jwtOf(t, map[string]any{"exp": exp, "sub": "test-user-" + id, "pad": "padding-to-reach-forty-characters"}),
		RefreshToken: "test-refresh-token-for-" + id + "-only", AccountID: id,
	}}
}

func newSwitchFixture(t *testing.T, currentIsTarget, expired bool) switchFixture {
	t.Helper()
	f := switchFixture{testEnv: newEnv(t)}
	f.currentAuth = fixtureAuth(t, "current", currentIsTarget && expired)
	f.current = upsert(t, f.store, f.currentAuth, true)
	if currentIsTarget {
		f.target, f.targetAuth = f.current, f.currentAuth
	} else {
		f.targetAuth = fixtureAuth(t, "target", expired)
		f.target = upsert(t, f.store, f.targetAuth, false)
	}
	f.writeOfficial(t, f.currentAuth)
	minutes := 300
	cached := f.target.ApplyQuota(QuotaSummary{Plan: "plus", UpdatedAt: testNow, Primary: QuotaWindow{UsedPercent: 25, WindowMinutes: &minutes}})
	noErr(t, f.store.UpdateMetadata(cached))
	return f
}

func (f switchFixture) switcher(h SwitchHooks) *Switcher {
	return &Switcher{Store: f.store, Hooks: h, Now: func() time.Time { return testNow }}
}

// rotatingRefresh simulates a successful token endpoint.
func rotatingRefresh(t testing.TB, calls *int) func(context.Context, *Auth) error {
	return func(_ context.Context, a *Auth) error {
		*calls++
		body := `{"access_token":"` + jwtExp(t, farFuture) + `","refresh_token":"` + rotatedRefresh + `"}`
		return a.MergeRefreshResponse([]byte(body), testNow)
	}
}

func TestSwitchReappliesCurrentOverDriftedOfficial(t *testing.T) {
	e := newEnv(t)
	currentAuth := sampleAuth(t, sampleOpt{accountID: "acct-current", email: "current@example.com", refresh: "refresh-current"})
	driftedAuth := sampleAuth(t, sampleOpt{accountID: "acct-drifted", email: "drifted@example.com", refresh: "refresh-drifted"})
	current := upsert(t, e.store, currentAuth, true)
	e.writeOfficial(t, driftedAuth)

	validated := 0
	sw := &Switcher{Store: e.store, Now: func() time.Time { return testNow }, Hooks: SwitchHooks{
		Validate: func(context.Context, Auth) error { validated++; return nil },
	}}
	res := must(sw.SwitchTo(context.Background(), current.ID))

	if res.Account.ID != current.ID || validated != 1 {
		t.Fatalf("result %+v validated=%d", res.Account, validated)
	}
	idx := must(e.store.LoadIndex())
	official := must(e.store.LoadOfficialAuth())
	if idx.ActiveAccountID != current.ID || official.Tokens.AccountID != "acct-current" ||
		official.Tokens.RefreshToken != currentAuth.Tokens.RefreshToken {
		t.Fatal("official login was not reapplied")
	}
}

func TestSwitchInstallsTargetAndUpdatesIndex(t *testing.T) {
	f := newSwitchFixture(t, false, false)
	res := must(f.switcher(SwitchHooks{}).SwitchTo(context.Background(), f.target.ID))
	idx := must(f.store.LoadIndex())
	row, _ := idx.Account(f.target.ID)
	if idx.ActiveAccountID != f.target.ID || res.Account.ID != f.target.ID {
		t.Fatal("target not active")
	}
	if row.CachedQuota != nil || row.LastQuotaAt != nil || row.LastUsedAt == nil || row.RequiresReauth {
		t.Fatalf("row after switch: %+v", row)
	}
	if got := must(f.store.LoadOfficialAuth()); !got.Equal(must(f.store.LoadCredential(f.target.ID))) || got.Tokens.AccountID != "target" {
		t.Fatal("official file does not hold the target credential")
	}
}

func TestSwitchBetweenSameWorkspaceUsersKeepsSnapshots(t *testing.T) {
	e := newEnv(t)
	firstAuth := sampleAuth(t, sampleOpt{accountID: "workspace-shared", email: "one@example.com", refresh: "refresh-first-member", userID: "user-one"})
	secondAuth := sampleAuth(t, sampleOpt{accountID: "workspace-shared", email: "two@example.com", refresh: "refresh-second-member", userID: "user-two"})
	first := upsert(t, e.store, firstAuth, true)
	second := upsert(t, e.store, secondAuth, false)
	sw := &Switcher{Store: e.store, Now: func() time.Time { return testNow }}

	userOfficial := func() string {
		return DecodeIdentityClaims(must(e.store.LoadOfficialAuth()).Tokens.IDToken).UserID
	}
	must(sw.SwitchTo(context.Background(), second.ID))
	if userOfficial() != "user-two" {
		t.Fatal("not switched to the second user")
	}
	must(sw.SwitchTo(context.Background(), first.ID))
	if userOfficial() != "user-one" {
		t.Fatal("not switched back to the first user")
	}
	if must(e.store.LoadCredential(first.ID)).Tokens.RefreshToken != "refresh-first-member" ||
		must(e.store.LoadCredential(second.ID)).Tokens.RefreshToken != "refresh-second-member" {
		t.Fatal("managed snapshots changed")
	}
}

func TestRevokedTargetKeepsOfficialAndMarksReauth(t *testing.T) {
	f := newSwitchFixture(t, false, false)
	before := f.officialBytes(t)
	sw := f.switcher(SwitchHooks{Validate: func(context.Context, Auth) error { return ErrReauthRequired }})

	_, err := sw.SwitchTo(context.Background(), f.target.ID)
	if !errors.Is(err, ErrReauthRequired) {
		t.Fatalf("err = %v", err)
	}
	idx := must(f.store.LoadIndex())
	row, _ := idx.Account(f.target.ID)
	if !bytes.Equal(f.officialBytes(t), before) || idx.ActiveAccountID != f.current.ID || !row.RequiresReauth || row.CachedQuota != nil || row.LastQuotaAt != nil {
		t.Fatalf("revoked target: official changed=%v active=%s row=%+v", !bytes.Equal(f.officialBytes(t), before), idx.ActiveAccountID, row)
	}
}

func TestRefreshSuccessThenValidationFailureStaysSynchronized(t *testing.T) {
	validationErrors := map[string]error{
		"timeout":      context.DeadlineExceeded,
		"server error": errors.New("quota status 500"),
	}
	for name, validationErr := range validationErrors {
		for _, currentIsTarget := range []bool{true, false} {
			t.Run(name+map[bool]string{true: "/current", false: "/other"}[currentIsTarget], func(t *testing.T) {
				f := newSwitchFixture(t, currentIsTarget, true)
				officialBefore := f.officialBytes(t)
				calls := 0
				sw := f.switcher(SwitchHooks{
					Refresh:  rotatingRefresh(t, &calls),
					Validate: func(context.Context, Auth) error { return validationErr },
				})
				_, err := sw.SwitchTo(context.Background(), f.target.ID)
				if !errors.Is(err, validationErr) {
					t.Fatalf("err = %v", err)
				}
				managed := must(f.store.LoadCredential(f.target.ID))
				if managed.Tokens.RefreshToken != rotatedRefresh {
					t.Fatal("rotated refresh token was lost")
				}
				if currentIsTarget {
					if !must(f.store.LoadOfficialAuth()).Equal(managed) {
						t.Fatal("official file out of sync with the rotated managed copy")
					}
				} else if !bytes.Equal(f.officialBytes(t), officialBefore) {
					t.Fatal("a non-current refresh touched the official file")
				}
				idx := must(f.store.LoadIndex())
				row, _ := idx.Account(f.target.ID)
				if idx.ActiveAccountID != f.current.ID || row.RequiresReauth {
					t.Fatalf("active=%s reauth=%v", idx.ActiveAccountID, row.RequiresReauth)
				}
			})
		}
	}
}

func TestRefreshFailuresLeaveEverythingUnchanged(t *testing.T) {
	for _, currentIsTarget := range []bool{true, false} {
		f := newSwitchFixture(t, currentIsTarget, true)
		officialBefore := f.officialBytes(t)
		managedBefore := must(f.store.LoadCredential(f.target.ID))
		indexBefore := must(os.ReadFile(f.store.IndexPath()))
		refreshErr := errors.New("token endpoint HTTP 503")
		calls := 0
		sw := f.switcher(SwitchHooks{
			Refresh:  func(context.Context, *Auth) error { calls++; return refreshErr },
			Validate: func(context.Context, Auth) error { t.Fatal("validate must not run after a failed refresh"); return nil },
		})
		if _, err := sw.SwitchTo(context.Background(), f.target.ID); !errors.Is(err, refreshErr) {
			t.Fatalf("err = %v", err)
		}
		if calls != 1 || !bytes.Equal(f.officialBytes(t), officialBefore) ||
			!must(f.store.LoadCredential(f.target.ID)).Equal(managedBefore) ||
			!bytes.Equal(must(os.ReadFile(f.store.IndexPath())), indexBefore) {
			t.Fatalf("state changed after a temporary refresh failure (currentIsTarget=%v)", currentIsTarget)
		}
	}
}

func TestRefusedRefreshMarksReauth(t *testing.T) {
	f := newSwitchFixture(t, false, true)
	before := f.officialBytes(t)
	sw := f.switcher(SwitchHooks{Refresh: func(context.Context, *Auth) error { return ErrReauthRequired }})
	if _, err := sw.SwitchTo(context.Background(), f.target.ID); !errors.Is(err, ErrReauthRequired) {
		t.Fatalf("err = %v", err)
	}
	row, _ := must(f.store.LoadIndex()).Account(f.target.ID)
	if !row.RequiresReauth || row.CachedQuota != nil || !bytes.Equal(f.officialBytes(t), before) {
		t.Fatalf("row: %+v", row)
	}
}

func TestExpiredTargetWithoutRefreshAbility(t *testing.T) {
	e := newEnv(t)
	expired := jwtOf(t, map[string]any{"exp": 1_700_000_000, "email": "x@y.z", "pad": "padding-to-reach-forty-characters"})
	session := Auth{AuthMode: "chatgpt", Tokens: Tokens{IDToken: expired, AccessToken: expired, AccountID: "sess"}}
	acc := upsert(t, e.store, session, false)
	sw := &Switcher{Store: e.store, Now: func() time.Time { return testNow }}
	if _, err := sw.SwitchTo(context.Background(), acc.ID); !errors.Is(err, ErrExpiredAccessWithoutRefresh) {
		t.Fatalf("err = %v", err)
	}

	f := newSwitchFixture(t, false, true)
	_, err := f.switcher(SwitchHooks{}).SwitchTo(context.Background(), f.target.ID)
	if !errors.Is(err, ErrRefreshUnavailable) {
		t.Fatalf("err = %v", err)
	}
}

func TestSwitchUnknownAccount(t *testing.T) {
	f := newSwitchFixture(t, false, false)
	_, err := f.switcher(SwitchHooks{}).SwitchTo(context.Background(), "ghost")
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestPersistenceFailureIsReportedWithoutLosingTheAuthError(t *testing.T) {
	for _, name := range []string{"corrupt index", "missing index"} {
		t.Run(name, func(t *testing.T) {
			f := newSwitchFixture(t, false, false)
			var reports []error
			sw := f.switcher(SwitchHooks{
				Validate: func(context.Context, Auth) error {
					if name == "corrupt index" {
						noErr(t, os.WriteFile(f.store.IndexPath(), []byte("invalid index"), 0o600))
					} else {
						noErr(t, os.Remove(f.store.IndexPath()))
					}
					return ErrReauthRequired
				},
				OnPersistError: func(err error) { reports = append(reports, err) },
			})
			_, err := sw.SwitchTo(context.Background(), f.target.ID)
			if !errors.Is(err, ErrReauthRequired) {
				t.Fatalf("original error lost: %v", err)
			}
			if len(reports) != 1 {
				t.Fatalf("reports = %d", len(reports))
			}
		})
	}
}

// snapshotAll captures the files a switch commits.
func snapshotAll(f switchFixture, id string) map[string][]byte {
	out := map[string][]byte{}
	for _, p := range []string{f.store.OfficialAuthPath(), f.store.CredentialPath(id), f.store.IndexPath(), f.store.ConfigPath()} {
		out[p], _ = os.ReadFile(p)
	}
	return out
}

func TestSwitchRollsBackOnEveryCommitStage(t *testing.T) {
	for _, stage := range []string{"official", "credential", "verify", "config", "index"} {
		t.Run(stage, func(t *testing.T) {
			f := newSwitchFixture(t, false, false)
			before := snapshotAll(f, f.target.ID)
			boom := errors.New("injected " + stage + " failure")
			sw := f.switcher(SwitchHooks{})
			sw.failpoint = func(s string) error {
				if s == stage {
					return boom
				}
				return nil
			}
			if _, err := sw.SwitchTo(context.Background(), f.target.ID); !errors.Is(err, boom) || errors.Is(err, ErrRollbackFailed) {
				t.Fatalf("err = %v", err)
			}
			if after := snapshotAll(f, f.target.ID); !reflect.DeepEqual(before, after) {
				t.Fatal("files differ from the pre-switch state after rollback")
			}
		})
	}
}

func TestSwitchRollbackRemovesFilesThatDidNotExist(t *testing.T) {
	f := newSwitchFixture(t, false, false)
	noErr(t, os.Remove(f.officialPath()))
	boom := errors.New("injected failure")
	sw := f.switcher(SwitchHooks{})
	sw.failpoint = func(s string) error {
		if s == "index" {
			return boom
		}
		return nil
	}
	if _, err := sw.SwitchTo(context.Background(), f.target.ID); !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(f.officialPath()); !os.IsNotExist(err) {
		t.Fatal("a failed switch left a new official auth.json behind")
	}
}

func TestSwitchRollbackFailureIsReported(t *testing.T) {
	f := newSwitchFixture(t, false, false)
	boom := errors.New("injected failure")
	sw := f.switcher(SwitchHooks{})
	sw.failpoint = func(s string) error {
		if s != "index" {
			return nil
		}
		// Make the official path unrestorable: replace the file with a
		// directory so the atomic rename back fails.
		noErr(t, os.Remove(f.officialPath()))
		noErr(t, os.Mkdir(f.officialPath(), 0o700))
		noErr(t, os.WriteFile(f.officialPath()+"/keep", []byte("x"), 0o600))
		return boom
	}
	_, err := sw.SwitchTo(context.Background(), f.target.ID)
	if !errors.Is(err, boom) || !errors.Is(err, ErrRollbackFailed) {
		t.Fatalf("err = %v", err)
	}
}

func TestEnsureValidCredentialRefreshesOnlyWhenNeeded(t *testing.T) {
	f := newSwitchFixture(t, false, false)
	calls := 0
	sw := f.switcher(SwitchHooks{Refresh: rotatingRefresh(t, &calls)})
	if auth := must(sw.EnsureValidCredential(context.Background(), f.target.ID)); calls != 0 || auth.Tokens.RefreshToken != f.targetAuth.Tokens.RefreshToken {
		t.Fatal("a valid credential must not be refreshed")
	}

	expired := newSwitchFixture(t, false, true)
	calls = 0
	officialBefore := expired.officialBytes(t)
	auth := must(expired.switcher(SwitchHooks{Refresh: rotatingRefresh(t, &calls)}).EnsureValidCredential(context.Background(), expired.target.ID))
	if calls != 1 || auth.Tokens.RefreshToken != rotatedRefresh ||
		must(expired.store.LoadCredential(expired.target.ID)).Tokens.RefreshToken != rotatedRefresh ||
		!bytes.Equal(expired.officialBytes(t), officialBefore) {
		t.Fatal("refresh was not persisted to the managed copy only")
	}
}
