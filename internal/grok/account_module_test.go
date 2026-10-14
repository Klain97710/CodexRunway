package grok

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/platform"
)

// rotatingServer answers refresh requests with fresh tokens and records which
// refresh tokens were presented.
type rotatingServer struct {
	mu        sync.Mutex
	refreshed []string
	fail      int // status to answer with when non-zero
}

func (s *rotatingServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = r.ParseForm()
	s.refreshed = append(s.refreshed, r.PostForm.Get("refresh_token"))
	if s.fail != 0 {
		w.WriteHeader(s.fail)
		return
	}
	n := len(s.refreshed)
	fmt.Fprintf(w, `{"access_token":"fresh-%d","refresh_token":"rotated-%d","expires_in":3600}`, n, n)
}

func (s *rotatingServer) calls() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.refreshed...)
}

type moduleRig struct {
	t       *testing.T
	store   *Store
	mod     *Module
	tokens  *rotatingServer
	mu      sync.Mutex
	homes   []string
	billing func(ctx context.Context, home string) (json.RawMessage, time.Time, error)
}

func newModuleRig(t *testing.T) *moduleRig {
	t.Helper()
	rig := &moduleRig{t: t, store: newTestStore(t), tokens: &rotatingServer{}}
	rig.mod = NewModule(rig.store)
	rig.mod.Now = func() time.Time { return testClock }
	rig.mod.Refresher = TokenRefresher{HTTP: handlerClient(rig.tokens)}
	rig.mod.RunningProcessIDs = func(context.Context) ([]int, error) { return nil, nil }
	rig.mod.Billing = func(ctx context.Context, home string) (json.RawMessage, time.Time, error) {
		rig.mu.Lock()
		rig.homes = append(rig.homes, home)
		hook := rig.billing
		rig.mu.Unlock()
		if hook != nil {
			return hook(ctx, home)
		}
		return json.RawMessage(`{"plan":"SuperGrok","includedUsagePercent":12.5}`), testClock, nil
	}
	return rig
}

func (r *moduleRig) billedHomes() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.homes...)
}

func (r *moduleRig) load() AccountState {
	r.t.Helper()
	state, err := r.mod.Load(context.Background())
	if err != nil {
		r.t.Fatal(err)
	}
	return state
}

func (r *moduleRig) apply(cmd Command) (AccountState, error) {
	return r.mod.Apply(context.Background(), cmd)
}

var expired = "2020-01-01T00:00:00Z"

func TestLoadImportsOfficialIdentityAsCurrent(t *testing.T) {
	rig := newModuleRig(t)
	writeOfficial(t, rig.store, cred("first@example.com", "first-user"))
	state := rig.load()
	if state.Official.Kind != OfficialAuthenticated || state.CurrentAccountID == "" || len(state.Accounts) != 1 {
		t.Fatalf("state = %+v", state)
	}
	acc := state.Accounts[0]
	if acc.ID != state.CurrentAccountID || acc.Email() != "first@example.com" {
		t.Errorf("account = %+v", acc)
	}
	if data, err := rig.store.LoadCredentialData(acc.ID); err != nil || !bytes.Equal(data, cred("first@example.com", "first-user")) {
		t.Errorf("managed copy missing (%v)", err)
	}
}

func TestLoadWithoutOfficialLoginClearsCurrent(t *testing.T) {
	rig := newModuleRig(t)
	acc := mustUpsert(t, rig.store, cred("a@example.com", "a"), true)
	state := rig.load()
	if state.CurrentAccountID != "" || len(state.Accounts) != 1 || state.Accounts[0].ID != acc.ID {
		t.Errorf("state = %+v", state)
	}
	if st := state.Official.Kind; st != OfficialMissing {
		t.Errorf("status = %v", st)
	}
}

func TestExpiredNonCurrentTokenRefreshedWithoutTouchingOfficial(t *testing.T) {
	rig := newModuleRig(t)
	official := cred("current@example.com", "current")
	writeOfficial(t, rig.store, official)
	other := mustUpsert(t, rig.store, credSpec{email: "other@example.com", user: "other", refresh: "other-refresh", expiresAt: expired}.json(), false)
	rig.load()

	report := rig.mod.Refresh(context.Background(), RefreshTarget{Kind: RefreshAccount, ID: other.ID})
	if len(report.Outcomes) != 1 || report.Outcomes[0].Err != nil || report.Outcomes[0].Snapshot == nil {
		t.Fatalf("report = %+v", report)
	}
	if got := rig.tokens.calls(); len(got) != 1 || got[0] != "other-refresh" {
		t.Errorf("refresh calls = %v", got)
	}
	if !bytes.Equal(readOfficial(t, rig.store), official) {
		t.Error("the official auth.json changed while refreshing a non-current account")
	}
	managed, _ := rig.store.LoadCredentialData(other.ID)
	if fieldOf(t, managed, "key") != "fresh-1" || fieldOf(t, managed, "refresh_token") != "rotated-1" {
		t.Errorf("managed copy not renewed: %s", managed)
	}
	if homes := rig.billedHomes(); len(homes) != 1 || homes[0] != rig.store.AccountDir(other.ID) {
		t.Errorf("billing homes = %v, want the account's isolated directory", homes)
	}
	idx, _ := rig.store.LoadIndex()
	stored, _ := idx.Account(other.ID)
	if len(stored.CachedQuota) == 0 || stored.LastQuotaAt == nil || stored.LastError != "" || stored.RequiresReauth {
		t.Errorf("account = %+v", stored)
	}
}

func TestRefreshCurrentUpdatesOfficialAndManagedCopyAndKeepsOthersAlive(t *testing.T) {
	rig := newModuleRig(t)
	writeOfficial(t, rig.store, credSpec{email: "current@example.com", user: "current", refresh: "current-refresh", expiresAt: expired}.json())
	other := mustUpsert(t, rig.store, credSpec{email: "other@example.com", user: "other", refresh: "other-refresh", expiresAt: expired}.json(), false)
	state := rig.load()

	report := rig.mod.Refresh(context.Background(), RefreshTarget{Kind: RefreshCurrent})
	if len(report.Outcomes) != 1 || report.Outcomes[0].Err != nil || report.Outcomes[0].AccountID != state.CurrentAccountID {
		t.Fatalf("report = %+v", report)
	}
	if homes := rig.billedHomes(); len(homes) != 1 || homes[0] != rig.store.OfficialHome() {
		t.Errorf("billing homes = %v, want only the official home", homes)
	}
	official := readOfficial(t, rig.store)
	managed, _ := rig.store.LoadCredentialData(state.CurrentAccountID)
	if !bytes.Equal(official, managed) || fieldOf(t, official, "key") == "" || !strings.HasPrefix(fieldOf(t, official, "key"), "fresh-") {
		t.Errorf("official and managed copy should hold the same renewed credential")
	}
	// The non-current account was kept alive too, in its managed copy only.
	otherData, _ := rig.store.LoadCredentialData(other.ID)
	if !strings.HasPrefix(fieldOf(t, otherData, "key"), "fresh-") {
		t.Errorf("non-current token not kept alive: %s", otherData)
	}
	if got := rig.tokens.calls(); len(got) != 2 {
		t.Errorf("refresh calls = %v", got)
	}
	if strings.Contains(string(readOfficial(t, rig.store)), "other@example.com") {
		t.Error("another account leaked into the official file")
	}
}

func TestRefreshAllBillsCurrentFirstAndRetainsStaleQuota(t *testing.T) {
	rig := newModuleRig(t)
	writeOfficial(t, rig.store, cred("current@example.com", "current"))
	a := mustUpsert(t, rig.store, cred("a@example.com", "a"), false)
	b := mustUpsert(t, rig.store, cred("b@example.com", "b"), false)
	state := rig.load()
	if first := rig.mod.Refresh(context.Background(), RefreshTarget{Kind: RefreshAll}); len(first.Outcomes) != 3 || first.Outcomes[0].AccountID != state.CurrentAccountID {
		t.Fatalf("first report = %+v", first)
	}

	rig.billing = func(ctx context.Context, home string) (json.RawMessage, time.Time, error) {
		if home == rig.store.AccountDir(a.ID) {
			return nil, time.Time{}, errors.New("upstream said no")
		}
		return json.RawMessage(`{"plan":"new"}`), testClock.Add(time.Hour), nil
	}
	report := rig.mod.Refresh(context.Background(), RefreshTarget{Kind: RefreshAll})
	byID := map[string]RefreshOutcome{}
	for _, o := range report.Outcomes {
		byID[o.AccountID] = o
	}
	failed := byID[a.ID]
	var refreshFailed *RefreshFailedError
	if !errors.As(failed.Err, &refreshFailed) || refreshFailed.Message != ErrorCodeRefreshFailed || !failed.RetainedStaleSnapshot {
		t.Errorf("failed outcome = %+v", failed)
	}
	if byID[b.ID].Err != nil || byID[state.CurrentAccountID].Err != nil {
		t.Errorf("other outcomes failed: %+v", report.Outcomes)
	}
	idx, _ := rig.store.LoadIndex()
	stored, _ := idx.Account(a.ID)
	if stored.LastError != ErrorCodeRefreshFailed || stored.RequiresReauth || !strings.Contains(string(stored.CachedQuota), "SuperGrok") {
		t.Errorf("stale quota not retained: %+v", stored)
	}
	updated, _ := idx.Account(b.ID)
	if !strings.Contains(string(updated.CachedQuota), "new") || updated.LastError != "" {
		t.Errorf("successful account = %+v", updated)
	}
}

func TestBillingAuthFailureRetriesOnceAfterForcedRefresh(t *testing.T) {
	rig := newModuleRig(t)
	writeOfficial(t, rig.store, cred("current@example.com", "current"))
	other := mustUpsert(t, rig.store, credSpec{email: "o@example.com", user: "o", refresh: "o-refresh"}.json(), false)
	rig.load()
	var calls int
	rig.billing = func(ctx context.Context, home string) (json.RawMessage, time.Time, error) {
		calls++
		if calls == 1 {
			return nil, time.Time{}, ErrAuthenticationRequired
		}
		data, err := os.ReadFile(filepath.Join(home, "auth.json"))
		if err != nil || !strings.HasPrefix(fieldOf(t, data, "key"), "fresh-") {
			t.Errorf("retry did not see the renewed token (%v)", err)
		}
		return json.RawMessage(`{"plan":"retry"}`), testClock, nil
	}
	report := rig.mod.Refresh(context.Background(), RefreshTarget{Kind: RefreshAccount, ID: other.ID})
	if report.Outcomes[0].Err != nil || calls != 2 || len(rig.tokens.calls()) != 1 {
		t.Fatalf("outcome %+v calls %d refreshes %v", report.Outcomes[0], calls, rig.tokens.calls())
	}
}

func TestAuthenticationFailureFlagsReauthentication(t *testing.T) {
	rig := newModuleRig(t)
	writeOfficial(t, rig.store, cred("current@example.com", "current"))
	other := mustUpsert(t, rig.store, credSpec{email: "o@example.com", user: "o", refresh: "o-refresh", expiresAt: expired}.json(), false)
	rig.load()
	rig.tokens.fail = http.StatusBadRequest // invalid_grant style answer

	report := rig.mod.Refresh(context.Background(), RefreshTarget{Kind: RefreshAccount, ID: other.ID})
	out := report.Outcomes[0]
	var refreshFailed *RefreshFailedError
	if !errors.As(out.Err, &refreshFailed) || refreshFailed.Message != ErrorCodeAuthenticationNeeded {
		t.Fatalf("outcome = %+v", out)
	}
	idx, _ := rig.store.LoadIndex()
	stored, _ := idx.Account(other.ID)
	if !stored.RequiresReauth || stored.LastError != ErrorCodeAuthenticationNeeded {
		t.Errorf("account = %+v", stored)
	}
	if len(rig.billedHomes()) != 0 {
		t.Error("billing ran although the token could not be renewed")
	}
}

func TestBillingThatChangesManagedIdentityIsRolledBack(t *testing.T) {
	rig := newModuleRig(t)
	writeOfficial(t, rig.store, cred("current@example.com", "current"))
	originalData := cred("other@example.com", "other")
	other := mustUpsert(t, rig.store, originalData, false)
	rig.load()
	rig.billing = func(ctx context.Context, home string) (json.RawMessage, time.Time, error) {
		_ = os.WriteFile(filepath.Join(home, "auth.json"), cred("intruder@example.com", "intruder"), 0o600)
		return nil, time.Time{}, errors.New("cli exited")
	}
	report := rig.mod.Refresh(context.Background(), RefreshTarget{Kind: RefreshAccount, ID: other.ID})
	if report.Outcomes[0].Err == nil {
		t.Fatal("expected a failure")
	}
	if got, _ := rig.store.LoadCredentialData(other.ID); !bytes.Equal(got, originalData) {
		t.Error("the managed credential was not restored")
	}
}

func TestSuccessfulBillingCannotCommitAfterCorruptedCredential(t *testing.T) {
	rig := newModuleRig(t)
	writeOfficial(t, rig.store, cred("current@example.com", "current"))
	originalData := cred("other@example.com", "other")
	other := mustUpsert(t, rig.store, originalData, false)
	rig.load()
	rig.billing = func(ctx context.Context, home string) (json.RawMessage, time.Time, error) {
		_ = os.WriteFile(filepath.Join(home, "auth.json"), []byte("{corrupted"), 0o600)
		return json.RawMessage(`{"plan":"ghost"}`), testClock, nil
	}
	report := rig.mod.Refresh(context.Background(), RefreshTarget{Kind: RefreshAccount, ID: other.ID})
	if report.Outcomes[0].Err == nil || report.Outcomes[0].Snapshot != nil {
		t.Fatalf("outcome = %+v", report.Outcomes[0])
	}
	if got, _ := rig.store.LoadCredentialData(other.ID); !bytes.Equal(got, originalData) {
		t.Error("the managed credential was not restored")
	}
	idx, _ := rig.store.LoadIndex()
	if stored, _ := idx.Account(other.ID); strings.Contains(string(stored.CachedQuota), "ghost") {
		t.Error("a quota was committed after the credential was corrupted")
	}
}

func TestBillingAuthFailureWithoutRefreshTokenIsNotRetried(t *testing.T) {
	rig := newModuleRig(t)
	writeOfficial(t, rig.store, cred("current@example.com", "current"))
	other := mustUpsert(t, rig.store, []byte(`{"https://auth.x.ai::desktop-client":{"key":"k","user_id":"nr","email":"nr@example.com"}}`), false)
	rig.load()
	rig.billing = func(ctx context.Context, home string) (json.RawMessage, time.Time, error) {
		return nil, time.Time{}, ErrAuthenticationRequired
	}
	report := rig.mod.Refresh(context.Background(), RefreshTarget{Kind: RefreshAccount, ID: other.ID})
	if report.Outcomes[0].Err == nil || len(rig.tokens.calls()) != 0 {
		t.Fatalf("outcome %+v refreshes %v", report.Outcomes[0], rig.tokens.calls())
	}
	idx, _ := rig.store.LoadIndex()
	if stored, _ := idx.Account(other.ID); !stored.RequiresReauth {
		t.Error("account should need re-authentication")
	}
}

func TestRefreshUnknownAccount(t *testing.T) {
	rig := newModuleRig(t)
	report := rig.mod.Refresh(context.Background(), RefreshTarget{Kind: RefreshAccount, ID: "nope"})
	var missing *AccountNotFoundError
	if len(report.Outcomes) != 1 || !errors.As(report.Outcomes[0].Err, &missing) {
		t.Errorf("report = %+v", report)
	}
}

func TestRefreshWithoutBillingOnlyKeepsTokensFresh(t *testing.T) {
	rig := newModuleRig(t)
	rig.mod.Billing = nil
	writeOfficial(t, rig.store, credSpec{email: "c@example.com", user: "c", expiresAt: expired}.json())
	rig.load()
	report := rig.mod.Refresh(context.Background(), RefreshTarget{Kind: RefreshCurrent})
	if len(report.Outcomes) != 1 || report.Outcomes[0].Err != nil || report.Outcomes[0].Snapshot != nil {
		t.Fatalf("report = %+v", report)
	}
	if !strings.HasPrefix(fieldOf(t, readOfficial(t, rig.store), "key"), "fresh-") {
		t.Error("current token not renewed")
	}
}

func TestSwitchWarnsForRunningProcessesAndPreservesScopes(t *testing.T) {
	rig := newModuleRig(t)
	firstWithKey, err := mergeAPIKey(cred("first@example.com", "first-user"), "official-api-key-for-tests")
	if err != nil {
		t.Fatal(err)
	}
	writeOfficial(t, rig.store, firstWithKey)
	second := mustUpsert(t, rig.store, cred("second@example.com", "second-user"), false)
	rig.mod.RunningProcessIDs = func(context.Context) ([]int, error) { return []int{42}, nil }
	rig.load()

	_, err = rig.apply(Command{Kind: CommandMakeCurrent, ID: second.ID})
	var running *ProcessesRunningError
	if !errors.As(err, &running) || len(running.PIDs) != 1 || running.PIDs[0] != 42 {
		t.Fatalf("err = %v", err)
	}
	if !bytes.Equal(readOfficial(t, rig.store), firstWithKey) {
		t.Error("the official file changed although the switch was refused")
	}
	state, err := rig.apply(Command{Kind: CommandMakeCurrent, ID: second.ID, AllowWhileRunning: true})
	if err != nil {
		t.Fatal(err)
	}
	official := readOfficial(t, rig.store)
	doc, err := ParseAuthDocument(official)
	if err != nil || doc.StableID() != second.ID || state.CurrentAccountID != second.ID {
		t.Fatalf("installed %v (%v), current %q", doc.StableID(), err, state.CurrentAccountID)
	}
	root, _ := acctParseObject(official)
	apiKey, _ := root[APIKeyScope].(map[string]any)
	if authString(apiKey["key"]) != "official-api-key-for-tests" {
		t.Error("the API key scope was lost")
	}
	// The identity that was replaced stays in the library.
	idx, _ := rig.store.LoadIndex()
	if len(idx.Accounts) != 2 {
		t.Errorf("%d accounts after the switch", len(idx.Accounts))
	}
}

func mergeAPIKey(credential []byte, key string) ([]byte, error) {
	root, err := acctParseObject(credential)
	if err != nil {
		return nil, err
	}
	root[APIKeyScope] = map[string]any{"auth_mode": "api_key", "key": key}
	return acctMarshalJSON(root, acctSerialization, true)
}

func TestMakeCurrentRenewsExpiredTokenBeforeInstalling(t *testing.T) {
	rig := newModuleRig(t)
	writeOfficial(t, rig.store, cred("first@example.com", "first-user"))
	second := mustUpsert(t, rig.store, credSpec{email: "second@example.com", user: "second-user", expiresAt: expired}.json(), false)
	rig.load()
	if _, err := rig.apply(Command{Kind: CommandMakeCurrent, ID: second.ID}); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(fieldOf(t, readOfficial(t, rig.store), "key"), "fresh-") {
		t.Error("the installed token was not renewed first")
	}
}

func TestReapplyingCurrentOverwritesDriftedOfficial(t *testing.T) {
	rig := newModuleRig(t)
	firstData := cred("first@example.com", "first-user")
	writeOfficial(t, rig.store, firstData)
	first := mustUpsert(t, rig.store, firstData, true)
	writeOfficial(t, rig.store, cred("drifted@example.com", "drifted-user"))

	state, err := rig.apply(Command{Kind: CommandMakeCurrent, ID: first.ID, AllowWhileRunning: true})
	if err != nil {
		t.Fatal(err)
	}
	doc, _ := ParseAuthDocument(readOfficial(t, rig.store))
	if state.CurrentAccountID != first.ID || doc.StableID() != first.ID {
		t.Errorf("current %q installed %q", state.CurrentAccountID, doc.StableID())
	}
}

func TestHeldAuthLockLeavesIdentityAndIndexUnchanged(t *testing.T) {
	rig := newModuleRig(t)
	firstData := cred("first@example.com", "first-user")
	writeOfficial(t, rig.store, firstData)
	initial := rig.load()
	second := mustUpsert(t, rig.store, cred("second@example.com", "second-user"), false)
	lock, err := platform.AcquireLock(rig.store.OfficialAuthPath() + ".lock")
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Release()

	_, err = rig.apply(Command{Kind: CommandMakeCurrent, ID: second.ID, AllowWhileRunning: true})
	if !errors.Is(err, ErrAuthFileLocked) {
		t.Fatalf("err = %v", err)
	}
	idx, _ := rig.store.LoadIndex()
	if idx.CurrentAccountID != initial.CurrentAccountID || !bytes.Equal(readOfficial(t, rig.store), firstData) {
		t.Error("identity or index changed under a held lock")
	}
}

func TestCancelledSwitchAfterInspectionNeverWrites(t *testing.T) {
	rig := newModuleRig(t)
	firstData := cred("first@example.com", "first-user")
	writeOfficial(t, rig.store, firstData)
	initial := rig.load()
	second := mustUpsert(t, rig.store, cred("second@example.com", "second-user"), false)
	ctx, cancel := context.WithCancel(context.Background())
	rig.mod.RunningProcessIDs = func(context.Context) ([]int, error) { cancel(); return nil, nil }

	_, err := rig.mod.Apply(ctx, Command{Kind: CommandMakeCurrent, ID: second.ID})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	idx, _ := rig.store.LoadIndex()
	if !bytes.Equal(readOfficial(t, rig.store), firstData) || idx.CurrentAccountID != initial.CurrentAccountID {
		t.Error("a cancelled switch wrote credentials")
	}
}

func TestProcessInspectionFailureIsReported(t *testing.T) {
	rig := newModuleRig(t)
	writeOfficial(t, rig.store, cred("first@example.com", "first-user"))
	second := mustUpsert(t, rig.store, cred("second@example.com", "second-user"), false)
	rig.mod.RunningProcessIDs = func(context.Context) ([]int, error) { return nil, errors.New("ps failed") }
	_, err := rig.apply(Command{Kind: CommandMakeCurrent, ID: second.ID})
	var ioErr *AccountIOError
	if !errors.As(err, &ioErr) {
		t.Fatalf("err = %v", err)
	}
}

func TestExternalIdentityChangeIsReportedOnce(t *testing.T) {
	rig := newModuleRig(t)
	writeOfficial(t, rig.store, cred("first@example.com", "first-user"))
	first := rig.load()
	writeOfficial(t, rig.store, cred("second@example.com", "second-user"))

	// A refresh notices the change but leaves it for the next load.
	rig.mod.Billing = nil
	rig.mod.Refresh(context.Background(), RefreshTarget{Kind: RefreshCurrent})
	state := rig.load()
	if !state.OfficialIdentityChangedExternally || state.CurrentAccountID == first.CurrentAccountID || len(state.Accounts) != 2 {
		t.Fatalf("state = %+v", state)
	}
	if again := rig.load(); again.OfficialIdentityChangedExternally {
		t.Error("the external change was reported twice")
	}

	// An explicit identity command acknowledges a pending change.
	writeOfficial(t, rig.store, cred("first@example.com", "first-user"))
	acked, err := rig.apply(Command{Kind: CommandMakeCurrent, ID: state.CurrentAccountID, AllowWhileRunning: true})
	if err != nil {
		t.Fatal(err)
	}
	if acked.OfficialIdentityChangedExternally {
		t.Error("an identity command should acknowledge the change")
	}
	if rig.load().OfficialIdentityChangedExternally {
		t.Error("the acknowledged change came back")
	}
}

func TestSetAliasRemoveAndReorderCommands(t *testing.T) {
	rig := newModuleRig(t)
	writeOfficial(t, rig.store, cred("current@example.com", "current"))
	a := mustUpsert(t, rig.store, cred("a@example.com", "a"), false)
	b := mustUpsert(t, rig.store, cred("b@example.com", "b"), false)
	rig.load()

	state, err := rig.apply(Command{Kind: CommandSetAlias, ID: a.ID, Alias: "  Work  "})
	if err != nil {
		t.Fatal(err)
	}
	for _, acc := range state.Accounts {
		if acc.ID == a.ID && acc.Alias != "Work" {
			t.Errorf("alias = %q", acc.Alias)
		}
	}
	if state, err = rig.apply(Command{Kind: CommandSetAlias, ID: a.ID, Alias: "   "}); err != nil {
		t.Fatal(err)
	}
	for _, acc := range state.Accounts {
		if acc.ID == a.ID && acc.Alias != "" {
			t.Errorf("alias not cleared: %q", acc.Alias)
		}
	}
	if state, err = rig.apply(Command{Kind: CommandReorder, IDs: []string{b.ID, a.ID}}); err != nil {
		t.Fatal(err)
	}
	if state.Accounts[0].ID != b.ID || state.Accounts[1].ID != a.ID {
		t.Errorf("order = %v", []string{state.Accounts[0].ID, state.Accounts[1].ID})
	}
	if state, err = rig.apply(Command{Kind: CommandRemove, ID: b.ID}); err != nil || len(state.Accounts) != 2 {
		t.Fatalf("remove: %v, %d accounts", err, len(state.Accounts))
	}
	var cannot *CurrentAccountCannotBeRemovedError
	if _, err = rig.apply(Command{Kind: CommandRemove, ID: state.CurrentAccountID}); !errors.As(err, &cannot) {
		t.Errorf("err = %v", err)
	}
	var missing *AccountNotFoundError
	if _, err = rig.apply(Command{Kind: CommandSetAlias, ID: "nope", Alias: "x"}); !errors.As(err, &missing) {
		t.Errorf("err = %v", err)
	}
}

func TestImportOfficialCommand(t *testing.T) {
	rig := newModuleRig(t)
	if _, err := rig.apply(Command{Kind: CommandImportOfficial}); !errors.Is(err, ErrOfficialCredentialMissing) {
		t.Errorf("missing: %v", err)
	}
	writeOfficial(t, rig.store, []byte(`{"xai::api_key":{"auth_mode":"api_key","key":"k"}}`))
	if _, err := rig.apply(Command{Kind: CommandImportOfficial}); !errors.Is(err, ErrAPIKeyOnlyNotManageable) {
		t.Errorf("api key only: %v", err)
	}
	writeOfficial(t, rig.store, []byte("garbage"))
	if _, err := rig.apply(Command{Kind: CommandImportOfficial}); !errors.Is(err, ErrOfficialCredentialMalformed) {
		t.Errorf("malformed: %v", err)
	}
	writeOfficial(t, rig.store, cred("ok@example.com", "ok"))
	state, err := rig.apply(Command{Kind: CommandImportOfficial})
	if err != nil || len(state.Accounts) != 1 || state.CurrentAccountID == "" {
		t.Errorf("import: %v %+v", err, state)
	}
}

func TestImportPastedTextBootstrapsOfficialLogin(t *testing.T) {
	rig := newModuleRig(t)
	text := `{"email":"p@example.com","key":"pk","user_id":"pu","refresh_token":"pr"}`
	batch, state, err := rig.mod.ImportPastedText(context.Background(), text)
	if err != nil || batch.SuccessCount() != 1 || batch.FailureCount() != 0 {
		t.Fatalf("batch %+v err %v", batch, err)
	}
	if state.CurrentAccountID != batch.Succeeded[0].ID || state.Official.Kind != OfficialAuthenticated {
		t.Errorf("state = %+v", state)
	}
	if doc, err := ParseAuthDocument(readOfficial(t, rig.store)); err != nil || doc.StableID() != batch.Succeeded[0].ID {
		t.Errorf("official login not installed (%v)", err)
	}

	// Re-importing the same identity updates, never duplicates.
	batch2, state2, err := rig.mod.ImportPastedText(context.Background(), `{"email":"p@example.com","key":"pk2","user_id":"pu","refresh_token":"pr2"}`)
	if err != nil || batch2.SuccessCount() != 1 || len(state2.Accounts) != 1 {
		t.Fatalf("batch %+v accounts %d err %v", batch2, len(state2.Accounts), err)
	}
	if fieldOf(t, readOfficial(t, rig.store), "key") != "pk2" {
		t.Error("the current account's official credential was not refreshed from the paste")
	}
}

func TestImportPastedTextLeavesOfficialAloneForOtherAccounts(t *testing.T) {
	rig := newModuleRig(t)
	official := cred("current@example.com", "current")
	writeOfficial(t, rig.store, official)
	rig.load()
	batch, _, err := rig.mod.ImportPastedText(context.Background(),
		`[{"email":"x@example.com","key":"k1","user_id":"x"},{"email":"y@example.com","key":"k2","user_id":"y"}]`)
	if err != nil || batch.SuccessCount() != 2 {
		t.Fatalf("batch %+v err %v", batch, err)
	}
	if !bytes.Equal(readOfficial(t, rig.store), official) {
		t.Error("importing other accounts changed the official file")
	}
}

func TestImportPastedTextReportsUnusableInput(t *testing.T) {
	rig := newModuleRig(t)
	batch, _, err := rig.mod.ImportPastedText(context.Background(), "hello there")
	if err != nil || batch.SuccessCount() != 0 || !equalStrings(batch.Failures, []string{"no_credentials"}) {
		t.Errorf("batch %+v err %v", batch, err)
	}
	batch, _, _ = rig.mod.ImportPastedText(context.Background(), `{"xai::api_key":{"key":"k"}}`)
	if !equalStrings(batch.Failures, []string{"no_credentials"}) {
		t.Errorf("failures = %v", batch.Failures)
	}
}

func TestExportThenImportThroughModules(t *testing.T) {
	src := newModuleRig(t)
	src.mod.AppVersion = "9.9.9"
	a := mustUpsert(t, src.store, cred("a@example.com", "a"), false)
	b := mustUpsert(t, src.store, cred("b@example.com", "b"), false)
	result, err := src.mod.ExportAccounts([]string{a.ID, b.ID})
	if err != nil || result.ExportedCount != 2 {
		t.Fatalf("export: %v %+v", err, result)
	}
	path := filepath.Join(t.TempDir(), "pack.json")
	if err := WritePack(path, result.Pack); err != nil {
		t.Fatal(err)
	}

	dst := newModuleRig(t)
	preview, err := dst.mod.PreviewImportFiles(context.Background(), []string{path})
	if err != nil || len(preview.Items) != 2 || !preview.SourceIsPack {
		t.Fatalf("preview %+v err %v", preview, err)
	}
	if _, err := os.Stat(dst.store.IndexPath()); err == nil {
		t.Error("previewing wrote the library")
	}
	ids := []string{preview.Items[0].ID()}
	batch, state, err := dst.mod.ImportPreviewSelection(context.Background(), preview.Items, ids)
	if err != nil || batch.SuccessCount() != 1 || len(state.Accounts) != 1 {
		t.Fatalf("import: %+v err %v", batch, err)
	}
	empty, _, _ := dst.mod.ImportPreviewSelection(context.Background(), preview.Items, nil)
	if !equalStrings(empty.Failures, []string{FailureNothingSelected}) {
		t.Errorf("failures = %v", empty.Failures)
	}
	if _, err := src.mod.ExportAccounts(nil); err == nil {
		t.Error("an empty selection must be refused")
	}
}

func TestLoginOAuthMakesOnlyTheFirstAccountCurrent(t *testing.T) {
	rig := newModuleRig(t)
	login := func(email, sub string) {
		auth := newLoginRig(fmt.Sprintf(`{"access_token":"tok-%s","refresh_token":"ref-%s","email":%q,"sub":%q,"expires_in":3600}`, sub, sub, email, sub))
		rig.mod.Login = auth.login
		if _, err := rig.apply(Command{Kind: CommandLoginOAuth}); err != nil {
			t.Fatal(err)
		}
	}
	login("first@example.com", "sub-1")
	state := rig.load()
	firstID := state.CurrentAccountID
	if firstID == "" || state.Official.Kind != OfficialAuthenticated {
		t.Fatalf("state = %+v", state)
	}
	officialAfterFirst := readOfficial(t, rig.store)
	login("second@example.com", "sub-2")
	state = rig.load()
	if state.CurrentAccountID != firstID || len(state.Accounts) != 2 {
		t.Errorf("current %q, %d accounts", state.CurrentAccountID, len(state.Accounts))
	}
	if !bytes.Equal(readOfficial(t, rig.store), officialAfterFirst) {
		t.Error("a second login changed the official file")
	}
	assertNoPendingHomes(t, rig.store)
}

func TestLoginCancellationCleansPendingHome(t *testing.T) {
	rig := newModuleRig(t)
	auth := newLoginRig(`{"error":"authorization_pending"}`)
	ctx, cancel := context.WithCancel(context.Background())
	auth.login.Sleep = func(c context.Context, d time.Duration) error { cancel(); return c.Err() }
	rig.mod.Login = auth.login
	_, err := rig.mod.Apply(ctx, Command{Kind: CommandLoginOAuth})
	if !errors.Is(err, ErrLoginCancelled) {
		t.Fatalf("err = %v", err)
	}
	assertNoPendingHomes(t, rig.store)
}

func TestLoginFailureMessagesAreSafe(t *testing.T) {
	rig := newModuleRig(t)
	rig.mod.Login = newLoginRig(`{"error":"access_denied"}`).login
	_, err := rig.apply(Command{Kind: CommandLoginOAuth})
	var failed *LoginFailedError
	if !errors.As(err, &failed) || failed.Message != "Grok device authorization was denied." {
		t.Fatalf("err = %v", err)
	}
	assertNoPendingHomes(t, rig.store)
}

func assertNoPendingHomes(t *testing.T, s *Store) {
	t.Helper()
	entries, _ := os.ReadDir(s.Root())
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "login-") {
			t.Errorf("pending home %s left behind", e.Name())
		}
	}
}

func TestOperationsAreSerialized(t *testing.T) {
	rig := newModuleRig(t)
	writeOfficial(t, rig.store, cred("c@example.com", "c"))
	rig.load()
	var mu sync.Mutex
	active, peak := 0, 0
	rig.billing = func(ctx context.Context, home string) (json.RawMessage, time.Time, error) {
		mu.Lock()
		active++
		peak = max(peak, active)
		mu.Unlock()
		time.Sleep(20 * time.Millisecond)
		mu.Lock()
		active--
		mu.Unlock()
		return json.RawMessage(`{}`), testClock, nil
	}
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rig.mod.Refresh(context.Background(), RefreshTarget{Kind: RefreshCurrent})
		}()
	}
	wg.Wait()
	if peak != 1 {
		t.Errorf("%d refreshes ran at once", peak)
	}
}

func TestParseGrokProcessIDs(t *testing.T) {
	out := strings.Join([]string{
		"  101 /usr/local/bin/grok grok chat",
		"  102 /usr/bin/vim vim grok.txt",
		"  103 grok",
		"  104 /opt/Grok /opt/Grok",
		"   42 /usr/local/bin/grok self",
		"junk",
	}, "\n")
	got := parseGrokProcessIDs(out, 42)
	if fmt.Sprint(got) != "[101 103 104]" {
		t.Errorf("pids = %v", got)
	}
}
