package codex

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// Tokens is the "tokens" object of an auth.json. Empty strings stand for
// absent values. Fields this package does not model are kept in extra.
type Tokens struct {
	IDToken      string
	AccessToken  string
	RefreshToken string
	AccountID    string
	extra        object
}

// HasOAuthTokens reports whether an access or refresh token is present.
func (t Tokens) HasOAuthTokens() bool { return t.AccessToken != "" || t.RefreshToken != "" }

// String never shows token text, so a stray %v cannot leak it.
func (t Tokens) String() string { return "Tokens(<redacted>)" }

// Auth is a Codex auth.json (official or managed). Unknown top-level and
// token-level fields are preserved and written back unchanged.
type Auth struct {
	AuthMode string
	Tokens   Tokens
	// LastRefresh is the raw last_refresh text ("2026-06-29T00:00:00Z").
	LastRefresh string
	// PlanType and AuthFilePlanType are Quosparr-only metadata that may
	// be present in imported files; they are read but never written out.
	PlanType         string
	AuthFilePlanType string
	// OpenAIAPIKey is nil when the file has no (or a null) OPENAI_API_KEY.
	OpenAIAPIKey *string
	extra        object
}

// APIKeyAuth builds an API-key-only credential.
func APIKeyAuth(key string) Auth {
	return Auth{AuthMode: "apikey", PlanType: "api", AuthFilePlanType: "api", OpenAIAPIKey: &key}
}

// String never shows credential text; see Redacted.
func (a Auth) String() string { return a.Redacted() }

// Redacted describes the auth without any secret.
func (a Auth) Redacted() string {
	keyFlag := "none"
	if a.OpenAIAPIKey != nil {
		keyFlag = "<redacted>"
	}
	mode, account := a.AuthMode, a.Tokens.AccountID
	if mode == "" {
		mode = "unknown"
	}
	if account == "" {
		account = "none"
	}
	return fmt.Sprintf("Auth(authMode: %s, accountId: %s, idToken: <redacted>, accessToken: <redacted>, "+
		"refreshToken: <redacted>, OPENAI_API_KEY: %s)", mode, account, keyFlag)
}

// APIKey returns the OPENAI_API_KEY text, or "".
func (a Auth) APIKey() string {
	if a.OpenAIAPIKey == nil {
		return ""
	}
	return *a.OpenAIAPIKey
}

// IsAPIKey reports an API-key credential: a key without OAuth tokens, or an
// explicit apikey auth mode.
func (a Auth) IsAPIKey() bool {
	if a.APIKey() != "" && !a.Tokens.HasOAuthTokens() {
		return true
	}
	switch strings.ToLower(a.AuthMode) {
	case "apikey", "api_key", "api-key":
		return true
	}
	return false
}

// CanRefreshOAuth reports whether a refresh token is present.
func (a Auth) CanRefreshOAuth() bool { return a.Tokens.RefreshToken != "" }

// IsAccessTokenOnly reports a browser-session credential: a real-looking
// access token and no refresh token.
func (a Auth) IsAccessTokenOnly() bool {
	return !a.IsAPIKey() && runeLen(a.Tokens.AccessToken) >= 40 && a.Tokens.RefreshToken == ""
}

// Equal compares the modeled fields; unknown fields do not count.
func (a Auth) Equal(b Auth) bool {
	ta, tb := a.Tokens, b.Tokens
	return a.AuthMode == b.AuthMode && a.LastRefresh == b.LastRefresh &&
		a.PlanType == b.PlanType && a.AuthFilePlanType == b.AuthFilePlanType &&
		a.APIKey() == b.APIKey() && (a.OpenAIAPIKey == nil) == (b.OpenAIAPIKey == nil) &&
		ta.IDToken == tb.IDToken && ta.AccessToken == tb.AccessToken &&
		ta.RefreshToken == tb.RefreshToken && ta.AccountID == tb.AccountID
}

// Usability says whether a credential may be installed as the Codex login.
type Usability int

const (
	// Usable credentials may be installed.
	Usable Usability = iota
	// ExpiredAccessWithoutRefresh: the access token has expired (or is
	// missing) and there is no refresh token to renew it.
	ExpiredAccessWithoutRefresh
	// InvalidTokens: placeholders or truncated junk.
	InvalidTokens
)

// LoginUsability classifies the credential, evaluated at time now.
func (a Auth) LoginUsability(now time.Time) Usability {
	if a.IsAPIKey() {
		if runeLen(a.APIKey()) >= 8 {
			return Usable
		}
		return InvalidTokens
	}
	if runeLen(a.Tokens.AccessToken) < 40 {
		return InvalidTokens
	}
	if a.CanRefreshOAuth() {
		if runeLen(a.Tokens.RefreshToken) < 20 {
			return InvalidTokens
		}
		return Usable
	}
	if IsTokenExpired(a.Tokens.AccessToken, now) {
		return ExpiredAccessWithoutRefresh
	}
	return Usable
}

func runeLen(s string) int { return utf8.RuneCountInString(s) }

// DecodeAuth parses an auth.json. It fails when the data is not an object,
// has fields of the wrong type, or holds neither tokens nor OPENAI_API_KEY.
// The error never contains file content.
func DecodeAuth(data []byte) (Auth, error) {
	o, err := parseObject(data)
	if err != nil {
		return Auth{}, ErrInvalidCredential
	}
	r := newReader(o)
	a := Auth{AuthMode: r.optString("auth_mode")}
	if raw, ok := r.raw("tokens"); ok {
		a.Tokens = decodeTokens(raw, r)
	}
	a.LastRefresh = r.optString("last_refresh")
	a.PlanType = r.optString("plan_type")
	a.AuthFilePlanType = r.optString("auth_file_plan_type")
	a.OpenAIAPIKey = r.optStringPtr("OPENAI_API_KEY")
	if r.err != nil {
		return Auth{}, ErrInvalidCredential
	}
	if a.Tokens.AccessToken == "" && a.Tokens.RefreshToken == "" && a.OpenAIAPIKey == nil {
		return Auth{}, ErrInvalidCredential
	}
	a.extra = r.rest()
	return a, nil
}

func decodeTokens(raw json.RawMessage, parent *reader) Tokens {
	o, err := parseObject(raw)
	if err != nil {
		parent.fail("tokens", "expected an object")
		return Tokens{}
	}
	r := newReader(o)
	t := Tokens{
		IDToken:      r.optString("id_token"),
		AccessToken:  r.optString("access_token"),
		RefreshToken: r.optString("refresh_token"),
		AccountID:    r.optString("account_id"),
	}
	if r.err != nil {
		parent.fail("tokens", "invalid field")
	}
	t.extra = r.rest()
	return t
}

// fileObject is the auth.json shape Codex expects for a: modeled fields plus
// preserved unknown ones. Quosparr-only metadata is never emitted.
func (a Auth) fileObject() object {
	o := a.extra.clone()
	mode := a.AuthMode
	if mode == "" {
		mode = "chatgpt"
		if a.IsAPIKey() {
			mode = "apikey"
		}
	}
	o["auth_mode"] = mustValue(mode)
	if a.OpenAIAPIKey != nil {
		o["OPENAI_API_KEY"] = mustValue(*a.OpenAIAPIKey)
	}
	if a.IsAPIKey() {
		return o
	}
	o["tokens"] = a.Tokens.fileObject()
	if a.LastRefresh != "" {
		o["last_refresh"] = mustValue(a.LastRefresh)
	}
	return o
}

// fileObject encodes the tokens. Codex expects refresh_token to exist even
// when empty; empty id_token and account_id are left out.
func (t Tokens) fileObject() json.RawMessage {
	b := newBuilder(t.extra)
	b.str("id_token", t.IDToken)
	b.set("access_token", t.AccessToken)
	b.set("refresh_token", t.RefreshToken)
	b.str("account_id", t.AccountID)
	return mustValue(b.o)
}

// MarshalFile renders a as an auth.json. existing is the current content of
// the destination, if any: its keys are kept unless a replaces them, which
// keeps fields newer Codex versions add. A bad or empty existing is
// ignored.
func (a Auth) MarshalFile(existing []byte) ([]byte, error) {
	merged := object{}
	if o, err := parseObject(existing); err == nil {
		merged = o
	}
	previousTokens := merged["tokens"]
	for k, v := range a.fileObject() {
		merged[k] = v
	}
	if !a.IsAPIKey() && a.OpenAIAPIKey == nil {
		// A login replacing an API key must not leave that key behind: the
		// key may belong to a relay and older Codex versions prefer it.
		if _, held := merged["OPENAI_API_KEY"]; held {
			merged["OPENAI_API_KEY"] = json.RawMessage("null")
		}
	}
	if !a.IsAPIKey() {
		merged["tokens"] = a.keepSameLoginTokenFields(previousTokens, merged["tokens"])
	}
	delete(merged, "plan_type")
	delete(merged, "auth_file_plan_type")
	raw, err := encodeValue(merged)
	if err != nil {
		return nil, err
	}
	return formatJSON(raw, formatOptions{pretty: true, escapeSlash: true})
}

// keepSameLoginTokenFields carries unknown token-level fields of the file
// being replaced into the new tokens object, but only when both belong to the
// same workspace login (equal non-empty account_id): another login's extras
// must not leak into this one.
func (a Auth) keepSameLoginTokenFields(previous, next json.RawMessage) json.RawMessage {
	old, err := parseObject(previous)
	if err != nil || a.Tokens.AccountID == "" {
		return next
	}
	var oldAccount string
	if raw, ok := old["account_id"]; !ok || json.Unmarshal(raw, &oldAccount) != nil || oldAccount != a.Tokens.AccountID {
		return next
	}
	updated, err := parseObject(next)
	if err != nil {
		return next
	}
	for k, v := range old {
		if _, set := updated[k]; !set {
			updated[k] = v
		}
	}
	return mustValue(updated)
}

// MergeRefreshResponse applies a token endpoint response body: the new access
// token (required), and the id and refresh tokens when present and non-empty.
// last_refresh becomes now. Unknown response fields are ignored.
func (a *Auth) MergeRefreshResponse(body []byte, now time.Time) error {
	o, err := parseObject(body)
	if err != nil {
		return ErrInvalidCredential
	}
	r := newReader(o)
	idToken := r.optString("id_token")
	access := r.reqString("access_token")
	refresh := r.optString("refresh_token")
	if r.err != nil {
		return ErrInvalidCredential
	}
	if idToken != "" {
		a.Tokens.IDToken = idToken
	}
	a.Tokens.AccessToken = access
	if refresh != "" {
		a.Tokens.RefreshToken = refresh
	}
	a.LastRefresh = formatSeconds(now)
	return nil
}
