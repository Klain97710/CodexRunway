package netx

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/store"
)

type fakeRaw struct {
	items       map[string][]byte
	getErr      error
	setErr      error
	interactive []bool
}

func (f *fakeRaw) get(id string, allow bool) ([]byte, bool, error) {
	f.interactive = append(f.interactive, allow)
	if f.getErr != nil {
		return nil, false, f.getErr
	}
	v, ok := f.items[id]
	return v, ok, nil
}

func (f *fakeRaw) set(id string, data []byte) error {
	if f.setErr != nil {
		return f.setErr
	}
	if f.items == nil {
		f.items = map[string][]byte{}
	}
	f.items[id] = data
	return nil
}

func (f *fakeRaw) del(id string) error { delete(f.items, id); return nil }

func TestStoredJSONMatchesSwiftShape(t *testing.T) {
	raw := &fakeRaw{}
	s := systemStore{raw: raw}
	if err := s.Save(Credentials{Username: "us<er>", Password: "pa&ss\"w"}, testUUID); err != nil {
		t.Fatal(err)
	}
	got := string(raw.items[testUUID])
	if got != `{"username":"us<er>","password":"pa&ss\"w"}` {
		t.Fatalf("stored value %q", got)
	}
	// A value written by the Swift JSONEncoder (sorted keys, escaped slashes).
	raw.items[testUUID] = []byte(`{"password":"p\/w","username":"swift-user"}`)
	c, err := s.Load(testUUID, false)
	if err != nil || c.Username != "swift-user" || c.Password != "p/w" {
		t.Fatalf("load: %+v %v", c, err)
	}
	if raw.interactive[len(raw.interactive)-1] {
		t.Fatal("background load must not allow interaction")
	}
	if _, err := s.Load(testUUID, true); err != nil || !raw.interactive[len(raw.interactive)-1] {
		t.Fatal("interactive load did not pass the flag")
	}
}

func TestStoreRulesOfTheSwiftImplementation(t *testing.T) {
	raw := &fakeRaw{}
	s := systemStore{raw: raw}
	if _, err := s.Load("not-a-uuid", false); !errors.Is(err, ErrCredentialsUnavailable) {
		t.Fatalf("bad id load: %v", err)
	}
	if err := s.Save(Credentials{Username: "u", Password: "p"}, "not-a-uuid"); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("bad id save: %v", err)
	}
	if _, err := s.Load(testUUID, false); !errors.Is(err, ErrCredentialsUnavailable) {
		t.Fatalf("missing: %v", err)
	}
	for _, bad := range []string{``, `{`, `{"username":"u"}`, `{"username":null,"password":"p"}`, `[1]`} {
		raw.items = map[string][]byte{testUUID: []byte(bad)}
		if _, err := s.Load(testUUID, false); !errors.Is(err, ErrCredentialsUnavailable) {
			t.Fatalf("%q: %v", bad, err)
		}
	}
	raw.getErr = errors.New("locked")
	if _, err := s.Load(testUUID, false); !errors.Is(err, ErrCredentialsUnavailable) {
		t.Fatalf("read failure: %v", err)
	}
	raw.setErr = errors.New("denied")
	if err := s.Save(Credentials{Username: "u", Password: "p"}, testUUID); !errors.Is(err, ErrCredentialStoreFailed) {
		t.Fatalf("write failure: %v", err)
	}
	if err := s.Delete(testUUID); err != nil {
		t.Fatal(err)
	}
}

func TestCredentialValidation(t *testing.T) {
	ok := Credentials{Username: "user", Password: "pass"}
	cases := []struct {
		name string
		c    Credentials
		mode store.ProxyMode
		ok   bool
	}{
		{"valid http", ok, store.ProxyModeHTTP, true},
		{"empty user", Credentials{Password: "p"}, store.ProxyModeHTTP, false},
		{"empty password http", Credentials{Username: "u"}, store.ProxyModeHTTP, true},
		{"empty password socks", Credentials{Username: "u"}, store.ProxyModeSOCKS5, false},
		{"colon http", Credentials{Username: "a:b", Password: "p"}, store.ProxyModeHTTP, false},
		{"colon socks", Credentials{Username: "a:b", Password: "p"}, store.ProxyModeSOCKS5, true},
		{"newline user", Credentials{Username: "a\nb", Password: "p"}, store.ProxyModeHTTP, false},
		{"NUL password", Credentials{Username: "a", Password: "p\x00"}, store.ProxyModeHTTP, false},
		{"U+2028", Credentials{Username: "a", Password: "p "}, store.ProxyModeSOCKS5, false},
		{"255 bytes", Credentials{Username: strings.Repeat("u", 255), Password: strings.Repeat("p", 255)}, store.ProxyModeSOCKS5, true},
		{"256 bytes", Credentials{Username: strings.Repeat("u", 256), Password: "p"}, store.ProxyModeSOCKS5, false},
	}
	for _, tc := range cases {
		_, err := tc.c.Validated(tc.mode)
		if (err == nil) != tc.ok {
			t.Errorf("%s: err = %v", tc.name, err)
		}
	}
}

func TestCredentialsAreRedacted(t *testing.T) {
	c := Credentials{Username: "secret-user", Password: "secret-password"}
	var logged strings.Builder
	slog.New(slog.NewTextHandler(&logged, nil)).Info("proxy", "credentials", c)
	for _, out := range []string{fmt.Sprint(c), fmt.Sprintf("%v %+v %#v %s", c, c, c, c), logged.String(), (&Error{Kind: ErrConnectionFailed, Cause: errors.New("x")}).Error()} {
		if strings.Contains(out, "secret") {
			t.Fatalf("secret leaked: %q", out)
		}
	}
}

func TestMemoryStore(t *testing.T) {
	var m MemoryCredentialStore
	if _, err := m.Load(testUUID, false); !errors.Is(err, ErrCredentialsUnavailable) {
		t.Fatal(err)
	}
	_ = m.Save(Credentials{Username: "u", Password: "p"}, testUUID)
	if c, err := m.Load(testUUID, false); err != nil || c.Username != "u" {
		t.Fatal(err)
	}
	_ = m.Delete(testUUID)
	if _, err := m.Load(testUUID, false); err == nil {
		t.Fatal("deleted item still loads")
	}
}

func TestL10nKeys(t *testing.T) {
	cases := map[error]l10n.Key{
		ErrAuthenticationFailed:           l10n.KeyProxyAuthenticationFailed,
		ErrConnectionFailed:               l10n.KeyProxyConnectionFailed,
		ErrCredentialsUnavailable:         l10n.KeyProxyCredentialsUnavailable,
		ErrInvalidHost:                    l10n.KeyProxyInvalidHost,
		&Error{Kind: ErrConnectionFailed}: l10n.KeyProxyConnectionFailed,
	}
	for err, want := range cases {
		if got := L10nKey(err); got != want {
			t.Errorf("%v: %q, want %q", err, got, want)
		}
	}
	if L10nKey(errors.New("other")) != "" {
		t.Fatal("unknown error got a key")
	}
}
