package codexapi

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestStartLoginSession(t *testing.T) {
	s, err := StartLoginSession(1456, fixedNow, 0)
	if err != nil {
		t.Fatal(err)
	}
	if s.RedirectURI != "http://localhost:1456/auth/callback" || s.Port != 1456 || !s.ExpiresAt.Equal(fixedNow.Add(5*time.Minute)) {
		t.Fatalf("%+v", s)
	}
	u, err := url.Parse(s.AuthURL)
	if err != nil || u.Scheme+"://"+u.Host+u.Path != AuthEndpoint {
		t.Fatalf("%s %v", s.AuthURL, err)
	}
	q := u.Query()
	want := map[string]string{
		"response_type": "code", "client_id": ClientID, "redirect_uri": s.RedirectURI, "scope": Scopes,
		"code_challenge": codeChallenge(s.CodeVerifier), "code_challenge_method": "S256", "state": s.State,
		"id_token_add_organizations": "true", "codex_cli_simplified_flow": "true", "originator": "codex_vscode",
	}
	for k, v := range want {
		if q.Get(k) != v {
			t.Errorf("%s = %q, want %q", k, q.Get(k), v)
		}
	}
	if strings.Contains(s.AuthURL, "+") || !strings.HasPrefix(u.RawQuery, "response_type=code&client_id=") {
		t.Fatalf("query encoding/order: %s", u.RawQuery)
	}
	// RFC 7636 appendix B vector for the S256 transform.
	if got := codeChallenge("dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"); got != "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM" {
		t.Fatalf("challenge %s", got)
	}
	if len(s.CodeVerifier) < 43 || s.LoginID == "" || s.LoginID != strings.ToUpper(s.LoginID) {
		t.Fatalf("verifier/id %+v", s)
	}
	other, _ := StartLoginSession(0, time.Time{}, time.Minute)
	if other.Port != PreferredCallbackPort || other.State == s.State || other.CodeVerifier == s.CodeVerifier {
		t.Fatalf("sessions must be unique and default the port: %+v", other)
	}
}

func TestAuthorizationCode(t *testing.T) {
	code, err := AuthorizationCode("http://localhost:1455/auth/callback?code=abc&state=st", "st")
	if err != nil || code != "abc" {
		t.Fatalf("%q %v", code, err)
	}
	for name, raw := range map[string]string{
		"error":    "http://localhost/auth/callback?error=access_denied&state=st",
		"mismatch": "http://localhost/auth/callback?code=abc&state=other",
		"nostate":  "http://localhost/auth/callback?code=abc",
		"nocode":   "http://localhost/auth/callback?state=st",
	} {
		_, err := AuthorizationCode(raw, "st")
		var oe *OAuthError
		if !errors.As(err, &oe) || !NeedsReauth(err) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := AuthorizationCode("http://localhost/x?error=access_denied", "st"); err == nil || err.Error() != "access_denied" {
		t.Fatalf("error description: %v", err)
	}
}

func TestExchangeCode(t *testing.T) {
	id := jwt(t, map[string]any{"https://api.openai.com/auth": map[string]any{"chatgpt_account_id": "acct-9", "chatgpt_plan_type": "plus"}})
	srv := newRecorder(t, func(recorded) (int, string) {
		return 200, `{"id_token":"` + id + `","access_token":"acc","refresh_token":"ref"}`
	})
	session, _ := StartLoginSession(1455, fixedNow, 0)
	l := OAuthLogin{HTTP: srv.Client(), TokenURL: srv.URL + "/oauth/token"}
	got, err := l.ExchangeCode(context.Background(), "the-code", session, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	if got.AccountID != "acct-9" || got.PlanType != "plus" || got.AuthMode != "chatgpt" || got.LastRefresh != FormatTimestamp(fixedNow) ||
		got.Tokens.AccessToken != "acc" || got.Tokens.RefreshToken != "ref" {
		t.Fatalf("%+v", got)
	}
	r := srv.last(t)
	for k, v := range map[string]string{"grant_type": "authorization_code", "client_id": ClientID, "code": "the-code",
		"redirect_uri": session.RedirectURI, "code_verifier": session.CodeVerifier} {
		if r.Form.Get(k) != v {
			t.Errorf("%s = %q", k, r.Form.Get(k))
		}
	}
}

func TestExchangeCodeFailures(t *testing.T) {
	session, _ := StartLoginSession(1455, fixedNow, 0)
	for _, tc := range []struct {
		status int
		body   string
	}{{400, `{"error":"invalid_grant"}`}, {500, `{}`}, {200, `{"access_token":null}`}} {
		srv := newRecorder(t, func(recorded) (int, string) { return tc.status, tc.body })
		l := OAuthLogin{HTTP: srv.Client(), TokenURL: srv.URL}
		_, err := l.ExchangeCode(context.Background(), "c", session, fixedNow)
		if err == nil || (tc.status != 200 && !NeedsReauth(err)) {
			t.Errorf("%d: %v", tc.status, err)
		}
	}
	srv := newRecorder(t, func(recorded) (int, string) { return 200, `{"access_token":""}` })
	l := OAuthLogin{HTTP: srv.Client(), TokenURL: srv.URL}
	if _, err := l.ExchangeCode(context.Background(), "c", session, fixedNow); !NeedsReauth(err) {
		t.Fatalf("empty tokens: %v", err)
	}
}
