package codexapi

import (
	"strings"
	"testing"
	"time"
)

func TestDecodeClaims(t *testing.T) {
	token := jwt(t, map[string]any{
		"email": " user@example.com ",
		"sub":   "sub-1",
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id":                "acct-1",
			"chatgpt_plan_type":                 "team",
			"chatgpt_user_id":                   "user-1",
			"chatgpt_subscription_active_until": "2026-07-01T00:00:00Z",
			"organizations": []any{
				map[string]any{"id": "org-a", "title": "Alpha", "is_default": true},
				map[string]any{"id": "org-b", "name": "Beta"},
			},
		},
	})
	c, ok := DecodeClaims(token)
	if !ok || c.Email != "user@example.com" || c.AccountID != "acct-1" || c.PlanType != "team" || c.UserID != "user-1" ||
		c.OrganizationID != "org-a" || c.OrganizationName != "Alpha" || !c.SubscriptionActiveUntil.Equal(time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("%+v %v", c, ok)
	}
	for _, bad := range []string{"", "abc", "a.b", "a.!!!.c"} {
		if _, ok := DecodeClaims(bad); ok {
			t.Errorf("%q decoded", bad)
		}
	}
}

func TestOrganizationSelection(t *testing.T) {
	org := func(orgs ...any) string {
		c, _ := DecodeClaims(jwt(t, map[string]any{"https://api.openai.com/auth": map[string]any{"organizations": orgs}}))
		return c.OrganizationID
	}
	a := map[string]any{"id": "a"}
	b := map[string]any{"id": "b"}
	bd := map[string]any{"id": "b", "is_default": true}
	if org(a) != "a" || org(a, b) != "" || org(a, bd) != "b" || org(bd, map[string]any{"id": "c", "is_default": true}) != "" || org() != "" {
		t.Fatal("organization selection mismatch")
	}
}

func TestSubscriptionDateAsNumber(t *testing.T) {
	c, _ := DecodeClaims(jwt(t, map[string]any{"https://api.openai.com/auth": map[string]any{"chatgpt_subscription_active_until": 1_782_864_000}}))
	if !c.SubscriptionActiveUntil.Equal(time.Unix(1_782_864_000, 0)) {
		t.Fatalf("%v", c.SubscriptionActiveUntil)
	}
}

func TestTokenExpired(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	cases := []struct {
		exp  any
		want bool
	}{{1_000_100, false}, {1_000_060, true}, {1_000_061, false}, {1, true}}
	for _, tc := range cases {
		if got := TokenExpired(jwt(t, map[string]any{"exp": tc.exp}), now, 60*time.Second); got != tc.want {
			t.Errorf("exp %v: %v", tc.exp, got)
		}
	}
	if !TokenExpired(jwt(t, map[string]any{}), now, 0) || !TokenExpired("not-a-jwt", now, 0) {
		t.Fatal("unreadable tokens count as expired")
	}
}

func TestOAuthAccountID(t *testing.T) {
	claims := func(account, org string) string {
		auth := map[string]any{}
		if account != "" {
			auth["chatgpt_account_id"] = account
		}
		if org != "" {
			auth["organizations"] = []any{map[string]any{"id": org}}
		}
		return jwt(t, map[string]any{"https://api.openai.com/auth": auth})
	}
	cases := []struct {
		name string
		a    Auth
		want string
	}{
		{"selected wins", Auth{AccountID: " sel ", IDToken: claims("x", "")}, "sel"},
		{"claims agree", Auth{IDToken: claims("x", ""), AccessToken: claims("x", "")}, "x"},
		{"claims conflict", Auth{IDToken: claims("x", ""), AccessToken: claims("y", "")}, ""},
		{"single token", Auth{IDToken: claims("x", "")}, "x"},
		{"organization fallback", Auth{IDToken: claims("", "org")}, "org"},
		{"explicit blocks organization", Auth{IDToken: claims("x", "org"), AccessToken: claims("y", "org")}, ""},
		{"nothing", Auth{}, ""},
	}
	for _, tc := range cases {
		if got := tc.a.OAuthAccountID(); got != tc.want {
			t.Errorf("%s: %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestResolveTier(t *testing.T) {
	cases := map[string]SubscriptionTier{
		"free": TierFree, "plus": TierPlus, "pro": TierPro200, "prolite": TierPro100, "Pro_Max": TierPro500,
		"pro lite": TierPro100, "chatgpt-team": TierTeam, "business": TierBusiness, "self_serve_business_prolite": TierBusinessPremium, "enterprise": TierEnterprise,
		"edu": TierEdu, "api": TierAPI, "pro-whatever": TierPro200, "": TierUnknown, "mystery": TierUnknown,
	}
	for in, want := range cases {
		if got := ResolveTier(in, ""); got != want {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
	if ResolveTier("", "plus") != TierPlus {
		t.Fatal("fallback ignored")
	}
}

func TestSubscriptionExpiryProjection(t *testing.T) {
	jan31 := time.Date(2026, 1, 31, 12, 0, 0, 0, time.UTC)
	now := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	// Calendar months clamp: 31 Jan -> 28 Feb -> 28 Mar.
	if got := ResolveSubscriptionExpiry(jan31, TierPlus, now); !got.Equal(time.Date(2026, 3, 28, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("projected %v", got)
	}
	future := now.Add(time.Hour)
	if got := ResolveSubscriptionExpiry(future, TierPlus, now); !got.Equal(future) {
		t.Fatal("future date must not change")
	}
	for _, tier := range []SubscriptionTier{TierFree, TierAPI, TierUnknown} {
		if got := ResolveSubscriptionExpiry(jan31, tier, now); !got.Equal(jan31) {
			t.Fatalf("%s projected", tier)
		}
	}
	if !ResolveSubscriptionExpiry(time.Time{}, TierPlus, now).IsZero() {
		t.Fatal("zero date")
	}
}

func TestMakeAccountDisplay(t *testing.T) {
	id := jwt(t, map[string]any{
		"email":                       "me@example.com",
		"https://api.openai.com/auth": map[string]any{"chatgpt_plan_type": "plus", "chatgpt_subscription_active_until": "2026-06-01T00:00:00Z"},
	})
	d := MakeAccountDisplay(Auth{IDToken: id, AccountID: "0123456789abcdef"}, "", "", "", time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC))
	if d.DisplayName != "me@example.com" || d.Tier != TierPlus || !d.SubscriptionExpiresAt.Equal(time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("%+v", d)
	}
	anon := MakeAccountDisplay(Auth{AccountID: "0123456789abcdef"}, "pro", "", "", time.Now())
	if anon.DisplayName != "01234...cdef" || anon.Tier != TierPro200 || strings.Contains(anon.DisplayName, "6789ab") {
		t.Fatalf("%+v", anon)
	}
}
