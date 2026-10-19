package runway

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/apiquota"
	"github.com/licoy/quosparr/internal/codex"
	"github.com/licoy/quosparr/internal/codexapi"
	"github.com/licoy/quosparr/internal/export"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/store"
	"github.com/licoy/quosparr/internal/usage"
)

// Every key below is synthetic and never printed by a failing assertion.
const (
	relayKeyA = "sk-relay-test-aaaaaaaaaaaaaaaaaaaa"
	relayKeyB = "sk-relay-test-bbbbbbbbbbbbbbbbbbbb"
	relayKeyC = "sk-relay-test-cccccccccccccccccccc"
	relayKeyD = "sk-relay-test-dddddddddddddddddddd"
	relayKeyE = "sk-relay-test-eeeeeeeeeeeeeeeeeeee"
	plainKey  = "sk-plain-test-pppppppppppppppppppp"

	relayBaseA   = "https://relay-a.example.com"
	relayBaseAV1 = "https://relay-a.example.com/v1"
)

// relayCall is one probe the fake site received.
type relayCall struct{ base, key string }

// relayEnv is a temp account library plus a fake relay site.
type relayEnv struct {
	st *codex.Store
	im *codex.Importer

	mu       sync.Mutex
	calls    []relayCall
	keyLoads map[string]int
	inflight int
	peak     int
	started  chan struct{} // receives one value per probe that starts, when non-nil
	hold     chan struct{} // probes block until it is closed, when non-nil
	probe    func(ctx context.Context, base, key string) apiquota.Quota
}

func newRelayEnv(t *testing.T) *relayEnv {
	t.Helper()
	st := tempStore(t)
	return &relayEnv{
		st:       st,
		im:       &codex.Importer{Store: st, Now: func() time.Time { return t0 }},
		keyLoads: map[string]int{},
		probe: func(context.Context, string, string) apiquota.Quota {
			return usdQuota(12.34, 20)
		},
	}
}

func usdQuota(remaining, total float64) apiquota.Quota {
	used := total - remaining
	return apiquota.Quota{
		Status: apiquota.StatusAvailable, Remaining: &remaining, Used: &used, Total: &total,
		Unit: apiquota.UnitUSD, Source: apiquota.SourceTokenUsage, FetchedAt: t0,
	}
}

func (e *relayEnv) addRelay(t *testing.T, key, base string, active bool) codex.ManagedAccount {
	t.Helper()
	acc, err := e.im.ImportRelayAPIKey(key, codex.RelayProfile{BaseURL: base}, "", active)
	if err != nil {
		t.Fatal(err)
	}
	return acc
}

func (e *relayEnv) addPlain(t *testing.T, key string, active bool) codex.ManagedAccount {
	t.Helper()
	acc, err := e.im.ImportRelayAPIKey(key, codex.RelayProfile{}, "", active)
	if err != nil {
		t.Fatal(err)
	}
	return acc
}

func (e *relayEnv) addOAuth(t *testing.T, auth codex.Auth, active bool) codex.ManagedAccount {
	t.Helper()
	acc, err := e.st.Upsert(auth, codex.UpsertOptions{MakeActive: active, Now: t0})
	if err != nil {
		t.Fatal(err)
	}
	return acc
}

func (e *relayEnv) row(t *testing.T, id string) codex.ManagedAccount {
	t.Helper()
	idx, err := e.st.LoadIndex()
	if err != nil {
		t.Fatal(err)
	}
	acc, ok := idx.Account(id)
	if !ok {
		t.Fatalf("account %s missing", id)
	}
	return acc
}

func (e *relayEnv) fetch(ctx context.Context, base, key string) apiquota.Quota {
	e.mu.Lock()
	e.calls = append(e.calls, relayCall{base, key})
	e.inflight++
	e.peak = max(e.peak, e.inflight)
	started, hold, probe := e.started, e.hold, e.probe
	e.mu.Unlock()
	defer func() {
		e.mu.Lock()
		e.inflight--
		e.mu.Unlock()
	}()
	if started != nil {
		started <- struct{}{}
	}
	if hold != nil {
		select {
		case <-hold:
		case <-ctx.Done():
		}
	}
	return probe(ctx, base, key)
}

func (e *relayEnv) loadKey(id string) (string, error) {
	e.mu.Lock()
	e.keyLoads[id]++
	e.mu.Unlock()
	auth, err := e.st.LoadCredential(id)
	if err != nil {
		return "", err
	}
	return auth.APIKey(), nil
}

func (e *relayEnv) callList() []relayCall {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]relayCall(nil), e.calls...)
}

func (e *relayEnv) loads(id string) int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.keyLoads[id]
}

func (e *relayEnv) apply() func(*Options) {
	return func(o *Options) {
		o.Sources.LoadIndex = e.st.LoadIndex
		o.Sources.SaveAccount = e.st.UpdateMetadata
		o.Sources.FetchAPIQuota = e.fetch
		o.Sources.LoadAPIKey = e.loadKey
	}
}

// noChatGPT fails the test when any ChatGPT backend read happened.
func noChatGPT(t *testing.T, f *fake) {
	t.Helper()
	for _, name := range []string{"quota", "credits", "daily", "profile"} {
		if n := f.count(name); n != 0 {
			t.Errorf("%s was requested %d times in API key mode", name, n)
		}
	}
}

// ---- key mode ---------------------------------------------------------------

func TestPlainAPIKeyMakesNoChatGPTRequestAnywhere(t *testing.T) {
	f := newFake(t)
	e := newRelayEnv(t)
	plain := e.addPlain(t, plainKey, true)
	f.setAuth(codex.APIKeyAuth(plainKey), nil)
	s, _ := newService(t, f, func(o *Options) {
		e.apply()(o)
		o.Preferences.ShowsQuotaEstimateSummary = true
	})
	showPanel(s)
	c := refreshAll(t, s).Codex

	wait(t, s.RefreshQuota())
	wait(t, s.RefreshResetCredits())
	wait(t, s.RefreshQuotaEstimate())
	wait(t, s.RefreshCost(usage.RefreshForce))
	wait(t, s.RefreshTokenHeatmap(usage.RefreshForce))
	wait(t, s.RefreshAllAccountQuotas())
	wait(t, s.RefreshAccountQuota(plain.ID))
	settle(t, s)

	ctx := context.Background()
	if _, err := s.QueryCurrentCycleCost(ctx); err != nil {
		t.Errorf("current cycle cost: %v", err)
	}
	if _, err := s.ResolvePreviousCycleRange(ctx); err != nil {
		t.Errorf("previous cycle: %v", err)
	}
	if _, err := s.QueryCost(ctx, usage.ThisMonth(t0, time.UTC)); err != nil {
		t.Errorf("range cost: %v", err)
	}
	settle(t, s)

	noChatGPT(t, f)
	if len(e.callList()) != 0 || e.loads(plain.ID) != 0 {
		t.Error("a plain API key probed a relay site")
	}
	if f.count("scan") == 0 {
		t.Error("local scans did not run")
	}
	if !c.Authenticated || !c.APIKeyMode || c.APIBaseURL != "" || c.APIBalance != nil {
		t.Errorf("view: %+v", c)
	}
	if c.StatusText != "API" || c.QuotaText != "API" || len(c.Meters) != 0 || c.Quota != nil {
		t.Errorf("status %q quota %q meters %d", c.StatusText, c.QuotaText, len(c.Meters))
	}
	if c.LastError != "" || c.EstimateError != "" || len(c.QuotaLines) != 0 {
		t.Errorf("a plain key reported an error: %q %q %v", c.LastError, c.EstimateError, c.QuotaLines)
	}
	if c.ResetCredits != nil || c.Estimate != nil {
		t.Error("ChatGPT-only sections were filled")
	}
	if c.Cost == nil || c.DisplayedCost == nil || !strings.Contains(c.CostText, "$2.0000") {
		t.Errorf("local cost missing: %q", c.CostText)
	}
	if c.Heatmap.CalculatedAt.IsZero() || len(c.Heatmap.AllDevices) != 0 {
		t.Errorf("heatmap: %+v", c.Heatmap)
	}
}

func TestAPIKeyCostWithoutLocalDataIsUnavailableNotOnline(t *testing.T) {
	f := newFake(t)
	e := newRelayEnv(t)
	e.addPlain(t, plainKey, true)
	f.setAuth(codex.APIKeyAuth(plainKey), nil)
	f.scanFn = func(context.Context, []usage.Query) (map[string]usage.Summary, error) {
		return nil, os.ErrNotExist
	}
	s, _ := newService(t, f, e.apply())
	showPanel(s)
	c := refreshAll(t, s).Codex
	wait(t, s.RefreshCost(usage.RefreshForce))
	if _, err := s.QueryCost(context.Background(), usage.ThisMonth(t0, time.UTC)); err == nil {
		t.Error("a range without local data resolved")
	}
	noChatGPT(t, f)
	if c.LastError != "" || c.CostText != en.Text(l10n.KeyUsageAnalyticsUnavailable) {
		t.Errorf("cost %q error %q", c.CostText, c.LastError)
	}
}

func TestAPIKeyCostUsesTheCalendarMonth(t *testing.T) {
	cases := map[store.CostRange]struct{ start, end time.Time }{
		store.CostRangeCurrent:  {time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), t0},
		store.CostRangePrevious: {time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC), time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)},
	}
	for rng, want := range cases {
		t.Run(string(rng), func(t *testing.T) {
			f := newFake(t)
			e := newRelayEnv(t)
			e.addPlain(t, plainKey, true)
			f.setAuth(codex.APIKeyAuth(plainKey), nil)
			var mu sync.Mutex
			windows := map[string]usage.Window{}
			inner := f.scanFn
			f.scanFn = func(ctx context.Context, qs []usage.Query) (map[string]usage.Summary, error) {
				mu.Lock()
				for _, q := range qs {
					windows[q.ID] = q.Window
				}
				mu.Unlock()
				return inner(ctx, qs)
			}
			s, _ := newService(t, f, func(o *Options) {
				e.apply()(o)
				o.Preferences.APICostSummaryRange = rng
			})
			showPanel(s)
			refreshAll(t, s)
			mu.Lock()
			defer mu.Unlock()
			cur := windows[queryCurrent]
			if !cur.Start.Equal(time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)) || !cur.End.Equal(t0) {
				t.Errorf("current window %v - %v", cur.Start, cur.End)
			}
			if rng == store.CostRangePrevious {
				sel := windows[querySelected]
				if !sel.Start.Equal(want.start) || !sel.End.Equal(want.end) {
					t.Errorf("previous window %v - %v", sel.Start, sel.End)
				}
			}
			noChatGPT(t, f)
		})
	}
}

// ---- relay balance ----------------------------------------------------------

func TestRelayBalanceIsProbedAndCachedOnTheRow(t *testing.T) {
	f := newFake(t)
	e := newRelayEnv(t)
	acc := e.addRelay(t, relayKeyA, relayBaseA, true)
	credBefore, err := os.ReadFile(e.st.CredentialPath(acc.ID))
	if err != nil {
		t.Fatal(err)
	}
	f.setAuth(codex.APIKeyAuth(relayKeyA), nil)
	s, _ := newService(t, f, e.apply())
	showPanel(s)
	c := refreshAll(t, s).Codex

	calls := e.callList()
	if len(calls) != 1 || calls[0].base != relayBaseAV1 || calls[0].key != relayKeyA {
		t.Fatalf("probe calls: %d, base %q", len(calls), calls[0].base)
	}
	noChatGPT(t, f)
	if !c.APIKeyMode || c.APIBaseURL != relayBaseAV1 || c.APIBalance == nil || c.APIBalance.Status != apiquota.StatusAvailable {
		t.Fatalf("view: mode %v base %q balance %+v", c.APIKeyMode, c.APIBaseURL, c.APIBalance)
	}
	if len(c.Meters) != 1 || !c.Meters[0].Balance || c.Meters[0].ValueText != "$12.34" || c.Meters[0].Title != en.Text(l10n.KeyWidgetBalance) {
		t.Errorf("meters: %+v", c.Meters)
	}
	if c.StatusText != "$12.34" || c.QuotaText != "$12.34 / $20.00" || c.LastError != "" {
		t.Errorf("status %q quota %q error %q", c.StatusText, c.QuotaText, c.LastError)
	}
	row := e.row(t, acc.ID)
	if row.APIQuota == nil || row.APIQuota.Remaining == nil || *row.APIQuota.Remaining != 12.34 {
		t.Errorf("balance not cached on the row: %+v", row.APIQuota)
	}
	if row.LastError != "" || row.RequiresReauth {
		t.Error("a probe changed the credential state of the row")
	}
	credAfter, err := os.ReadFile(e.st.CredentialPath(acc.ID))
	if err != nil || string(credAfter) != string(credBefore) {
		t.Error("the managed credential changed")
	}
	if len(s.Snapshot().Accounts.RefreshingIDs) != 0 {
		t.Error("refreshing flags left on")
	}
}

func TestRelayBalanceFromTheRowShowsBeforeTheFirstProbe(t *testing.T) {
	f := newFake(t)
	e := newRelayEnv(t)
	acc := e.addRelay(t, relayKeyA, relayBaseA, true)
	if err := e.st.UpdateMetadata(acc.ApplyAPIQuota(usdQuota(5, 10))); err != nil {
		t.Fatal(err)
	}
	f.setAuth(codex.APIKeyAuth(relayKeyA), nil)
	s, _ := newService(t, f, e.apply())
	// Nothing has loaded the credential yet: the account list alone cannot
	// tell which row is the login, so the cached balance appears with it.
	gate := newGate()
	e.probe = func(context.Context, string, string) apiquota.Quota { gate.wait(); return usdQuota(4, 10) }
	done := s.Refresh(usage.RefreshForce)
	gate.waitStarted(t)
	c := s.Snapshot().Codex
	if c.APIBalance == nil || c.StatusText != "$5.00" {
		t.Errorf("cached balance missing while probing: %+v", c.StatusText)
	}
	if !s.IsRefreshing(SectionQuota) {
		t.Error("the quota section is not marked refreshing during the probe")
	}
	gate.open()
	wait(t, done)
	settle(t, s)
	if got := s.Snapshot().Codex.StatusText; got != "$4.00" {
		t.Errorf("status after probe %q", got)
	}
}

func TestRelayProbeFailureIsNotAnAccountError(t *testing.T) {
	statuses := []apiquota.Status{apiquota.StatusError, apiquota.StatusUnauthorized, apiquota.StatusUnsupported, apiquota.StatusInvalidData}
	for _, status := range statuses {
		t.Run(string(status), func(t *testing.T) {
			f := newFake(t)
			e := newRelayEnv(t)
			acc := e.addRelay(t, relayKeyA, relayBaseA, true)
			f.setAuth(codex.APIKeyAuth(relayKeyA), nil)
			e.probe = func(context.Context, string, string) apiquota.Quota {
				return apiquota.Quota{Status: status, Error: "HTTP 401", FetchedAt: t0}
			}
			s, _ := newService(t, f, e.apply())
			showPanel(s)
			c := refreshAll(t, s).Codex
			if c.StatusText != "API" || len(c.Meters) != 0 || c.APIBalance == nil || c.APIBalance.Status != status {
				t.Errorf("status %q meters %d balance %+v", c.StatusText, len(c.Meters), c.APIBalance)
			}
			if c.LastError != "" || !c.Authenticated {
				t.Errorf("the probe signed out or reported an error: %q", c.LastError)
			}
			if len(c.QuotaLines) != 1 || c.QuotaLines[0].Value != "HTTP 401" {
				t.Errorf("lines: %v", c.QuotaLines)
			}
			if row := e.row(t, acc.ID); row.LastError != "" || row.RequiresReauth {
				t.Error("the row took the probe failure as a credential error")
			}
			noChatGPT(t, f)
		})
	}
}

func TestManualQuotaRefreshProbesTheRelayOnly(t *testing.T) {
	f := newFake(t)
	e := newRelayEnv(t)
	e.addRelay(t, relayKeyA, relayBaseA, true)
	f.setAuth(codex.APIKeyAuth(relayKeyA), nil)
	s, _ := newService(t, f, e.apply())
	wait(t, s.RefreshQuota())
	settle(t, s)
	if n := len(e.callList()); n != 1 {
		t.Errorf("probes %d", n)
	}
	noChatGPT(t, f)
	if got := s.Snapshot().Codex.StatusText; got != "$12.34" {
		t.Errorf("status %q", got)
	}
}

func TestPlainKeyIsNotSignedOutByEarlierFailureMarks(t *testing.T) {
	f := newFake(t)
	e := newRelayEnv(t)
	e.addPlain(t, plainKey, true)
	f.setAuth(codex.Auth{}, &AuthLoadError{Kind: AuthFileInvalid})
	s, _ := newService(t, f, e.apply())
	if c := refreshAll(t, s).Codex; c.Authenticated {
		t.Fatal("an unreadable login was shown as signed in")
	}
	f.setAuth(codex.APIKeyAuth(plainKey), nil)
	c := refreshAll(t, s).Codex
	if !c.Authenticated || c.StatusText != "API" || c.LastError != "" {
		t.Errorf("after fixing the login: auth %v status %q error %q", c.Authenticated, c.StatusText, c.LastError)
	}
}

// ---- switching ----------------------------------------------------------------

func TestSwitchingBetweenOAuthAndRelayChangesWhatIsRead(t *testing.T) {
	f := newFake(t)
	e := newRelayEnv(t)
	oauth := e.addOAuth(t, f.auth, true)
	relay := e.addRelay(t, relayKeyA, relayBaseA, false)
	relayAuth := codex.APIKeyAuth(relayKeyA)
	oauthAuth := f.auth
	s, _ := newService(t, f, func(o *Options) {
		e.apply()(o)
		o.Sources.SwitchAccount = func(_ context.Context, id string) (codex.SwitchResult, error) {
			a := oauthAuth
			if id == relay.ID {
				a = relayAuth
			}
			f.setAuth(a, nil)
			return codex.SwitchResult{Auth: a, Account: codex.ManagedAccount{ID: id}}, nil
		}
	})
	showPanel(s)
	if c := refreshAll(t, s).Codex; c.APIKeyMode || c.Quota == nil {
		t.Fatalf("oauth login: mode %v quota %v", c.APIKeyMode, c.Quota)
	}
	quotaCalls := f.count("quota")
	if quotaCalls == 0 {
		t.Fatal("the OAuth login did not read its quota")
	}

	if err := s.SwitchAccount(context.Background(), relay.ID); err != nil {
		t.Fatal(err)
	}
	settle(t, s)
	c := s.Snapshot().Codex
	if !c.APIKeyMode || c.Quota != nil || c.ResetCredits != nil || c.Estimate != nil || c.StatusText != "$12.34" {
		t.Errorf("relay login: mode %v quota %v status %q", c.APIKeyMode, c.Quota, c.StatusText)
	}
	wait(t, s.RefreshQuota())
	wait(t, s.RefreshResetCredits())
	settle(t, s)
	if got := f.count("quota"); got != quotaCalls {
		t.Errorf("the relay login read the ChatGPT quota (%d -> %d)", quotaCalls, got)
	}
	if f.count("credits") != 1 || f.count("daily") > 1 {
		t.Errorf("credits %d daily %d", f.count("credits"), f.count("daily"))
	}

	if err := s.SwitchAccount(context.Background(), oauth.ID); err != nil {
		t.Fatal(err)
	}
	settle(t, s)
	c = s.Snapshot().Codex
	if c.APIKeyMode || c.APIBalance != nil || c.Quota == nil || len(c.Meters) == 0 || c.Meters[0].Balance {
		t.Errorf("back on OAuth: mode %v balance %v quota %v", c.APIKeyMode, c.APIBalance, c.Quota)
	}
}

// ---- account list -----------------------------------------------------------------

func TestAccountListProbesRelaysBoundedAndSkipsPlainKeys(t *testing.T) {
	f := newFake(t)
	e := newRelayEnv(t)
	oauth := e.addOAuth(t, f.auth, true)
	plain := e.addPlain(t, plainKey, false)
	keys := []string{relayKeyA, relayKeyB, relayKeyC, relayKeyD, relayKeyE}
	var relays []codex.ManagedAccount
	for i, k := range keys {
		relays = append(relays, e.addRelay(t, k, "https://relay-"+string(rune('a'+i))+".example.com", false))
	}
	e.started = make(chan struct{}, len(keys))
	e.hold = make(chan struct{})
	s, _ := newService(t, f, e.apply())

	done := s.RefreshAllAccountQuotas()
	for range relayProbeConcurrency {
		select {
		case <-e.started:
		case <-time.After(5 * time.Second):
			t.Fatal("probes did not start")
		}
	}
	time.Sleep(50 * time.Millisecond)
	e.mu.Lock()
	running, started := e.inflight, len(e.calls)
	e.mu.Unlock()
	if running != relayProbeConcurrency || started != relayProbeConcurrency {
		t.Errorf("in flight %d, started %d, want %d", running, started, relayProbeConcurrency)
	}
	snap := s.Snapshot()
	if !snap.Refresh.AccountQuotas || len(snap.Accounts.RefreshingIDs) != len(keys)+2 {
		t.Errorf("refreshing: %v", snap.Accounts.RefreshingIDs)
	}
	close(e.hold)
	wait(t, done)
	settle(t, s)

	e.mu.Lock()
	peak := e.peak
	e.mu.Unlock()
	if peak > relayProbeConcurrency || len(e.callList()) != len(keys) {
		t.Errorf("peak %d, calls %d", peak, len(e.callList()))
	}
	for _, r := range relays {
		if row := e.row(t, r.ID); row.APIQuota == nil || row.APIQuota.Status != apiquota.StatusAvailable {
			t.Errorf("relay %s not cached", r.APIBaseURL)
		}
	}
	for _, id := range []string{plain.ID, oauth.ID} {
		if e.loads(id) != 0 || e.row(t, id).APIQuota != nil {
			t.Error("a non-relay account was probed")
		}
	}
	if len(s.Snapshot().Accounts.RefreshingIDs) != 0 || s.Snapshot().Refresh.AccountQuotas {
		t.Error("refreshing flags left on")
	}
	noChatGPT(t, f)
}

func TestFullRefreshProbesTheCurrentRelayOnce(t *testing.T) {
	f := newFake(t)
	e := newRelayEnv(t)
	e.addRelay(t, relayKeyA, relayBaseA, true)
	e.addRelay(t, relayKeyB, "https://relay-b.example.com", false)
	f.setAuth(codex.APIKeyAuth(relayKeyA), nil)
	s, _ := newService(t, f, e.apply())
	refreshAll(t, s)
	byBase := map[string]int{}
	for _, c := range e.callList() {
		byBase[c.base]++
	}
	if byBase[relayBaseAV1] != 1 || byBase["https://relay-b.example.com/v1"] != 1 || len(byBase) != 2 {
		t.Errorf("probes per site: %v", byBase)
	}
}

func TestRefreshAccountQuotaProbesOneRelay(t *testing.T) {
	f := newFake(t)
	e := newRelayEnv(t)
	oauth := e.addOAuth(t, f.auth, true)
	relay := e.addRelay(t, relayKeyA, relayBaseA, false)
	plain := e.addPlain(t, plainKey, false)
	var polled []string
	var pmu sync.Mutex
	gate := newGate()
	e.probe = func(context.Context, string, string) apiquota.Quota { gate.wait(); return usdQuota(3, 4) }
	s, _ := newService(t, f, func(o *Options) {
		e.apply()(o)
		o.Sources.Generations = &codexapi.Generations{}
		o.Sources.RefreshAccount = func(_ context.Context, id string) codexapi.QuotaRefreshResult {
			pmu.Lock()
			polled = append(polled, id)
			pmu.Unlock()
			return codexapi.QuotaRefreshResult{AccountID: id}
		}
	})
	done := s.RefreshAccountQuota(relay.ID)
	gate.waitStarted(t)
	if ids := s.Snapshot().Accounts.RefreshingIDs; len(ids) != 1 || ids[0] != relay.ID {
		t.Errorf("refreshing %v", ids)
	}
	if again := s.RefreshAccountQuota(relay.ID); again == nil {
		t.Error("nil channel")
	}
	gate.open()
	wait(t, done)
	settle(t, s)
	if row := e.row(t, relay.ID); row.APIQuota == nil || *row.APIQuota.Remaining != 3 {
		t.Errorf("relay row: %+v", row.APIQuota)
	}
	if n := len(e.callList()); n != 1 {
		t.Errorf("probes %d", n)
	}

	wait(t, s.RefreshAccountQuota(plain.ID))
	wait(t, s.RefreshAccountQuota(oauth.ID))
	settle(t, s)
	pmu.Lock()
	defer pmu.Unlock()
	if len(polled) != 2 || polled[0] != plain.ID || polled[1] != oauth.ID {
		t.Errorf("the OAuth poller saw %v", polled)
	}
	if len(e.callList()) != 1 || e.loads(plain.ID) != 0 {
		t.Error("a plain key or OAuth account probed a relay")
	}
}

func TestProbeResultForAChangedSiteIsDropped(t *testing.T) {
	f := newFake(t)
	e := newRelayEnv(t)
	relay := e.addRelay(t, relayKeyA, relayBaseA, false)
	e.started = make(chan struct{}, 1)
	e.hold = make(chan struct{})
	s, _ := newService(t, f, e.apply())
	done := s.RefreshAccountQuota(relay.ID)
	<-e.started
	if _, err := e.st.UpdateRelayProfile(relay.ID, codex.RelayProfile{BaseURL: "https://relay-new.example.com"}, t0); err != nil {
		t.Fatal(err)
	}
	close(e.hold)
	wait(t, done)
	settle(t, s)
	row := e.row(t, relay.ID)
	if row.APIQuota != nil || row.APIBaseURL != "https://relay-new.example.com/v1" {
		t.Errorf("a reading of the old site was cached: %+v base %q", row.APIQuota, row.APIBaseURL)
	}
}

func TestCancelledProbeStoresNothing(t *testing.T) {
	f := newFake(t)
	e := newRelayEnv(t)
	relay := e.addRelay(t, relayKeyA, relayBaseA, false)
	e.started = make(chan struct{}, 1)
	e.hold = make(chan struct{})
	s, _ := newService(t, f, e.apply())
	done := s.RefreshAccountQuota(relay.ID)
	<-e.started
	s.Close()
	wait(t, done)
	if row := e.row(t, relay.ID); row.APIQuota != nil {
		t.Errorf("a cancelled probe was cached: %+v", row.APIQuota)
	}
}

func TestRelayAccountsAreNotPolledWithoutProbeSources(t *testing.T) {
	f := newFake(t)
	e := newRelayEnv(t)
	relay := e.addRelay(t, relayKeyA, relayBaseA, false)
	s, _ := newService(t, f, func(o *Options) {
		o.Sources.LoadIndex = e.st.LoadIndex
		o.Sources.SaveAccount = e.st.UpdateMetadata
	})
	wait(t, s.RefreshAllAccountQuotas())
	wait(t, s.RefreshAccountQuota(relay.ID))
	if len(e.callList()) != 0 || e.loads(relay.ID) != 0 {
		t.Error("a relay was probed without sources")
	}
}

// ---- widget ---------------------------------------------------------------------------

func TestWidgetCarriesTheRelayBalance(t *testing.T) {
	unit := func(u string, remaining float64, unlimited bool) *apiquota.Quota {
		q := usdQuota(remaining, remaining*2)
		q.Unit, q.Unlimited = u, unlimited
		return &q
	}
	cases := []struct {
		name    string
		balance *apiquota.Quota
		text    string
		usd     bool
	}{
		{"usd", unit(apiquota.UnitUSD, 12.5, false), "$12.50", true},
		{"unit unset is dollars", unit("", 12.5, false), "$12.50", true},
		{"cny", unit(apiquota.UnitCNY, 12.5, false), "¥12.50", false},
		{"tokens", unit(apiquota.UnitTokens, 1_200_000, false), "1.20M tokens", false},
		{"unlimited", unit(apiquota.UnitUSD, 12.5, true), "∞", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			meter, ok := APIBalanceMeter(en, *tc.balance)
			if !ok {
				t.Fatal("no meter")
			}
			p := codexWidgetProvider(CodexView{Authenticated: true, APIKeyMode: true, APIBalance: tc.balance, Meters: []Meter{meter}})
			if p.Availability != export.Available {
				t.Errorf("availability %q", p.Availability)
			}
			if p.BalanceText == nil || *p.BalanceText != tc.text {
				t.Errorf("balance text %v, want %q", p.BalanceText, tc.text)
			}
			if (p.BalanceUSD != nil) != tc.usd {
				t.Errorf("balance USD %v, want present=%v", p.BalanceUSD, tc.usd)
			}
			if len(p.Quota) != 1 || p.Quota[0].ValueText == nil || *p.Quota[0].ValueText != tc.text {
				t.Errorf("quota rows: %+v", p.Quota)
			}
		})
	}
	t.Run("plain key", func(t *testing.T) {
		p := codexWidgetProvider(CodexView{Authenticated: true, APIKeyMode: true})
		if p.Availability != export.Available || p.BalanceText != nil || p.BalanceUSD != nil || len(p.Quota) != 0 {
			t.Errorf("plain key block: %+v", p)
		}
	})
	t.Run("failed probe", func(t *testing.T) {
		q := apiquota.Quota{Status: apiquota.StatusError, Error: "timeout"}
		p := codexWidgetProvider(CodexView{Authenticated: true, APIKeyMode: true, APIBalance: &q})
		if p.BalanceText != nil || p.BalanceUSD != nil {
			t.Errorf("a failed probe produced a balance: %+v", p)
		}
	})
}

func TestWidgetSnapshotOfARelayLoginKeepsLocalUsage(t *testing.T) {
	f := newFake(t)
	e := newRelayEnv(t)
	e.addRelay(t, relayKeyA, relayBaseA, true)
	f.setAuth(codex.APIKeyAuth(relayKeyA), nil)
	s, _ := newService(t, f, func(o *Options) {
		e.apply()(o)
		o.WidgetRequirements = export.RequireAll
	})
	showPanel(s)
	refreshAll(t, s)
	snap := s.WidgetSnapshot(t0)
	p := snap.ProviderOf(export.ProviderCodex)
	if p == nil || p.Availability != export.Available || p.BalanceText == nil || *p.BalanceText != "$12.34" || p.BalanceUSD == nil {
		t.Fatalf("codex block: %+v", p)
	}
	if p.APIEquivalentCostUSD == nil {
		t.Error("the local API-equivalent cost is missing")
	}
	noChatGPT(t, f)
}

func TestRelayTransientFailureKeepsLastBalance(t *testing.T) {
	good := apiquota.Quota{Status: apiquota.StatusAvailable, Remaining: f64(4), Unit: "USD", FetchedAt: t0}
	for status, keep := range map[apiquota.Status]bool{
		apiquota.StatusError: true, apiquota.StatusInvalidData: true,
		apiquota.StatusUnauthorized: false, apiquota.StatusUnsupported: false,
	} {
		if got := keepCachedBalance(&good, apiquota.Quota{Status: status}); got != keep {
			t.Errorf("%s: keep %v, want %v", status, got, keep)
		}
	}
	if keepCachedBalance(nil, apiquota.Quota{Status: apiquota.StatusError}) {
		t.Error("nothing to keep without a reading")
	}
}
