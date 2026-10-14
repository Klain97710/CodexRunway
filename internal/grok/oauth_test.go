package grok

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	testDeviceEndpoint = "https://auth.x.ai/oauth2/device/code"
	testTokenEndpoint  = "https://auth.x.ai/oauth2/token"
)

// authServer plays auth.x.ai for a device-code login.
type authServer struct {
	mu        sync.Mutex
	discovery string
	device    string
	tokens    []string // successive token endpoint bodies
	forms     []map[string]string
	polls     int
}

func (s *authServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = r.ParseForm()
	form := map[string]string{}
	for k := range r.PostForm {
		form[k] = r.PostForm.Get(k)
	}
	switch r.URL.String() {
	case OAuthDiscoveryURL:
		fmt.Fprint(w, firstOf(s.discovery, fmt.Sprintf(
			`{"device_authorization_endpoint":%q,"token_endpoint":%q}`, testDeviceEndpoint, testTokenEndpoint)))
	case testDeviceEndpoint:
		s.forms = append(s.forms, form)
		fmt.Fprint(w, firstOf(s.device,
			`{"device_code":"dev-code","user_code":"ABCD-1234","verification_uri":"https://accounts.x.ai/device","verification_uri_complete":"https://accounts.x.ai/device?user_code=ABCD-1234","expires_in":600,"interval":5}`))
	case testTokenEndpoint:
		s.forms = append(s.forms, form)
		body := s.tokens[min(s.polls, len(s.tokens)-1)]
		s.polls++
		if strings.Contains(body, `"error"`) {
			w.WriteHeader(http.StatusBadRequest)
		}
		fmt.Fprint(w, body)
	default:
		http.NotFound(w, r)
	}
}

func idToken(claims string) string {
	enc := base64.RawURLEncoding.EncodeToString
	return enc([]byte(`{"alg":"none"}`)) + "." + enc([]byte(claims)) + ".sig"
}

type loginRig struct {
	login  OAuthLogin
	server *authServer
	opened []string
	sleeps []time.Duration
	clock  time.Time
}

func newLoginRig(tokens ...string) *loginRig {
	rig := &loginRig{server: &authServer{tokens: tokens}, clock: fixtureNow}
	rig.login = OAuthLogin{
		HTTP:    handlerClient(rig.server),
		OpenURL: func(u string) error { rig.opened = append(rig.opened, u); return nil },
		Now:     func() time.Time { return rig.clock },
		Sleep: func(ctx context.Context, d time.Duration) error {
			rig.sleeps = append(rig.sleeps, d)
			rig.clock = rig.clock.Add(d)
			return ctx.Err()
		},
	}
	return rig
}

func TestLoginWritesAuthJSONAndOpensVerificationURL(t *testing.T) {
	id := idToken(`{"email":"Device@Example.com","sub":"sub-9"}`)
	rig := newLoginRig(
		`{"error":"authorization_pending"}`,
		`{"error":"slow_down"}`,
		fmt.Sprintf(`{"access_token":"device-access","refresh_token":"device-refresh","id_token":%q,"expires_in":3600,"token_type":"Bearer"}`, id))
	var shown DeviceCode
	rig.login.OnDeviceCode = func(d DeviceCode) { shown = d }
	home := filepath.Join(t.TempDir(), "pending")

	if err := rig.login.Login(context.Background(), home); err != nil {
		t.Fatal(err)
	}
	if shown.UserCode != "ABCD-1234" || len(rig.opened) != 1 || rig.opened[0] != "https://accounts.x.ai/device?user_code=ABCD-1234" {
		t.Errorf("shown %+v opened %v", shown, rig.opened)
	}
	if want := []time.Duration{5 * time.Second, 10 * time.Second}; fmt.Sprint(rig.sleeps) != fmt.Sprint(want) {
		t.Errorf("poll waits = %v, want %v (slow_down adds 5s)", rig.sleeps, want)
	}
	if f := rig.server.forms[0]; f["client_id"] != OAuthClientID || f["scope"] != OAuthScope {
		t.Errorf("device request form = %v", f)
	}
	if f := rig.server.forms[1]; f["grant_type"] != oauthDeviceGrantType || f["device_code"] != "dev-code" || f["client_id"] != OAuthClientID {
		t.Errorf("token request form = %v", f)
	}

	path := filepath.Join(home, "auth.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := ParseAuthDocument(data)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Identity.Email != "Device@Example.com" || doc.Identity.UserID != "sub-9" || !doc.Identity.HasRefreshToken ||
		doc.Identity.ClientID != OAuthClientID || doc.Identity.ExpiresAt == nil {
		t.Errorf("identity = %+v", doc.Identity)
	}
	if runtime.GOOS != "windows" {
		for p, want := range map[string]os.FileMode{home: 0o700, path: 0o600} {
			if info, err := os.Stat(p); err != nil || info.Mode().Perm() != want {
				t.Errorf("%s mode = %v (%v)", filepath.Base(p), info.Mode().Perm(), err)
			}
		}
	}
}

func TestLoginContinuesWhenBrowserCannotOpen(t *testing.T) {
	rig := newLoginRig(`{"access_token":"a","email":"x@example.com"}`)
	rig.login.OpenURL = func(string) error { return &OAuthError{Kind: OAuthBrowserOpenFailed, Message: "no browser"} }
	home := filepath.Join(t.TempDir(), "h")
	if err := rig.login.Login(context.Background(), home); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, "auth.json")); err != nil {
		t.Error(err)
	}
}

func TestLoginFailures(t *testing.T) {
	cases := []struct {
		name string
		rig  func() *loginRig
		kind OAuthErrorKind
	}{
		{"denied", func() *loginRig { return newLoginRig(`{"error":"access_denied"}`) }, OAuthDenied},
		{"expired token", func() *loginRig { return newLoginRig(`{"error":"expired_token"}`) }, OAuthExpired},
		{"other error", func() *loginRig { return newLoginRig(`{"error":"x","error_description":"nope"}`) }, OAuthTokenFailed},
		{"missing access token", func() *loginRig { return newLoginRig(`{"refresh_token":"r"}`) }, OAuthTokenFailed},
		{"no identity", func() *loginRig { return newLoginRig(`{"access_token":"a"}`) }, OAuthMissingIdentity},
		{"http discovery", func() *loginRig {
			r := newLoginRig(`{}`)
			r.server.discovery = `{"device_authorization_endpoint":"http://auth.x.ai/d","token_endpoint":"https://auth.x.ai/t"}`
			return r
		}, OAuthInvalidEndpoint},
		{"foreign host", func() *loginRig {
			r := newLoginRig(`{}`)
			r.server.discovery = `{"device_authorization_endpoint":"https://auth.x.ai/d","token_endpoint":"https://evil.example.com/t"}`
			return r
		}, OAuthInvalidEndpoint},
		{"lookalike host", func() *loginRig {
			r := newLoginRig(`{}`)
			r.server.discovery = `{"device_authorization_endpoint":"https://notx.ai/d","token_endpoint":"https://auth.x.ai/t"}`
			return r
		}, OAuthInvalidEndpoint},
		{"unparseable discovery", func() *loginRig {
			r := newLoginRig(`{}`)
			r.server.discovery = `nope`
			return r
		}, OAuthDiscoveryFailed},
		{"device code without user code", func() *loginRig {
			r := newLoginRig(`{}`)
			r.server.device = `{"device_code":"d"}`
			return r
		}, OAuthDeviceCodeFailed},
		{"device code without verification url", func() *loginRig {
			r := newLoginRig(`{}`)
			r.server.device = `{"device_code":"d","user_code":"u"}`
			return r
		}, OAuthDeviceCodeFailed},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			home := filepath.Join(t.TempDir(), "h")
			err := c.rig().login.Login(context.Background(), home)
			var oauth *OAuthError
			if !errors.As(err, &oauth) || oauth.Kind != c.kind {
				t.Fatalf("err = %v, want kind %d", err, c.kind)
			}
			if _, statErr := os.Stat(filepath.Join(home, "auth.json")); statErr == nil {
				t.Error("auth.json written despite the failure")
			}
		})
	}
}

func TestLoginGivesUpAfterTheDeviceCodeExpires(t *testing.T) {
	rig := newLoginRig(`{"error":"authorization_pending"}`)
	rig.server.device = `{"device_code":"d","user_code":"u","verification_uri":"https://accounts.x.ai/device","expires_in":12,"interval":5}`
	err := rig.login.Login(context.Background(), filepath.Join(t.TempDir(), "h"))
	if !errors.Is(err, &OAuthError{Kind: OAuthExpired}) {
		t.Fatalf("err = %v", err)
	}
	if len(rig.sleeps) < 2 || len(rig.sleeps) > 4 {
		t.Errorf("polled with waits %v", rig.sleeps)
	}
}

func TestLoginCancellation(t *testing.T) {
	rig := newLoginRig(`{"error":"authorization_pending"}`)
	ctx, cancel := context.WithCancel(context.Background())
	rig.login.Sleep = func(ctx context.Context, d time.Duration) error { cancel(); return ctx.Err() }
	err := rig.login.Login(ctx, filepath.Join(t.TempDir(), "h"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
}

func TestMakeAuthDocumentDataUsesStableIdentity(t *testing.T) {
	withSubject, err := MakeAuthDocumentData(TokenBundle{AccessToken: "a", Email: "E@x.com", Subject: "sub-1"}, fixtureNow)
	if err != nil {
		t.Fatal(err)
	}
	emailOnly, err := MakeAuthDocumentData(TokenBundle{AccessToken: "a", Email: "E@x.com"}, fixtureNow)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := ParseAuthDocument(withSubject)
	b, _ := ParseAuthDocument(emailOnly)
	if a.StableID() == b.StableID() {
		t.Error("a subject should identify the user, not the email")
	}
	if !strings.HasPrefix(b.Identity.UserID, "email-") {
		t.Errorf("email-only user id = %q", b.Identity.UserID)
	}
}
