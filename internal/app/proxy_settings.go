package app

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/store"
)

// ProxyCredentials are the username and password of an authenticated proxy.
// They never reach the preferences, a log or an error message.
type ProxyCredentials struct {
	Username string
	Password string
}

// String hides the secret from accidental formatting.
func (ProxyCredentials) String() string { return "ProxyCredentials{redacted}" }

// GoString hides the secret from %#v.
func (ProxyCredentials) GoString() string { return "ProxyCredentials{redacted}" }

// Errors of the proxy settings that the store package does not have.
var (
	// ErrProxyInvalidCredentials means the entered username or password
	// cannot be used with the proxy mode.
	ErrProxyInvalidCredentials = errors.New("invalid network proxy credentials")
	// ErrCredentialStoreFailed means the system credential store refused.
	ErrCredentialStoreFailed = errors.New("network proxy credential store failed")
)

// CredentialStore keeps proxy passwords outside the preferences, under the
// opaque id the proxy configuration refers to. The real implementation (the
// macOS Keychain, Windows Credential Manager, Linux Secret Service) is
// written elsewhere; MemoryCredentials stands in for tests and dev runs.
type CredentialStore interface {
	Save(id string, c ProxyCredentials) error
	// Load returns store.ErrProxyCredentialsUnavailable when id is unknown.
	Load(id string) (ProxyCredentials, error)
	// Delete removes id; an unknown id is not an error.
	Delete(id string) error
}

// MemoryCredentials is a CredentialStore in memory. It forgets everything
// at exit, so it is not for real use.
type MemoryCredentials struct {
	mu sync.Mutex
	m  map[string]ProxyCredentials
}

// NewMemoryCredentials returns an empty MemoryCredentials.
func NewMemoryCredentials() *MemoryCredentials {
	return &MemoryCredentials{m: map[string]ProxyCredentials{}}
}

// Save implements CredentialStore.
func (s *MemoryCredentials) Save(id string, c ProxyCredentials) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[id] = c
	return nil
}

// Load implements CredentialStore.
func (s *MemoryCredentials) Load(id string) (ProxyCredentials, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.m[id]
	if !ok {
		return ProxyCredentials{}, store.ErrProxyCredentialsUnavailable
	}
	return c, nil
}

// Delete implements CredentialStore.
func (s *MemoryCredentials) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, id)
	return nil
}

// Has reports whether id is stored (for tests).
func (s *MemoryCredentials) Has(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.m[id]
	return ok
}

// Count is the number of entries stored (for tests).
func (s *MemoryCredentials) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.m)
}

// UnavailableCredentials is the CredentialStore used until the real one is
// wired: it stores nothing, so a proxy that needs a password cannot be
// saved (and none is silently lost).
type UnavailableCredentials struct{}

// Save implements CredentialStore.
func (UnavailableCredentials) Save(string, ProxyCredentials) error { return ErrCredentialStoreFailed }

// Load implements CredentialStore.
func (UnavailableCredentials) Load(string) (ProxyCredentials, error) {
	return ProxyCredentials{}, store.ErrProxyCredentialsUnavailable
}

// Delete implements CredentialStore.
func (UnavailableCredentials) Delete(string) error { return nil }

// ProxyHooks lets other packages follow the proxy settings.
type ProxyHooks struct {
	// Applied is called on the main thread after a proxy configuration was
	// saved, with its credentials (nil without authentication).
	Applied func(store.NetworkProxyConfiguration, *ProxyCredentials)
	// Test checks the connection through the draft configuration; nil hides
	// the test button.
	Test func(ctx context.Context, c store.NetworkProxyConfiguration, creds *ProxyCredentials) error
}

// SetProxyHooks installs hooks; call before the window opens.
func (s *Settings) SetProxyHooks(h ProxyHooks) { s.proxyHooks = h }

// Proxy returns the current proxy configuration. While it is unusable it is
// the system proxy, for display only.
func (s *Settings) Proxy() store.NetworkProxyConfiguration { return s.proxy }

// ProxyDraft is the proxy form as typed.
type ProxyDraft struct {
	Mode         store.ProxyMode
	Host         string
	Port         string
	UsesAuth     bool
	Username     string
	Password     string
	CredentialID string
}

// NewProxyDraft fills a form from a stored configuration.
func NewProxyDraft(c store.NetworkProxyConfiguration) ProxyDraft {
	d := ProxyDraft{Mode: c.Mode, Host: c.Host, UsesAuth: c.UsesAuthentication, CredentialID: c.CredentialID}
	if c.Port > 0 {
		d.Port = strconv.Itoa(c.Port)
	}
	return d
}

// Configuration validates the form. The system mode ignores the rest.
func (d ProxyDraft) Configuration() (store.NetworkProxyConfiguration, error) {
	n, err := strconv.Atoi(strings.TrimSpace(d.Port))
	if d.Mode != store.ProxyModeSystem && err != nil {
		return store.NetworkProxyConfiguration{}, store.ErrProxyInvalidPort
	}
	return store.NetworkProxyConfiguration{
		Mode: d.Mode, Host: d.Host, Port: n, UsesAuthentication: d.UsesAuth, CredentialID: d.CredentialID,
	}.Validated()
}

func (d ProxyDraft) entered() *ProxyCredentials {
	if d.Username == "" && d.Password == "" {
		return nil
	}
	return &ProxyCredentials{Username: d.Username, Password: d.Password}
}

// validFor applies the rules of the Swift credential check.
func (c ProxyCredentials) validFor(mode store.ProxyMode) error {
	bad := func(s string) bool { return strings.ContainsAny(s, "\r\n\x00") }
	if c.Username == "" || bad(c.Username) || bad(c.Password) {
		return ErrProxyInvalidCredentials
	}
	if mode == store.ProxyModeHTTP && strings.Contains(c.Username, ":") {
		return ErrProxyInvalidCredentials
	}
	if mode == store.ProxyModeSOCKS5 &&
		(len(c.Username) < 1 || len(c.Username) > 255 || len(c.Password) < 1 || len(c.Password) > 255) {
		return ErrProxyInvalidCredentials
	}
	return nil
}

// credentialsFor returns the credentials the configuration needs: the
// entered ones, else the saved ones. nil without authentication.
func credentialsFor(cfg store.NetworkProxyConfiguration, entered *ProxyCredentials, cs CredentialStore) (*ProxyCredentials, error) {
	if cfg.Mode == store.ProxyModeSystem || !cfg.UsesAuthentication {
		return nil, nil
	}
	var c ProxyCredentials
	switch {
	case entered != nil:
		c = *entered
	case cfg.CredentialID != "":
		saved, err := cs.Load(cfg.CredentialID)
		if err != nil {
			return nil, err
		}
		c = saved
	default:
		return nil, ErrProxyInvalidCredentials
	}
	if err := c.validFor(cfg.Mode); err != nil {
		return nil, err
	}
	return &c, nil
}

// ProxySaveResult is a saved proxy configuration.
type ProxySaveResult struct {
	Config store.NetworkProxyConfiguration
	Creds  *ProxyCredentials
	// CleanupFailed is true when the configuration was saved but the old
	// password could not be removed from the credential store.
	CleanupFailed bool
}

// SaveProxy validates the draft and saves it, as the Swift app does: the
// credentials go to the credential store under a fresh id first, then the
// configuration; a failure removes the new credentials again. It may block
// on the credential store: call it off the main thread, and hand the result
// to ProxySaved on the main thread. It reads s.proxy only for the previous
// credential id, which the caller passes in prev.
func (s *Settings) SaveProxy(d ProxyDraft, prev store.NetworkProxyConfiguration) (ProxySaveResult, error) {
	cfg, err := d.Configuration()
	if err != nil {
		return ProxySaveResult{}, err
	}
	creds, err := credentialsFor(cfg, d.entered(), s.creds)
	if err != nil {
		return ProxySaveResult{}, err
	}
	cfg.CredentialID = ""
	if creds != nil {
		cfg.CredentialID = newUUID()
		if err := s.creds.Save(cfg.CredentialID, *creds); err != nil {
			return ProxySaveResult{}, fmt.Errorf("%w: %v", ErrCredentialStoreFailed, err)
		}
	}
	if err := s.st.SaveNetworkProxy(cfg); err != nil {
		if cfg.CredentialID != "" {
			if s.creds.Delete(cfg.CredentialID) != nil {
				return ProxySaveResult{}, ErrCredentialStoreFailed
			}
		}
		return ProxySaveResult{}, err
	}
	res := ProxySaveResult{Config: cfg, Creds: creds}
	if prev.CredentialID != "" && prev.CredentialID != cfg.CredentialID {
		res.CleanupFailed = s.creds.Delete(prev.CredentialID) != nil
	}
	return res, nil
}

// ProxySaved records a saved proxy configuration: main thread.
func (s *Settings) ProxySaved(r ProxySaveResult) {
	s.proxy = r.Config
	s.proxyErr = nil
	if s.proxyHooks.Applied != nil {
		s.proxyHooks.Applied(r.Config, r.Creds)
	}
	s.changed()
}

// LoadProxyCredentials returns the saved credentials of the current proxy
// configuration. It may block on the credential store.
func (s *Settings) LoadProxyCredentials(id string) (ProxyCredentials, error) {
	if id == "" {
		return ProxyCredentials{}, store.ErrProxyCredentialsUnavailable
	}
	return s.creds.Load(id)
}

// proxyErrorKey is the message of a proxy error.
func proxyErrorKey(err error) l10n.Key {
	switch {
	case errors.Is(err, store.ErrProxyInvalidHost):
		return l10n.KeyProxyInvalidHost
	case errors.Is(err, store.ErrProxyInvalidPort):
		return l10n.KeyProxyInvalidPort
	case errors.Is(err, ErrProxyInvalidCredentials):
		return l10n.KeyProxyInvalidCredentials
	case errors.Is(err, store.ErrProxyCredentialsUnavailable):
		return l10n.KeyProxyCredentialsUnavailable
	case errors.Is(err, ErrCredentialStoreFailed):
		return l10n.KeyProxyCredentialStoreFailed
	case errors.Is(err, store.ErrProxyInvalidConfiguration):
		return l10n.KeyProxyInvalidConfiguration
	}
	return l10n.KeyProxyConnectionFailed
}

// newUUID returns a random version 4 UUID in upper case, as Swift writes it.
func newUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("crypto/rand: " + err.Error())
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return strings.ToUpper(fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]))
}
