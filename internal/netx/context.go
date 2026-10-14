package netx

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/licoy/quosparr/internal/store"
)

// Policy selects timeouts and cookie handling, like RunwayNetworkPolicy.
type Policy int

const (
	// Standard keeps cookies in memory for the life of the context.
	Standard Policy = iota
	// WithoutCookies neither sends nor stores cookies.
	WithoutCookies
	// Update is for downloads: no cookies and long timeouts. Every Client(Update)
	// call returns a new client with its own connections; call
	// CloseIdleConnections on it when the transfer is over.
	Update
)

func (p Policy) requestTimeout() time.Duration {
	if p == Update {
		return 60 * time.Second
	}
	return 20 * time.Second
}

func (p Policy) resourceTimeout() time.Duration {
	if p == Update {
		return time.Hour
	}
	return 60 * time.Second
}

// Option customizes NewContext. It exists for tests.
type Option func(*Context)

// WithSystemProxy replaces the platform system proxy resolver.
func WithSystemProxy(f SystemProxyFunc) Option { return func(c *Context) { c.system = f } }

// WithTLSConfig sets the TLS client configuration of every transport, for
// tests that talk to a local TLS server.
func WithTLSConfig(cfg *tls.Config) Option { return func(c *Context) { c.tlsConfig = cfg } }

// Context is one immutable network configuration: a validated proxy setting
// plus its credentials. Requests started from it keep using it even after the
// application configures a newer one; the old Context is simply dropped once
// its callers finish.
type Context struct {
	cfg       store.NetworkProxyConfiguration
	creds     *Credentials
	system    SystemProxyFunc
	tlsConfig *tls.Config
	standard  *http.Client
	plain     *http.Client
}

// NewContext validates cfg and creds like the Swift RunwayNetworkContext:
// authentication requires credentials, which must be valid for the mode. The
// system mode ignores any credentials.
func NewContext(cfg store.NetworkProxyConfiguration, creds *Credentials, opts ...Option) (*Context, error) {
	validated, err := cfg.Validated()
	if err != nil {
		return nil, err
	}
	c := &Context{cfg: validated}
	if validated.UsesAuthentication {
		if creds == nil {
			return nil, ErrCredentialsUnavailable
		}
		checked, err := creds.Validated(validated.Mode)
		if err != nil {
			return nil, err
		}
		c.creds = &checked
	}
	for _, opt := range opts {
		opt(c)
	}
	if c.system == nil {
		c.system = DefaultSystemProxy()
	}
	c.standard = c.newClient(Standard)
	c.plain = c.newClient(WithoutCookies)
	return c, nil
}

// SystemContext is the default context: the system proxy settings.
func SystemContext(opts ...Option) *Context {
	c, _ := NewContext(store.DefaultNetworkProxy(), nil, opts...)
	return c
}

// LoadContext builds the Context for cfg, reading the credentials from creds
// when the configuration uses authentication. allowInteraction is passed to
// the credential store (false for background loads).
func LoadContext(cfg store.NetworkProxyConfiguration, creds CredentialStore, allowInteraction bool, opts ...Option) (*Context, error) {
	validated, err := cfg.Validated()
	if err != nil {
		return nil, err
	}
	if !validated.UsesAuthentication {
		return NewContext(validated, nil, opts...)
	}
	if creds == nil || validated.CredentialID == "" {
		return nil, ErrCredentialsUnavailable
	}
	loaded, err := creds.Load(validated.CredentialID, allowInteraction)
	if err != nil {
		return nil, err
	}
	return NewContext(validated, &loaded, opts...)
}

// Configuration returns the validated configuration of the context.
func (c *Context) Configuration() store.NetworkProxyConfiguration { return c.cfg }

// Client returns the HTTP client for policy. Standard and WithoutCookies
// clients are shared by the context; an Update client is new on every call.
// A request context carries cancellation and deadlines.
func (c *Context) Client(policy Policy) *http.Client {
	switch policy {
	case Standard:
		return c.standard
	case WithoutCookies:
		return c.plain
	}
	return c.newClient(policy)
}

// CloseIdleConnections drops the pooled connections of the shared clients.
func (c *Context) CloseIdleConnections() {
	c.standard.CloseIdleConnections()
	c.plain.CloseIdleConnections()
}

// Do sends req with the Standard client.
func (c *Context) Do(req *http.Request) (*http.Response, error) {
	return c.standard.Do(req)
}

func (c *Context) newClient(policy Policy) *http.Client {
	base := c.newTransport(policy)
	client := &http.Client{
		Transport: &mappingTransport{base: base, manual: c.cfg.Mode != store.ProxyModeSystem},
		Timeout:   policy.resourceTimeout(),
	}
	if policy == Standard {
		client.Jar, _ = cookiejar.New(nil)
	}
	return client
}

func (c *Context) newTransport(policy Policy) *http.Transport {
	timeout := policy.requestTimeout()
	t := &http.Transport{
		Proxy:                 c.proxyFunc(),
		DialContext:           (&net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          16,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   timeout,
		ResponseHeaderTimeout: timeout,
		ExpectContinueTimeout: time.Second,
		TLSClientConfig:       c.tlsConfig,
	}
	if c.cfg.Mode == store.ProxyModeHTTP {
		t.OnProxyConnectResponse = func(_ context.Context, _ *url.URL, _ *http.Request, res *http.Response) error {
			if res.StatusCode == http.StatusProxyAuthRequired {
				return errProxyAuthRequired
			}
			return nil
		}
	}
	return t
}

var errProxyAuthRequired = errors.New("proxy authentication required")

// proxyFunc returns the per-request proxy choice. A manual proxy is used for
// every request, loopback included, like the Swift connection proxy
// dictionary. The system mode asks the OS each time, because settings and PAC
// results change; its errors fail the request.
func (c *Context) proxyFunc() func(*http.Request) (*url.URL, error) {
	switch c.cfg.Mode {
	case store.ProxyModeHTTP, store.ProxyModeSOCKS5:
		proxy := manualProxyURL(c.cfg, c.creds)
		return func(*http.Request) (*url.URL, error) { return proxy, nil }
	}
	return func(req *http.Request) (*url.URL, error) { return c.system(req.URL) }
}

func manualProxyURL(cfg store.NetworkProxyConfiguration, creds *Credentials) *url.URL {
	scheme := "http"
	if cfg.Mode == store.ProxyModeSOCKS5 {
		scheme = "socks5"
	}
	u := &url.URL{Scheme: scheme, Host: net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))}
	if creds != nil {
		u.User = url.UserPassword(creds.Username, creds.Password)
	}
	return u
}

// mappingTransport turns transport failures of a manual proxy into the
// ErrConnectionFailed / ErrAuthenticationFailed kinds, as the Swift
// mappedError does. The system mode passes errors through unchanged.
type mappingTransport struct {
	base   *http.Transport
	manual bool
}

func (m *mappingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := m.base.RoundTrip(req)
	if err != nil {
		return nil, m.mapError(req.Context(), err)
	}
	if m.manual && resp.StatusCode == http.StatusProxyAuthRequired {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
		_ = resp.Body.Close()
		return nil, &Error{Kind: ErrAuthenticationFailed}
	}
	return resp, nil
}

func (m *mappingTransport) CloseIdleConnections() { m.base.CloseIdleConnections() }

func (m *mappingTransport) mapError(ctx context.Context, err error) error {
	if !m.manual {
		return err
	}
	var proxyErr *Error
	if errors.As(err, &proxyErr) {
		return err
	}
	if errors.Is(err, errProxyAuthRequired) || socksAuthenticationFailure(err) {
		return &Error{Kind: ErrAuthenticationFailed, Cause: err}
	}
	if ctx.Err() != nil || errors.Is(err, context.Canceled) {
		return err
	}
	return &Error{Kind: ErrConnectionFailed, Cause: err}
}

// socksAuthenticationFailure recognizes the SOCKS5 username/password failures
// of the Go dialer (RFC 1929 rejection, or no method both sides accept).
func socksAuthenticationFailure(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "authentication failed") ||
		strings.Contains(msg, "no acceptable authentication methods")
}
