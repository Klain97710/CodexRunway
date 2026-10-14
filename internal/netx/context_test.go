package netx

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/store"
)

const testUUID = "0F1E2D3C-4B5A-6978-8899-AABBCCDDEEFF"

func manualConfig(mode store.ProxyMode, host string, port int, auth bool) store.NetworkProxyConfiguration {
	c := store.NetworkProxyConfiguration{Mode: mode, Host: host, Port: port, UsesAuthentication: auth}
	if auth {
		c.CredentialID = testUUID
	}
	return c
}

func tlsOption(srv *httptest.Server) Option {
	pool := x509.NewCertPool()
	pool.AddCert(srv.Certificate())
	return WithTLSConfig(&tls.Config{RootCAs: pool})
}

func get(t *testing.T, c *Context, rawURL string) (string, error) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.Client(Standard).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b), nil
}

func TestHTTPProxyCarriesPlainAndTLSRequests(t *testing.T) {
	for _, auth := range []bool{false, true} {
		user, pass := "", ""
		var creds *Credentials
		if auth {
			user, pass = "fixture-user", "fixture-password"
			creds = &Credentials{Username: user, Password: pass}
		}
		proxy := newHTTPProxy(t, user, pass)
		host, port := proxy.addr()
		plain := httptest.NewServer(okHandler("plain"))
		secure := httptest.NewTLSServer(okHandler("secure"))
		c, err := NewContext(manualConfig(store.ProxyModeHTTP, host, port, auth), creds, tlsOption(secure))
		if err != nil {
			t.Fatal(err)
		}
		if body, err := get(t, c, plain.URL); err != nil || body != "plain" {
			t.Fatalf("auth=%v plain: %q %v", auth, body, err)
		}
		if body, err := get(t, c, secure.URL); err != nil || body != "secure" {
			t.Fatalf("auth=%v tls: %q %v", auth, body, err)
		}
		if proxy.forwarded.Load() != 1 || proxy.connects.Load() != 1 {
			t.Fatalf("auth=%v proxy saw forwarded=%d connects=%d", auth, proxy.forwarded.Load(), proxy.connects.Load())
		}
		if auth && proxy.rejected.Load() != 0 {
			t.Fatalf("credentials were not sent preemptively: rejected=%d", proxy.rejected.Load())
		}
		plain.Close()
		secure.Close()
	}
}

func TestProxyAuthenticationFailures(t *testing.T) {
	bad := &Credentials{Username: "fixture-user", Password: "wrong-password"}
	t.Run("http plain", func(t *testing.T) {
		proxy := newHTTPProxy(t, "fixture-user", "fixture-password")
		host, port := proxy.addr()
		target := httptest.NewServer(okHandler("x"))
		defer target.Close()
		c, _ := NewContext(manualConfig(store.ProxyModeHTTP, host, port, true), bad)
		if _, err := get(t, c, target.URL); !errors.Is(err, ErrAuthenticationFailed) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("http connect", func(t *testing.T) {
		proxy := newHTTPProxy(t, "fixture-user", "fixture-password")
		host, port := proxy.addr()
		target := httptest.NewTLSServer(okHandler("x"))
		defer target.Close()
		c, _ := NewContext(manualConfig(store.ProxyModeHTTP, host, port, true), bad, tlsOption(target))
		if _, err := get(t, c, target.URL); !errors.Is(err, ErrAuthenticationFailed) {
			t.Fatalf("err = %v", err)
		}
		if proxy.connects.Load() != 0 {
			t.Fatal("tunnel was opened with wrong credentials")
		}
	})
	t.Run("socks wrong password", func(t *testing.T) {
		proxy := newSOCKSProxy(t, "fixture-user", "fixture-password")
		host, port := proxy.addr()
		target := httptest.NewServer(okHandler("x"))
		defer target.Close()
		c, _ := NewContext(manualConfig(store.ProxyModeSOCKS5, host, port, true), bad)
		if _, err := get(t, c, target.URL); !errors.Is(err, ErrAuthenticationFailed) {
			t.Fatalf("err = %v", err)
		}
		if proxy.connects.Load() != 0 {
			t.Fatal("connection was relayed with wrong credentials")
		}
	})
}

func TestSOCKS5CarriesRequests(t *testing.T) {
	for _, auth := range []bool{false, true} {
		user, pass := "", ""
		var creds *Credentials
		if auth {
			user, pass = "fixture-user", "fixture-password"
			creds = &Credentials{Username: user, Password: pass}
		}
		proxy := newSOCKSProxy(t, user, pass)
		host, port := proxy.addr()
		secure := httptest.NewTLSServer(okHandler("secure"))
		c, err := NewContext(manualConfig(store.ProxyModeSOCKS5, host, port, auth), creds, tlsOption(secure))
		if err != nil {
			t.Fatal(err)
		}
		if body, err := get(t, c, secure.URL); err != nil || body != "secure" {
			t.Fatalf("auth=%v: %q %v", auth, body, err)
		}
		if proxy.connects.Load() != 1 {
			t.Fatalf("auth=%v relayed %d connections", auth, proxy.connects.Load())
		}
		secure.Close()
	}
}

func TestUnavailableProxyNeverFallsBackToDirect(t *testing.T) {
	var hits atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1) }))
	defer target.Close()
	// A closed port: reserve one and release it.
	ln := httptest.NewServer(nil)
	host, port := hostPortOf(ln.Listener.Addr().String())
	ln.Close()
	for _, mode := range []store.ProxyMode{store.ProxyModeHTTP, store.ProxyModeSOCKS5} {
		c, err := NewContext(manualConfig(mode, host, port, false), nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := get(t, c, target.URL); !errors.Is(err, ErrConnectionFailed) {
			t.Fatalf("%s: err = %v", mode, err)
		}
	}
	if hits.Load() != 0 {
		t.Fatalf("target was reached directly %d times", hits.Load())
	}
}

func TestSystemProxyErrorFailsInsteadOfConnectingDirectly(t *testing.T) {
	var hits atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1) }))
	defer target.Close()
	boom := errors.New("system proxy: resolution failed")
	c := SystemContext(WithSystemProxy(func(*url.URL) (*url.URL, error) { return nil, boom }))
	if _, err := get(t, c, target.URL); !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	if hits.Load() != 0 {
		t.Fatal("request went out directly after a system proxy failure")
	}
}

func TestSystemProxyResolverIsAskedPerRequest(t *testing.T) {
	proxy := newHTTPProxy(t, "", "")
	host, port := proxy.addr()
	target := httptest.NewServer(okHandler("ok"))
	defer target.Close()
	proxyURL := &url.URL{Scheme: "http", Host: net_JoinHostPort(host, port)}
	var useProxy atomic.Bool
	c := SystemContext(WithSystemProxy(func(*url.URL) (*url.URL, error) {
		if useProxy.Load() {
			return proxyURL, nil
		}
		return nil, nil
	}))
	if body, err := get(t, c, target.URL); err != nil || body != "ok" || proxy.forwarded.Load() != 0 {
		t.Fatalf("direct: %q %v forwarded=%d", body, err, proxy.forwarded.Load())
	}
	useProxy.Store(true)
	if body, err := get(t, c, target.URL); err != nil || body != "ok" || proxy.forwarded.Load() != 1 {
		t.Fatalf("proxied: %q %v forwarded=%d", body, err, proxy.forwarded.Load())
	}
}

func TestCancellationIsNotMappedToProxyErrors(t *testing.T) {
	block := make(chan struct{})
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-block:
		case <-r.Context().Done():
		}
	}))
	defer target.Close()
	defer close(block)
	proxy := newHTTPProxy(t, "", "")
	host, port := proxy.addr()
	c, _ := NewContext(manualConfig(store.ProxyModeHTTP, host, port, false), nil)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, target.URL, nil)
	_, err := c.Client(Standard).Do(req)
	if !errors.Is(err, context.DeadlineExceeded) || errors.Is(err, ErrConnectionFailed) {
		t.Fatalf("err = %v", err)
	}
}

func TestContextRequiresValidCredentials(t *testing.T) {
	cfg := manualConfig(store.ProxyModeHTTP, "localhost", 7890, true)
	if _, err := NewContext(cfg, nil); !errors.Is(err, ErrCredentialsUnavailable) {
		t.Fatalf("missing credentials: %v", err)
	}
	if _, err := NewContext(cfg, &Credentials{Username: "bad:user", Password: "x"}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("colon user: %v", err)
	}
	socks := manualConfig(store.ProxyModeSOCKS5, "localhost", 1080, true)
	long := make([]byte, 256)
	for i := range long {
		long[i] = 'x'
	}
	if _, err := NewContext(socks, &Credentials{Username: "u", Password: string(long)}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("long password: %v", err)
	}
	if _, err := NewContext(manualConfig(store.ProxyModeHTTP, "bad host", 1, false), nil); !errors.Is(err, ErrInvalidHost) {
		t.Fatalf("host: %v", err)
	}
	if _, err := NewContext(manualConfig(store.ProxyModeHTTP, "localhost", 0, false), nil); !errors.Is(err, ErrInvalidPort) {
		t.Fatalf("port: %v", err)
	}
}

func TestPolicyTimeoutsAndCookies(t *testing.T) {
	c, _ := NewContext(manualConfig(store.ProxyModeHTTP, "::1", 7890, false), nil)
	if got := c.Client(Standard).Timeout; got != 60*time.Second {
		t.Fatalf("standard timeout %v", got)
	}
	if c.Client(Standard).Jar == nil || c.Client(WithoutCookies).Jar != nil || c.Client(Update).Jar != nil {
		t.Fatal("cookie policy mismatch")
	}
	if c.Client(Update).Timeout <= time.Minute {
		t.Fatal("update timeout must be long")
	}
	if c.Client(Update) == c.Client(Update) {
		t.Fatal("update clients must not be shared")
	}
	proxy := c.proxyFunc()
	u, _ := proxy(&http.Request{URL: &url.URL{Scheme: "https", Host: "example.com"}})
	if u.Host != "[::1]:7890" || u.Scheme != "http" {
		t.Fatalf("proxy url %v", u)
	}
}

func TestPackageDoesNotTouchDefaultClient(t *testing.T) {
	before, beforeTransport := http.DefaultClient, http.DefaultTransport
	c, _ := NewContext(manualConfig(store.ProxyModeHTTP, "localhost", 7890, false), nil)
	Configure(c)
	defer Configure(SystemContext())
	if http.DefaultClient != before || http.DefaultTransport != beforeTransport {
		t.Fatal("default client or transport was replaced")
	}
	if http.DefaultClient.Transport != nil {
		t.Fatal("default client transport was modified")
	}
}

func TestCurrentBlockAndScopedContext(t *testing.T) {
	defer Configure(SystemContext())
	a := SystemContext()
	Configure(a)
	got, err := Current()
	if err != nil || got != a {
		t.Fatalf("current %v %v", got, err)
	}
	Block(nil)
	if _, err := Current(); !errors.Is(err, ErrNotReady) {
		t.Fatalf("blocked: %v", err)
	}
	if _, err := ClientFor(context.Background(), Standard); !errors.Is(err, ErrNotReady) {
		t.Fatalf("blocked client: %v", err)
	}
	scoped := SystemContext()
	got, err = Resolve(WithContext(context.Background(), scoped))
	if err != nil || got != scoped {
		t.Fatalf("scoped %v %v", got, err)
	}
	if err := ConfigureFrom(manualConfig(store.ProxyModeHTTP, "localhost", 1, true), &MemoryCredentialStore{}, false); !errors.Is(err, ErrCredentialsUnavailable) {
		t.Fatalf("configure: %v", err)
	}
	if _, err := Current(); !errors.Is(err, ErrNotReady) {
		t.Fatal("a failed ConfigureFrom must not unblock the network")
	}
}

func TestLoadContextReadsCredentialsFromStore(t *testing.T) {
	proxy := newHTTPProxy(t, "fixture-user", "fixture-password")
	host, port := proxy.addr()
	target := httptest.NewServer(okHandler("ok"))
	defer target.Close()
	creds := &MemoryCredentialStore{}
	if err := creds.Save(Credentials{Username: "fixture-user", Password: "fixture-password"}, testUUID); err != nil {
		t.Fatal(err)
	}
	c, err := LoadContext(manualConfig(store.ProxyModeHTTP, host, port, true), creds, false)
	if err != nil {
		t.Fatal(err)
	}
	if body, err := get(t, c, target.URL); err != nil || body != "ok" {
		t.Fatalf("%q %v", body, err)
	}
}
