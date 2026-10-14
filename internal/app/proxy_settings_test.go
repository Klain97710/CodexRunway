package app

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/licoy/quosparr/internal/store"
)

func proxyEnv(t *testing.T) *env {
	e := newEnv(t, nil)
	if err := e.s.Load(); err != nil {
		t.Fatal(err)
	}
	if err := e.s.LoadProxy(); err != nil {
		t.Fatal(err)
	}
	return e
}

func TestProxyDraftValidation(t *testing.T) {
	e := proxyEnv(t)
	for name, tc := range map[string]struct {
		d    ProxyDraft
		want error
	}{
		"empty host":    {ProxyDraft{Mode: store.ProxyModeHTTP, Host: "", Port: "8080"}, store.ErrProxyInvalidHost},
		"host with url": {ProxyDraft{Mode: store.ProxyModeHTTP, Host: "http://x", Port: "8080"}, store.ErrProxyInvalidHost},
		"port text":     {ProxyDraft{Mode: store.ProxyModeHTTP, Host: "p.local", Port: "abc"}, store.ErrProxyInvalidPort},
		"port zero":     {ProxyDraft{Mode: store.ProxyModeSOCKS5, Host: "p.local", Port: "0"}, store.ErrProxyInvalidPort},
		"port too big":  {ProxyDraft{Mode: store.ProxyModeSOCKS5, Host: "p.local", Port: "70000"}, store.ErrProxyInvalidPort},
		"auth no creds": {ProxyDraft{Mode: store.ProxyModeHTTP, Host: "p.local", Port: "8080", UsesAuth: true}, ErrProxyInvalidCredentials},
		"colon user":    {ProxyDraft{Mode: store.ProxyModeHTTP, Host: "p.local", Port: "8080", UsesAuth: true, Username: "a:b", Password: "x"}, ErrProxyInvalidCredentials},
	} {
		before := e.mem.Writes()
		_, err := e.s.SaveProxy(tc.d, e.s.Proxy())
		if !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", name, err, tc.want)
		}
		if e.mem.Writes() != before {
			t.Errorf("%s: something was written", name)
		}
		if proxyErrorKey(err) == "" {
			t.Errorf("%s: no message", name)
		}
	}
	// The system mode ignores the form.
	if _, err := e.s.SaveProxy(ProxyDraft{Mode: store.ProxyModeSystem, Host: "!!", Port: "x"}, e.s.Proxy()); err != nil {
		t.Errorf("system mode: %v", err)
	}
}

func TestProxySaveKeepsPasswordOutOfTheConfig(t *testing.T) {
	e := proxyEnv(t)
	d := ProxyDraft{Mode: store.ProxyModeHTTP, Host: "p.local", Port: "3128", UsesAuth: true, Username: "bob", Password: "s3cret-pw"}
	res, err := e.s.SaveProxy(d, e.s.Proxy())
	if err != nil {
		t.Fatal(err)
	}
	e.s.ProxySaved(res)
	if res.Config.CredentialID == "" || !e.creds.Has(res.Config.CredentialID) {
		t.Fatalf("credentials not stored under %q", res.Config.CredentialID)
	}
	raw, _, _ := e.mem.Read(store.KeyNetworkProxy)
	if strings.Contains(string(raw), "s3cret-pw") || strings.Contains(string(raw), "bob") {
		t.Errorf("the stored configuration contains credentials: %s", raw)
	}
	if got := e.s.Proxy(); got.Host != "p.local" || got.Port != 3128 {
		t.Errorf("proxy = %+v", got)
	}
	// Saved credentials are reused when the form keeps them blank.
	d2 := NewProxyDraft(e.s.Proxy())
	d2.Port = "3129"
	res2, err := e.s.SaveProxy(d2, e.s.Proxy())
	if err != nil {
		t.Fatal(err)
	}
	if res2.Creds == nil || res2.Creds.Username != "bob" {
		t.Error("saved credentials were not reused")
	}
	if e.creds.Has(res.Config.CredentialID) {
		t.Error("the previous credential entry was not removed")
	}
}

func TestProxyCredentialsAreRedactedInFormatting(t *testing.T) {
	c := ProxyCredentials{Username: "bob", Password: "s3cret-pw"}
	for _, s := range []string{fmt.Sprint(c), fmt.Sprintf("%v %+v %#v %s", c, c, c, c), fmt.Sprint(&c)} {
		if strings.Contains(s, "s3cret-pw") || strings.Contains(s, "bob") {
			t.Errorf("leaked: %s", s)
		}
	}
}

func TestProxySaveFailureRollsCredentialsBack(t *testing.T) {
	e := proxyEnv(t)
	e.mem.fail.Store(true)
	d := ProxyDraft{Mode: store.ProxyModeSOCKS5, Host: "p.local", Port: "1080", UsesAuth: true, Username: "bob", Password: "pw"}
	if _, err := e.s.SaveProxy(d, e.s.Proxy()); err == nil {
		t.Fatal("expected the save to fail")
	}
	if n := e.creds.Count(); n != 0 {
		t.Errorf("%d credential entries left behind", n)
	}
}

func TestProxyWithoutCredentialStoreSavesNothing(t *testing.T) {
	e := newEnv(t, nil)
	e.s = NewSettings(e.st, e.sys.system(), UnavailableCredentials{})
	_ = e.s.Load()
	_ = e.s.LoadProxy()
	before := e.mem.Writes()
	d := ProxyDraft{Mode: store.ProxyModeHTTP, Host: "p.local", Port: "3128", UsesAuth: true, Username: "bob", Password: "pw"}
	_, err := e.s.SaveProxy(d, e.s.Proxy())
	if !errors.Is(err, ErrCredentialStoreFailed) || e.mem.Writes() != before {
		t.Errorf("err = %v, writes %d -> %d", err, before, e.mem.Writes())
	}
	// A proxy without a password still works.
	if _, err := e.s.SaveProxy(ProxyDraft{Mode: store.ProxyModeHTTP, Host: "p.local", Port: "3128"}, e.s.Proxy()); err != nil {
		t.Error(err)
	}
}
