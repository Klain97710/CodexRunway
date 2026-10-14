package netx

import (
	"bytes"
	"encoding/json"
	"github.com/licoy/quosparr/internal/personal"
	"log/slog"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/licoy/quosparr/internal/store"
)

// Credentials are the proxy username and password. The JSON form is exactly
// what the Swift app keeps in the Keychain (keys "username" and "password").
// The type never prints its fields: String, GoString and LogValue are redacted.
type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (Credentials) String() string   { return "Credentials{<redacted>}" }
func (Credentials) GoString() string { return "netx.Credentials{<redacted>}" }

// LogValue keeps the secret out of slog output.
func (Credentials) LogValue() slog.Value { return slog.StringValue("<redacted>") }

// Validated applies the Swift NetworkProxyCredentials.validated(for:) rules:
// a non-empty user name, no newline or NUL, no ':' in an HTTP user name, and
// 1...255 UTF-8 bytes for both fields with SOCKS5.
func (c Credentials) Validated(mode store.ProxyMode) (Credentials, error) {
	if c.Username == "" || hasNewlineOrNUL(c.Username) || hasNewlineOrNUL(c.Password) {
		return Credentials{}, ErrInvalidCredentials
	}
	if mode == store.ProxyModeHTTP && strings.Contains(c.Username, ":") {
		return Credentials{}, ErrInvalidCredentials
	}
	if mode == store.ProxyModeSOCKS5 && (!between(len(c.Username), 1, 255) || !between(len(c.Password), 1, 255)) {
		return Credentials{}, ErrInvalidCredentials
	}
	return c, nil
}

func between(n, lo, hi int) bool { return n >= lo && n <= hi }

// hasNewlineOrNUL mirrors Swift's Character.isNewline plus "\0".
func hasNewlineOrNUL(s string) bool {
	return strings.ContainsFunc(s, func(r rune) bool {
		switch r {
		case 0, '\n', '\r', '\v', '\f', 0x85, 0x2028, 0x2029:
			return true
		}
		return false
	})
}

func encodeCredentials(c Credentials) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(c); err != nil {
		return nil, ErrCredentialStoreFailed
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

func decodeCredentials(data []byte) (Credentials, error) {
	var c Credentials
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&c); err != nil || !utf8.Valid(data) {
		return Credentials{}, ErrCredentialsUnavailable
	}
	// Swift's synthesized decoder requires both keys.
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(data, &keys); err != nil {
		return Credentials{}, ErrCredentialsUnavailable
	}
	for _, k := range []string{"username", "password"} {
		if raw, ok := keys[k]; !ok || string(raw) == "null" {
			return Credentials{}, ErrCredentialsUnavailable
		}
	}
	return c, nil
}

// CredentialStore keeps proxy credentials outside every preference file. Only
// the opaque UUID id (store.NetworkProxyConfiguration.CredentialID) is stored
// elsewhere. allowInteraction lets the system show a prompt (macOS Keychain
// access dialog); background loads pass false and fail instead of blocking.
type CredentialStore interface {
	Load(id string, allowInteraction bool) (Credentials, error)
	Save(c Credentials, id string) error
	Delete(id string) error
}

// MemoryCredentialStore is an in-memory CredentialStore for tests.
type MemoryCredentialStore struct {
	mu    sync.Mutex
	items map[string]Credentials
	// Err, when set, fails every call like an unavailable system store.
	Err error
}

func (m *MemoryCredentialStore) Load(id string, _ bool) (Credentials, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return Credentials{}, m.Err
	}
	if !validCredentialID(id) {
		return Credentials{}, ErrCredentialsUnavailable
	}
	c, ok := m.items[id]
	if !ok {
		return Credentials{}, ErrCredentialsUnavailable
	}
	return c, nil
}

func (m *MemoryCredentialStore) Save(c Credentials, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return m.Err
	}
	if !validCredentialID(id) {
		return ErrInvalidConfiguration
	}
	if m.items == nil {
		m.items = map[string]Credentials{}
	}
	m.items[id] = c
	return nil
}

func (m *MemoryCredentialStore) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return m.Err
	}
	delete(m.items, id)
	return nil
}

// validCredentialID reports whether id is a UUID (8-4-4-4-12 hex), the only
// shape the Swift store accepts.
func validCredentialID(id string) bool {
	if len(id) != 36 {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		switch {
		case i == 8 || i == 13 || i == 18 || i == 23:
			if c != '-' {
				return false
			}
		case !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'):
			return false
		}
	}
	return true
}

// rawSecrets is the platform seam: one opaque byte value per (service, id).
// get reports found=false (not an error) for a missing item.
type rawSecrets interface {
	get(id string, allowInteraction bool) (data []byte, found bool, err error)
	set(id string, data []byte) error
	del(id string) error
}

// systemStore applies the Swift ProxyCredentialStore rules on top of a
// platform secret backend.
type systemStore struct{ raw rawSecrets }

func (s systemStore) Load(id string, allowInteraction bool) (Credentials, error) {
	if !validCredentialID(id) {
		return Credentials{}, ErrCredentialsUnavailable
	}
	data, found, err := s.raw.get(id, allowInteraction)
	if err != nil || !found {
		return Credentials{}, ErrCredentialsUnavailable
	}
	return decodeCredentials(data)
}

func (s systemStore) Save(c Credentials, id string) error {
	if !validCredentialID(id) {
		return ErrInvalidConfiguration
	}
	data, err := encodeCredentials(c)
	if err != nil {
		return err
	}
	if err := s.raw.set(id, data); err != nil {
		return &Error{Kind: ErrCredentialStoreFailed, Cause: err}
	}
	return nil
}

func (s systemStore) Delete(id string) error {
	if !validCredentialID(id) {
		return ErrInvalidConfiguration
	}
	if err := s.raw.del(id); err != nil {
		return &Error{Kind: ErrCredentialStoreFailed, Cause: err}
	}
	return nil
}

// DefaultService is the credential service name of the shipped app, the same
// one the Swift app derives from its bundle identifier.
const DefaultService = personal.CredentialService

// NewCredentialStore opens the platform credential store under service (use
// DefaultService): the macOS Keychain, the Windows Credential Manager, or the
// Linux Secret Service. Nothing falls back to a plain file.
func NewCredentialStore(service string) CredentialStore {
	current := newPlatformSecrets(service)

	return systemStore{raw: current}
}
