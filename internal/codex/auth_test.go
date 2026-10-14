package codex

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func parseT(t *testing.T, s string) time.Time {
	t.Helper()
	tm, ok := ParseTime(s)
	if !ok {
		t.Fatalf("bad time %q", s)
	}
	return tm
}

func TestDecodeAuthAndRedaction(t *testing.T) {
	data := `{"auth_mode":"chatgpt","tokens":{"id_token":"id-secret","access_token":"access-secret",
	"refresh_token":"refresh-secret","account_id":"account-1"},"last_refresh":"2026-06-29T00:00:00Z"}`
	auth := must(DecodeAuth([]byte(data)))
	if auth.AuthMode != "chatgpt" || auth.Tokens.IDToken != "id-secret" || auth.Tokens.AccountID != "account-1" {
		t.Fatalf("unexpected decode: %#v", auth.Redacted())
	}
	for _, shown := range []string{auth.Redacted(), auth.String(), auth.Tokens.String()} {
		for _, secret := range []string{"id-secret", "access-secret", "refresh-secret"} {
			if strings.Contains(shown, secret) {
				t.Errorf("%q leaks %s", shown, secret)
			}
		}
	}
	if !strings.Contains(auth.Redacted(), "account-1") {
		t.Errorf("redacted text should keep the account id: %s", auth.Redacted())
	}
}

func TestDecodeAuthShapes(t *testing.T) {
	tests := []struct {
		name, in string
		ok       bool
	}{
		{"no id token", `{"auth_mode":"chatgpt","tokens":{"access_token":"a","refresh_token":"r","account_id":"x"}}`, true},
		{"api key only", `{"OPENAI_API_KEY":"sk-123456789"}`, true},
		{"null api key and tokens", `{"OPENAI_API_KEY":null,"tokens":null}`, false},
		{"tokens wrong type", `{"tokens":"nope"}`, false},
		{"token field wrong type", `{"tokens":{"access_token":5}}`, false},
		{"refresh token only", `{"tokens":{"refresh_token":"r"}}`, true},
		{"not an object", `[1]`, false},
		{"garbage", `not json`, false},
		{"empty", ``, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := DecodeAuth([]byte(tc.in))
			if (err == nil) != tc.ok {
				t.Fatalf("err = %v, want ok=%v", err, tc.ok)
			}
			if err != nil && !errors.Is(err, ErrInvalidCredential) {
				t.Fatalf("error %v should be ErrInvalidCredential", err)
			}
			if err != nil && strings.Contains(err.Error(), tc.in) && tc.in != "" {
				t.Fatalf("error repeats the input: %v", err)
			}
		})
	}
}

func TestTokenExpiry(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	tests := []struct {
		name, jwt string
		expired   bool
	}{
		{"past", jwtExp(t, 1_700_000_000), true},
		{"future", jwtExp(t, farFuture), false},
		{"within skew", jwtExp(t, 1_800_000_030), true},
		{"just past skew", jwtExp(t, 1_800_000_090), false},
		{"not a jwt", "not-a-jwt", true},
		{"no exp", jwtOf(t, map[string]any{"sub": "x"}), true},
	}
	for _, tc := range tests {
		if got := IsTokenExpired(tc.jwt, now); got != tc.expired {
			t.Errorf("%s: expired = %v, want %v", tc.name, got, tc.expired)
		}
	}
}

func TestMergeRefreshResponse(t *testing.T) {
	t.Run("keeps refresh token when omitted", func(t *testing.T) {
		auth := Auth{AuthMode: "chatgpt", Tokens: Tokens{IDToken: "old-id", AccessToken: "old-access", RefreshToken: "old-refresh", AccountID: "account-1"}}
		body := `{"access_token":"new-access","id_token":"new-id","expires_in":3600}`
		noErr(t, auth.MergeRefreshResponse([]byte(body), time.Unix(1_782_710_000, 0)))
		if auth.Tokens.IDToken != "new-id" || auth.Tokens.AccessToken != "new-access" || auth.Tokens.RefreshToken != "old-refresh" {
			t.Fatalf("tokens: %s", auth.Redacted())
		}
		if auth.LastRefresh != "2026-06-29T05:13:20Z" {
			t.Fatalf("last_refresh = %s", auth.LastRefresh)
		}
	})
	t.Run("keeps id token when omitted", func(t *testing.T) {
		auth := Auth{Tokens: Tokens{IDToken: "old-id", AccessToken: "old-access", RefreshToken: "old-refresh"}}
		noErr(t, auth.MergeRefreshResponse([]byte(`{"access_token":"new-access"}`), testNow))
		if auth.Tokens.IDToken != "old-id" || auth.Tokens.AccessToken != "new-access" {
			t.Fatalf("tokens: %s", auth.Redacted())
		}
	})
	t.Run("rotates refresh token", func(t *testing.T) {
		auth := Auth{Tokens: Tokens{AccessToken: "a", RefreshToken: "old"}}
		noErr(t, auth.MergeRefreshResponse([]byte(`{"access_token":"b","refresh_token":"rotated"}`), testNow))
		if auth.Tokens.RefreshToken != "rotated" {
			t.Fatal("refresh token not rotated")
		}
	})
	t.Run("rejects a response without access token", func(t *testing.T) {
		auth := Auth{Tokens: Tokens{AccessToken: "a", RefreshToken: "old"}}
		if err := auth.MergeRefreshResponse([]byte(`{"refresh_token":"x"}`), testNow); err == nil {
			t.Fatal("expected an error")
		}
		if auth.Tokens.RefreshToken != "old" {
			t.Fatal("a failed merge must not change the auth")
		}
	})
}

func TestIdentityClaims(t *testing.T) {
	token := jwtOf(t, map[string]any{
		"email": "top@example.com", "name": "Top Name", "preferred_username": "top-user", "sub": "user-sub",
		"https://api.openai.com/profile": map[string]any{"email": "profile@example.com"},
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_plan_type": "prolite", "chatgpt_account_id": "acct_123",
			"chatgpt_user_id": "chat-user-id", "chatgpt_subscription_active_until": "2026-06-20T11:08:02+00:00",
		},
	})
	c := DecodeIdentityClaims(token)
	if c == nil {
		t.Fatal("claims not decoded")
	}
	if c.Email != "top@example.com" || c.Username != "top-user" || c.Subject != "user-sub" ||
		c.UserID != "chat-user-id" || c.PlanType != "prolite" || c.AccountID != "acct_123" ||
		c.OrganizationID != "" || c.OrganizationName != "" {
		t.Fatalf("claims: %+v", c)
	}
	if c.SubscriptionActiveUntil == nil || !c.SubscriptionActiveUntil.Equal(parseT(t, "2026-06-20T11:08:02+00:00")) {
		t.Fatalf("subscription end: %v", c.SubscriptionActiveUntil)
	}
	if DecodeIdentityClaims("not-a-jwt") != nil {
		t.Fatal("a non-JWT must not decode")
	}
}

func TestProfileEmailFallback(t *testing.T) {
	c := DecodeIdentityClaims(jwtOf(t, map[string]any{
		"https://api.openai.com/profile": map[string]any{"email": "profile@example.com"},
	}))
	if c == nil || c.Email != "profile@example.com" {
		t.Fatalf("claims: %+v", c)
	}
}

func orgs(entries ...map[string]any) []any {
	out := make([]any, len(entries))
	for i, e := range entries {
		out[i] = e
	}
	return out
}

func TestOrganizationFallback(t *testing.T) {
	explicit := DecodeIdentityClaims(jwtOf(t, map[string]any{"https://api.openai.com/auth": map[string]any{
		"chatgpt_account_id": "workspace-explicit",
		"organizations":      orgs(map[string]any{"id": "org-default", "is_default": true, "title": "Workspace Alpha"}),
	}}))
	if explicit.AccountID != "workspace-explicit" || explicit.OrganizationID != "org-default" || explicit.OrganizationName != "Workspace Alpha" {
		t.Fatalf("explicit: %+v", explicit)
	}

	fallbackToken := jwtOf(t, map[string]any{"https://api.openai.com/auth": map[string]any{
		"organizations": orgs(
			map[string]any{"id": "org-first", "is_default": false, "title": "First"},
			map[string]any{"id": "org-selected", "is_default": true, "title": "Selected Workspace"}),
	}})
	fallback := DecodeIdentityClaims(fallbackToken)
	if fallback.AccountID != "" || fallback.OrganizationID != "org-selected" || fallback.OrganizationName != "Selected Workspace" {
		t.Fatalf("fallback: %+v", fallback)
	}
	auth := Auth{AuthMode: "chatgpt", Tokens: Tokens{IDToken: fallbackToken, AccessToken: jwtOf(t, nil), RefreshToken: "organization-fallback-refresh-token"}}
	managed := NewManagedAccount("", auth, 0, "", "", "", testNow)
	if managed.AccountID != "org-selected" || managed.WorkspaceName != "Selected Workspace" {
		t.Fatalf("managed: %+v", managed)
	}
}

func TestAmbiguousOrganizationsAreNotGuessed(t *testing.T) {
	token := jwtOf(t, map[string]any{"email": "member@example.com", "https://api.openai.com/auth": map[string]any{
		"chatgpt_user_id": "user-shared",
		"organizations": orgs(
			map[string]any{"id": "org-first", "is_default": false, "title": "First"},
			map[string]any{"id": "org-second", "is_default": false, "title": "Second"}),
	}})
	c := DecodeIdentityClaims(token)
	if c.OrganizationID != "" || c.OrganizationName != "" {
		t.Fatalf("claims: %+v", c)
	}
	auth := Auth{AuthMode: "chatgpt", Tokens: Tokens{IDToken: token, AccessToken: jwtOf(t, nil), RefreshToken: "ambiguous-organizations-refresh-token"}}
	managed := NewManagedAccount("", auth, 0, "", "", "", testNow)
	if managed.AccountID != "" || managed.WorkspaceName != "" {
		t.Fatalf("managed: %+v", managed)
	}
	if !strings.HasPrefix(MatchKey(auth), "oauth:credential:") {
		t.Fatalf("match key = %s", MatchKey(auth))
	}
}

func TestConflictingClaimsFallBackToCredentialFingerprint(t *testing.T) {
	authClaims := func(user, workspace string) map[string]any {
		return map[string]any{"chatgpt_user_id": user, "chatgpt_account_id": workspace}
	}
	userConflict := Auth{AuthMode: "chatgpt", Tokens: Tokens{
		IDToken:      jwtOf(t, map[string]any{"email": "same@example.com", "https://api.openai.com/auth": authClaims("user-one", "workspace-shared")}),
		AccessToken:  jwtOf(t, map[string]any{"email": "same@example.com", "https://api.openai.com/auth": authClaims("user-two", "workspace-shared")}),
		RefreshToken: "conflicting-user-refresh-token", AccountID: "workspace-shared",
	}}
	if !strings.HasPrefix(MatchKey(userConflict), "oauth:credential:") {
		t.Errorf("user conflict key = %s", MatchKey(userConflict))
	}
	workspaceConflict := Auth{AuthMode: "chatgpt", Tokens: Tokens{
		IDToken:      jwtOf(t, map[string]any{"https://api.openai.com/auth": authClaims("user-one", "workspace-claim")}),
		AccessToken:  jwtOf(t, nil),
		RefreshToken: "conflicting-workspace-refresh-token", AccountID: "workspace-selected",
	}}
	if got := oauthAccountID(workspaceConflict); got != "workspace-selected" {
		t.Errorf("selected workspace = %s", got)
	}
	if got, want := MatchKey(workspaceConflict), "oauth:user:user-one|account:workspace-selected"; got != want {
		t.Errorf("match key = %s, want %s", got, want)
	}
}

func TestResolveTier(t *testing.T) {
	tests := []struct {
		plan, fallback string
		want           Tier
	}{
		{"free", "", TierFree}, {"plus", "", TierPlus},
		{"prolite", "", TierPro100}, {"pro-lite", "", TierPro100}, {"pro-5x", "", TierPro100},
		{"codex-pro-5x", "", TierPro100}, {"pro-100", "", TierPro100}, {"pro100", "", TierPro100},
		{"pro", "", TierPro200}, {"pro-20x", "", TierPro200}, {"codex-pro-20x", "", TierPro200},
		{"pro-200", "", TierPro200}, {"pro200", "", TierPro200}, {"pro-custom", "", TierPro200},
		{"promax", "", TierPro500}, {"pro-max", "", TierPro500}, {"pro_500", "", TierPro500},
		{"pro 500", "", TierPro500}, {"pro500", "", TierPro500}, {"codex-pro-500", "", TierPro500},
		{"self-serve-business-prolite", "", TierBusiness}, {"business", "", TierBusiness},
		{"team", "", TierTeam}, {"enterprise", "", TierEnterprise}, {"edu", "", TierEdu},
		{"api_key", "", TierAPI}, {"", "plus", TierPlus}, {"mystery", "", TierUnknown},
	}
	for _, tc := range tests {
		if got := ResolveTier(tc.plan, tc.fallback); got != tc.want {
			t.Errorf("ResolveTier(%q,%q) = %s, want %s", tc.plan, tc.fallback, got, tc.want)
		}
	}
}

func TestDisplayForAuth(t *testing.T) {
	now := parseT(t, "2026-07-15T12:00:00Z")
	idToken := jwtOf(t, map[string]any{"email": "person@example.com", "https://api.openai.com/auth": map[string]any{
		"chatgpt_plan_type": "plus", "account_id": "acct_123456789",
		"chatgpt_subscription_active_until": "2026-08-01T00:00:00Z",
	}})
	auth := Auth{AuthMode: "chatgpt", Tokens: Tokens{
		IDToken: idToken, AccessToken: jwtOf(t, map[string]any{"preferred_username": "access-user"}),
		RefreshToken: "refresh", AccountID: "acct_123456789",
	}}
	d := DisplayForAuth(auth, "pro", now)
	if d.DisplayName != "person@example.com" || d.Email != "person@example.com" || d.AccountID != "acct_123456789" || d.Tier != TierPro200 {
		t.Fatalf("display: %+v", d)
	}
	if d.SubscriptionExpiresAt == nil || !d.SubscriptionExpiresAt.Equal(parseT(t, "2026-08-01T00:00:00Z")) {
		t.Fatalf("subscription end: %v", d.SubscriptionExpiresAt)
	}
}

func TestDisplayFallbacks(t *testing.T) {
	byUsername := Auth{Tokens: Tokens{
		IDToken: jwtOf(t, map[string]any{"preferred_username": "id-user"}), AccessToken: jwtOf(t, nil),
		RefreshToken: "refresh", AccountID: "acct_123456789",
	}}
	if got := DisplayForAuth(byUsername, "", testNow).DisplayName; got != "id-user" {
		t.Errorf("username fallback = %q", got)
	}
	byAccount := Auth{Tokens: Tokens{AccessToken: jwtOf(t, nil), RefreshToken: "refresh", AccountID: "acct_123456789"}}
	if got := DisplayForAuth(byAccount, "", testNow).DisplayName; got != "acct_...6789" {
		t.Errorf("account fallback = %q", got)
	}
	unknown := Auth{Tokens: Tokens{AccessToken: jwtOf(t, nil), RefreshToken: "refresh"}}
	if d := DisplayForAuth(unknown, "", testNow); !d.Authenticated || d.DisplayName != "" {
		t.Errorf("unknown identity: %+v", d)
	}
}

func TestSubscriptionProjection(t *testing.T) {
	now := parseT(t, "2026-07-19T14:00:00Z")
	raw := parseT(t, "2026-06-20T11:08:02+00:00")
	want := parseT(t, "2026-07-20T11:08:02+00:00")
	tests := []struct {
		name string
		raw  time.Time
		tier Tier
		want time.Time
	}{
		{"stale pro100 advances", raw, TierPro100, want},
		{"stale pro500 advances", raw, TierPro500, want},
		{"free keeps the past claim", raw, TierFree, raw},
		{"future untouched", parseT(t, "2026-08-20T11:08:02+00:00"), TierPro100, parseT(t, "2026-08-20T11:08:02+00:00")},
		// Swift advances month by month from the previous result, so the
		// clamp to Feb 28 sticks.
		{"month end clamps", parseT(t, "2026-01-31T10:00:00Z"), TierPlus, parseT(t, "2026-07-28T10:00:00Z")},
	}
	for _, tc := range tests {
		got := ResolveSubscriptionExpiry(&tc.raw, tc.tier, now)
		if got == nil || !got.Equal(tc.want) {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
	if ResolveSubscriptionExpiry(nil, TierPlus, now) != nil {
		t.Error("nil stays nil")
	}
}

func TestLoginUsability(t *testing.T) {
	long := jwtOf(t, map[string]any{"exp": farFuture, "pad": strings.Repeat("p", 40)})
	expired := jwtOf(t, map[string]any{"exp": 1_700_000_000, "email": "x@y.z", "pad": strings.Repeat("p", 40)})
	tests := []struct {
		name string
		auth Auth
		want Usability
	}{
		{"oauth with refresh", Auth{Tokens: Tokens{AccessToken: long, RefreshToken: strings.Repeat("r", 20)}}, Usable},
		{"short refresh", Auth{Tokens: Tokens{AccessToken: long, RefreshToken: "short"}}, InvalidTokens},
		{"short access", Auth{Tokens: Tokens{AccessToken: "short", RefreshToken: strings.Repeat("r", 20)}}, InvalidTokens},
		{"session valid", Auth{Tokens: Tokens{AccessToken: long}}, Usable},
		{"session expired", Auth{Tokens: Tokens{AccessToken: expired}}, ExpiredAccessWithoutRefresh},
		{"api key", APIKeyAuth("sk-12345678"), Usable},
		{"short api key", APIKeyAuth("sk-123"), InvalidTokens},
	}
	for _, tc := range tests {
		if got := tc.auth.LoginUsability(testNow); got != tc.want {
			t.Errorf("%s: %v, want %v", tc.name, got, tc.want)
		}
	}
	session := Auth{Tokens: Tokens{AccessToken: long}}
	if !session.IsAccessTokenOnly() {
		t.Error("session should be access-token-only")
	}
}

func TestMarshalFilePreservesUnknownFields(t *testing.T) {
	existing := `{"auth_mode":"chatgpt","custom_top":{"keep":[1,2,3],"n":1.50},"tokens":{"access_token":"a","refresh_token":"r","id_token":"i","account_id":"x","custom_token_field":"keep-me"},"last_refresh":"2026-01-01T00:00:00Z"}`
	auth := must(DecodeAuth([]byte(existing)))
	auth.Tokens.AccessToken = "new-access"
	out := must(auth.MarshalFile([]byte(existing)))
	var got map[string]json.RawMessage
	noErr(t, json.Unmarshal(out, &got))
	if string(got["custom_top"]) == "" || !strings.Contains(string(out), `"n" : 1.50`) {
		t.Fatalf("unknown top-level field lost or reformatted:\n%s", out)
	}
	if !strings.Contains(string(out), `"custom_token_field" : "keep-me"`) || !strings.Contains(string(out), `"access_token" : "new-access"`) {
		t.Fatalf("token fields wrong:\n%s", out)
	}
	// The same auth decoded from out carries the unknown fields forward even
	// when no existing file is passed.
	again := must(DecodeAuth(out))
	out2 := must(again.MarshalFile(nil))
	if string(out2) != string(out) {
		t.Fatalf("not stable:\n%s\n%s", out, out2)
	}
}

// Swift's official save keeps whatever keys the file already holds, so an
// API-key write leaves earlier tokens and last_refresh in place.
func TestAPIKeyWriteKeepsExistingKeys(t *testing.T) {
	existing := `{"auth_mode":"chatgpt","tokens":{"access_token":"a","refresh_token":"r"},"last_refresh":"2026-01-01T00:00:00Z"}`
	out := string(must(APIKeyAuth("sk-test-key-1234567890").MarshalFile([]byte(existing))))
	for _, want := range []string{`"access_token" : "a"`, `"last_refresh" : "2026-01-01T00:00:00Z"`, `"OPENAI_API_KEY" : "sk-test-key-1234567890"`, `"auth_mode" : "apikey"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %s in\n%s", want, out)
		}
	}
}
