package runway

import (
	"context"
	"errors"
	"time"

	"github.com/licoy/quosparr/internal/codex"
	"github.com/licoy/quosparr/internal/codexapi"
	"github.com/licoy/quosparr/internal/export/swiftjson"
	"github.com/licoy/quosparr/internal/sessions"
	"github.com/licoy/quosparr/internal/usage"
)

// AuthFailKind says why the local credential cannot be used.
type AuthFailKind int

// AuthFailKind values.
const (
	// AuthFileInvalid: missing, unreadable or placeholder credential.
	AuthFileInvalid AuthFailKind = iota
	// AuthExpired: the access token expired and cannot be renewed.
	AuthExpired
)

// AuthLoadError is a local credential failure (Swift RunwayModelAuthError.load).
// It never carries token text.
type AuthLoadError struct {
	Kind AuthFailKind
	Err  error
}

func (e *AuthLoadError) Error() string {
	if e.Kind == AuthExpired {
		return "auth expired"
	}
	return "auth file invalid"
}

// Unwrap exposes the cause for errors.Is.
func (e *AuthLoadError) Unwrap() error { return e.Err }

// IsAuthFailure reports whether err means "sign in again": a local credential
// failure or a server refusal (HTTP 401).
func IsAuthFailure(err error) bool {
	var load *AuthLoadError
	return errors.As(err, &load) || errors.Is(err, codexapi.ErrAuthenticationRequired)
}

// Sources is the Service's outside world: credentials, network, scans. Tests
// replace every field with a fake; NewLiveSources wires the real ones. Fields
// marked optional may be nil.
type Sources struct {
	// LoadValidAuth returns a usable credential, refreshing and persisting
	// its tokens when expired. With preferCached it returns cached as is.
	// Local credential failures must be *AuthLoadError.
	LoadValidAuth func(ctx context.Context, preferCached bool, cached *codex.Auth) (codex.Auth, error)
	// FetchQuota, FetchResetCredits, FetchDailyUsage and FetchProfileTokens
	// are the ChatGPT backend reads. A refused credential must satisfy
	// errors.Is(err, codexapi.ErrAuthenticationRequired).
	FetchQuota         func(ctx context.Context, a codexapi.Auth) (codexapi.QuotaSnapshot, error)
	FetchResetCredits  func(ctx context.Context, a codexapi.Auth) (codexapi.ResetCreditsSnapshot, error)
	FetchDailyUsage    func(ctx context.Context, a codexapi.Auth, startDate, endDate string, w usage.Window, calculatedAt time.Time) (usage.Summary, error)
	FetchProfileTokens func(ctx context.Context, a codexapi.Auth) (usage.ProfileTokenUsage, error)
	// ScanCost summarizes the local session logs for the queries.
	ScanCost func(ctx context.Context, q []usage.Query, calculatedAt time.Time, p usage.RefreshPolicy, progress *usage.ProgressReporter) (map[string]usage.Summary, error)

	// DryRunSessions / RepairSessions / ScanRecentSessions (optional) read or
	// repair the session index and list recent sessions.
	DryRunSessions     func(ctx context.Context) (sessions.Report, error)
	RepairSessions     func(ctx context.Context) (sessions.Report, error)
	ScanRecentSessions func(ctx context.Context, limit int) (sessions.Summary, error)

	// Account library (optional). SyncOfficial aligns the library with the
	// official auth.json; LoadIndex reads the account list; SaveAccount
	// persists one account row; RecordAuth mirrors a freshly loaded
	// credential into the library; OfficialUsability tells why a 401 happened.
	SyncOfficial      func(now time.Time) (codex.AccountIndex, error)
	LoadIndex         func() (codex.AccountIndex, error)
	SaveAccount       func(acc codex.ManagedAccount) error
	RecordAuth        func(auth codex.Auth, quotaPlan string, now time.Time) error
	OfficialUsability func(now time.Time) (codex.Usability, bool)

	// SwitchAccount installs a managed account as the official login.
	SwitchAccount func(ctx context.Context, id string) (codex.SwitchResult, error)
	// RefreshAccounts / RefreshAccount poll the quota of managed accounts.
	// Results carry the account id and generation; Generations (shared with
	// the refresher) lets the Service drop stale ones.
	RefreshAccounts func(ctx context.Context) []codexapi.QuotaRefreshResult
	RefreshAccount  func(ctx context.Context, id string) codexapi.QuotaRefreshResult
	Generations     *codexapi.Generations
}

// apiAuth reduces a credential to what the API calls need.
func apiAuth(a codex.Auth) codexapi.Auth {
	return codexapi.Auth{IDToken: a.Tokens.IDToken, AccessToken: a.Tokens.AccessToken, AccountID: a.Tokens.AccountID}
}

// LiveConfig wires NewLiveSources. All paths live inside Store.
type LiveConfig struct {
	Store     *codex.Store
	CodexHome string
	Quota     codexapi.QuotaClient
	Refresher codexapi.TokenRefresher
	// Repo is the local usage index; nil disables local cost scans.
	Repo *usage.Repository
	Now  func() time.Time
}

// NewLiveSources builds the production Sources.
func NewLiveSources(cfg LiveConfig) Sources {
	l := &live{LiveConfig: cfg, gens: &codexapi.Generations{}}
	if l.Now == nil {
		l.Now = time.Now
	}
	l.switcher = &codex.Switcher{Store: cfg.Store, Now: l.Now, Hooks: codex.SwitchHooks{
		Refresh:  l.refreshInPlace,
		Validate: l.validate,
	}}
	l.accounts = codexapi.AccountQuotaRefresher{
		Client:      cfg.Quota,
		Accounts:    l.targets,
		Credential:  l.credential,
		Generations: l.gens,
	}
	return Sources{
		LoadValidAuth:      l.loadValidAuth,
		FetchQuota:         cfg.Quota.FetchQuota,
		FetchResetCredits:  cfg.Quota.FetchResetCredits,
		FetchDailyUsage:    l.fetchDaily,
		FetchProfileTokens: l.fetchProfile,
		ScanCost:           l.scanCost,
		DryRunSessions: func(context.Context) (sessions.Report, error) {
			return sessions.Repairer{CodexHome: cfg.CodexHome}.DryRun()
		},
		RepairSessions: func(context.Context) (sessions.Report, error) {
			return sessions.Repairer{CodexHome: cfg.CodexHome}.Repair()
		},
		ScanRecentSessions: func(_ context.Context, limit int) (sessions.Summary, error) {
			return sessions.Scanner{CodexHome: cfg.CodexHome, Pricer: sessionPricer}.Scan(limit)
		},
		SyncOfficial: func(now time.Time) (codex.AccountIndex, error) {
			idx, _, err := cfg.Store.SyncFromOfficialAuth(now)
			return idx, err
		},
		LoadIndex:   cfg.Store.LoadIndex,
		SaveAccount: cfg.Store.UpdateMetadata,
		RecordAuth:  l.recordAuth,
		OfficialUsability: func(now time.Time) (codex.Usability, bool) {
			a, err := cfg.Store.LoadOfficialAuth()
			if err != nil {
				return codex.InvalidTokens, false
			}
			return a.LoginUsability(now), true
		},
		SwitchAccount:   l.switcher.SwitchTo,
		RefreshAccounts: l.accounts.RefreshAll,
		RefreshAccount:  l.accounts.Refresh,
		Generations:     l.gens,
	}
}

type live struct {
	LiveConfig
	gens     *codexapi.Generations
	switcher *codex.Switcher
	accounts codexapi.AccountQuotaRefresher
}

func sessionPricer(model string, t sessions.Totals) (swiftjson.Decimal, bool) {
	cost, ok := usage.CostForTotals(model, usage.Totals{
		TotalTokens: int64(t.TotalTokens), UncachedInputTokens: int64(t.UncachedInputTokens),
		CachedInputTokens: int64(t.CachedInputTokens), OutputTokens: int64(t.OutputTokens),
		Turns: int64(t.Turns), Threads: int64(t.Threads),
	})
	if !ok {
		return swiftjson.Decimal{}, false
	}
	d, err := swiftjson.ParseDecimal(cost.String())
	return d, err == nil
}

func (l *live) loadValidAuth(ctx context.Context, preferCached bool, cached *codex.Auth) (codex.Auth, error) {
	if err := l.Store.CheckOfficialCredentialStore(); err != nil {
		return codex.Auth{}, &AuthLoadError{Kind: AuthFileInvalid, Err: err}
	}
	now := l.Now()
	var auth codex.Auth
	if preferCached && cached != nil {
		auth = *cached
	} else {
		a, err := l.Store.LoadOfficialAuth()
		if err != nil {
			return codex.Auth{}, &AuthLoadError{Kind: AuthFileInvalid, Err: err}
		}
		auth = a
	}
	switch auth.LoginUsability(now) {
	case codex.InvalidTokens:
		return codex.Auth{}, &AuthLoadError{Kind: AuthFileInvalid}
	case codex.ExpiredAccessWithoutRefresh:
		return codex.Auth{}, &AuthLoadError{Kind: AuthExpired}
	}
	// Access-token-only sessions are used while the JWT is valid; they never
	// hit the refresh endpoint.
	if !auth.IsAPIKey() && codex.IsTokenExpired(auth.Tokens.AccessToken, now) {
		if !auth.CanRefreshOAuth() {
			return codex.Auth{}, &AuthLoadError{Kind: AuthExpired}
		}
		return l.refreshCurrent(ctx, auth, now)
	}
	return auth, nil
}

// refreshCurrent renews the official credential. The persister writes the
// managed copy and, for the current account, the official file.
func (l *live) refreshCurrent(ctx context.Context, auth codex.Auth, now time.Time) (codex.Auth, error) {
	id := l.managedID(auth)
	cur := codexapi.TokenSet{IDToken: auth.Tokens.IDToken, AccessToken: auth.Tokens.AccessToken, RefreshToken: auth.Tokens.RefreshToken}
	var persister codexapi.TokenPersister = TokenPersister{Store: l.Store}
	if id == "" {
		persister = discardPersister{}
	}
	ts, err := l.Refresher.RefreshAndPersist(ctx, persister, id, cur, now)
	if ts.AccessToken == "" {
		if codexapi.NeedsReauth(err) {
			return codex.Auth{}, &AuthLoadError{Kind: AuthExpired, Err: err}
		}
		return codex.Auth{}, err
	}
	// A persist failure still leaves valid tokens for this run.
	auth.Tokens.IDToken, auth.Tokens.AccessToken, auth.Tokens.RefreshToken = ts.IDToken, ts.AccessToken, ts.RefreshToken
	auth.LastRefresh = codexapi.FormatTimestamp(now)
	return auth, nil
}

type discardPersister struct{}

func (discardPersister) SaveRefreshed(context.Context, string, codexapi.TokenSet, time.Time) error {
	return nil
}

func (l *live) managedID(auth codex.Auth) string {
	idx, err := l.Store.LoadIndex()
	if err != nil {
		return ""
	}
	for _, a := range idx.Accounts {
		if codex.AccountMatches(a, auth) {
			return a.ID
		}
	}
	return ""
}

func (l *live) refreshInPlace(ctx context.Context, a *codex.Auth) error {
	ts, err := l.Refresher.Refresh(ctx, a.Tokens.RefreshToken)
	if err != nil {
		if codexapi.NeedsReauth(err) {
			return errors.Join(codex.ErrReauthRequired, err)
		}
		return err
	}
	m := ts.MergedInto(codexapi.TokenSet{IDToken: a.Tokens.IDToken, AccessToken: a.Tokens.AccessToken, RefreshToken: a.Tokens.RefreshToken})
	a.Tokens.IDToken, a.Tokens.AccessToken, a.Tokens.RefreshToken = m.IDToken, m.AccessToken, m.RefreshToken
	a.LastRefresh = codexapi.FormatTimestamp(l.Now())
	return nil
}

func (l *live) validate(ctx context.Context, a codex.Auth) error {
	_, err := l.Quota.FetchQuota(ctx, apiAuth(a))
	if codexapi.NeedsReauth(err) {
		return errors.Join(codex.ErrReauthRequired, err)
	}
	return err
}

func (l *live) targets(context.Context) ([]codexapi.AccountTarget, error) {
	idx, err := l.Store.LoadIndex()
	if err != nil {
		return nil, err
	}
	var out []codexapi.AccountTarget
	for _, a := range idx.OrderedForSidebar() {
		out = append(out, codexapi.AccountTarget{
			ID:               a.ID,
			APIKey:           a.AuthMode == codex.AuthModeAPIKey,
			HasWorkspaceName: a.WorkspaceName != "",
			Tier:             codexapi.SubscriptionTier(a.Tier()),
		})
	}
	return out, nil
}

func (l *live) credential(ctx context.Context, id string) (codexapi.Auth, error) {
	auth, err := l.switcher.EnsureValidCredential(ctx, id)
	if err != nil {
		if errors.Is(err, codex.ErrReauthRequired) {
			return codexapi.Auth{}, errors.Join(codexapi.ErrAuthenticationRequired, err)
		}
		return codexapi.Auth{}, err
	}
	return apiAuth(auth), nil
}

func (l *live) fetchDaily(ctx context.Context, a codexapi.Auth, start, end string, w usage.Window, at time.Time) (usage.Summary, error) {
	raw, err := l.Quota.FetchDailyWorkspaceUsageRaw(ctx, a, start, end)
	if err != nil {
		return usage.Summary{}, err
	}
	return usage.DecodeAnalytics(raw, w, at, start, end)
}

func (l *live) fetchProfile(ctx context.Context, a codexapi.Auth) (usage.ProfileTokenUsage, error) {
	p, err := l.Quota.FetchTokenUsageProfile(ctx, a)
	if err != nil {
		return usage.ProfileTokenUsage{}, err
	}
	days := make(map[string]int64, len(p.DailyTokens))
	for d, n := range p.DailyTokens {
		days[d] = int64(n)
	}
	return usage.ProfileTokenUsage{DailyTokens: days, StatsAsOf: p.StatsAsOf, GeneratedAt: p.GeneratedAt}, nil
}

func (l *live) scanCost(ctx context.Context, q []usage.Query, at time.Time, p usage.RefreshPolicy, progress *usage.ProgressReporter) (map[string]usage.Summary, error) {
	if l.Repo == nil {
		return nil, errors.New("local usage index unavailable")
	}
	return l.Repo.Summaries(ctx, q, at, p, progress)
}

// recordAuth mirrors the freshly loaded credential into the library: the
// stored copy is replaced only by an equal-or-better one, and the row's
// identity metadata is refreshed. It never touches the official file.
func (l *live) recordAuth(auth codex.Auth, quotaPlan string, now time.Time) error {
	idx, err := l.Store.LoadIndex()
	if err != nil || idx.ActiveAccountID == "" || auth.LoginUsability(now) != codex.Usable {
		return err
	}
	acc, ok := idx.Account(idx.ActiveAccountID)
	if !ok || !codex.AccountMatches(acc, auth) {
		return nil
	}
	prev, perr := l.Store.LoadCredential(acc.ID)
	if perr != nil || !prev.Equal(auth) {
		if perr != nil || prev.LoginUsability(now) != codex.Usable || auth.LoginUsability(now) == codex.Usable {
			if err := l.Store.SaveCredential(acc.ID, auth); err != nil {
				return err
			}
		}
	}
	return l.Store.UpdateMetadata(acc.WithIdentity(auth, quotaPlan, now))
}
