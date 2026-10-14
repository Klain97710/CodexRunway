package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"unicode"
)

// Errors of the network proxy configuration. They mirror the Swift
// NetworkProxyError cases so the UI can map each to its message. A malformed
// configuration must never fall back to a direct connection, so callers treat
// any of these as "proxy not ready".
var (
	ErrProxyInvalidConfiguration   = errors.New("invalid network proxy configuration")
	ErrProxyInvalidHost            = errors.New("invalid network proxy host")
	ErrProxyInvalidPort            = errors.New("invalid network proxy port")
	ErrProxyCredentialsUnavailable = errors.New("network proxy credentials are unavailable")
)

// NetworkProxyConfiguration is the proxy settings document (UserDefaults key
// "runway.networkProxy"). It holds no secret: CredentialID is an opaque UUID
// that names the username/password entry in the system credential store.
type NetworkProxyConfiguration struct {
	Mode               ProxyMode `json:"mode"`
	Host               string    `json:"host"`
	Port               int       `json:"port"`
	UsesAuthentication bool      `json:"usesAuthentication"`
	// CredentialID is omitted from the JSON when empty (a nil optional in Swift).
	CredentialID string `json:"credentialID,omitempty"`

	extra map[string]json.RawMessage
}

// DefaultNetworkProxy returns the system proxy configuration, which is what a
// missing setting means.
func DefaultNetworkProxy() NetworkProxyConfiguration {
	return NetworkProxyConfiguration{Mode: ProxyModeSystem}
}

// DecodeNetworkProxy reads the stored JSON with the Swift synthesized Codable
// rules: mode, host, port and usesAuthentication are all required, credentialID
// is optional (null counts as absent), and unknown keys are kept for
// EncodeNetworkProxy. It does not validate the values; see Validated.
func DecodeNetworkProxy(data []byte) (NetworkProxyConfiguration, error) {
	c, err := decodeNetworkProxy(data)
	if err != nil {
		return NetworkProxyConfiguration{}, fmt.Errorf("%w: %w", ErrProxyInvalidConfiguration, err)
	}
	return c, nil
}

func decodeNetworkProxy(data []byte) (NetworkProxyConfiguration, error) {
	rest, err := parseObject(data)
	if err != nil {
		return NetworkProxyConfiguration{}, err
	}
	for _, key := range []string{"mode", "host", "port", "usesAuthentication"} {
		if raw, ok := rest[key]; !ok || string(raw) == "null" {
			return NetworkProxyConfiguration{}, corruptf("%q is missing", key)
		}
	}
	r := &reader{rest: rest}
	c := NetworkProxyConfiguration{Mode: ProxyModeSystem}
	c.Mode = readEnum(r, "mode", c.Mode, ProxyMode.Valid)
	c.Host, _ = r.string("host")
	c.Port = r.int("port", 0)
	c.UsesAuthentication = r.bool("usesAuthentication", false)
	c.CredentialID, _ = r.string("credentialID")
	if r.err != nil {
		return NetworkProxyConfiguration{}, r.err
	}
	if len(r.rest) > 0 {
		c.extra = r.rest
	}
	return c, nil
}

// EncodeNetworkProxy validates c and returns the JSON the Swift decoder reads
// back to the same configuration. Unknown keys kept by DecodeNetworkProxy are
// written back unchanged.
func EncodeNetworkProxy(c NetworkProxyConfiguration) ([]byte, error) {
	v, err := c.Validated()
	if err != nil {
		return nil, err
	}
	if v.UsesAuthentication && v.CredentialID == "" {
		return nil, ErrProxyCredentialsUnavailable
	}
	return encodeFields(v, v.extra)
}

// Validated returns the normalized configuration or an error, exactly like the
// Swift validated(): the system mode drops everything else; otherwise the port
// must be 1...65535, the host is normalized, the credential reference is
// cleared without authentication and must be a UUID with it.
//
// Unlike Swift, a host with non-ASCII characters is rejected instead of being
// converted to punycode (no IDNA table without a new dependency). Hosts that
// the Swift app saved are already ASCII, so stored values are unaffected.
func (c NetworkProxyConfiguration) Validated() (NetworkProxyConfiguration, error) {
	if c.Mode == ProxyModeSystem {
		return NetworkProxyConfiguration{Mode: ProxyModeSystem, extra: c.extra}, nil
	}
	if !c.Mode.Valid() {
		return NetworkProxyConfiguration{}, ErrProxyInvalidConfiguration
	}
	if c.Port < 1 || c.Port > 65_535 {
		return NetworkProxyConfiguration{}, ErrProxyInvalidPort
	}
	host, err := normalizeProxyHost(c.Host)
	if err != nil {
		return NetworkProxyConfiguration{}, err
	}
	c.Host = host
	if !c.UsesAuthentication {
		c.CredentialID = ""
	}
	if c.CredentialID != "" && !isUUID(c.CredentialID) {
		return NetworkProxyConfiguration{}, ErrProxyInvalidConfiguration
	}
	return c, nil
}

// checkLoaded applies the extra rule of the Swift store on load and save:
// authentication needs a credential reference.
func (c NetworkProxyConfiguration) checkLoaded() error {
	if c.UsesAuthentication && c.CredentialID == "" {
		return ErrProxyCredentialsUnavailable
	}
	return nil
}

// normalizeProxyHost trims and checks a proxy host, unwrapping "[v6]" and
// lowercasing names and IPv6 literals.
func normalizeProxyHost(raw string) (string, error) {
	host := strings.TrimSpace(raw)
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		host = host[1 : len(host)-1]
		if !strings.Contains(host, ":") {
			return "", ErrProxyInvalidHost
		}
	}
	if host == "" || len(host) > 253 || strings.ContainsFunc(host, forbiddenHostRune) {
		return "", ErrProxyInvalidHost
	}
	switch {
	case strings.Contains(host, ":"):
		addr, err := netip.ParseAddr(host)
		if err != nil || !addr.Is6() {
			return "", ErrProxyInvalidHost
		}
		return strings.ToLower(host), nil
	case strings.Trim(host, "0123456789.") == "":
		addr, err := netip.ParseAddr(host)
		if err != nil || !addr.Is4() {
			return "", ErrProxyInvalidHost
		}
		return host, nil
	}
	return normalizeDNSName(host)
}

func forbiddenHostRune(r rune) bool {
	return unicode.IsSpace(r) || unicode.IsControl(r) || strings.ContainsRune(`/\@?#%[]`, r)
}

// normalizeDNSName lowercases an ASCII host name and checks its labels. A
// single trailing dot is allowed.
func normalizeDNSName(host string) (string, error) {
	name := strings.ToLower(host)
	for _, label := range strings.Split(strings.TrimSuffix(name, "."), ".") {
		if !validDNSLabel(label) {
			return "", ErrProxyInvalidHost
		}
	}
	return name, nil
}

func validDNSLabel(label string) bool {
	if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
		return false
	}
	for i := 0; i < len(label); i++ {
		b := label[i]
		if (b < 'a' || b > 'z') && (b < '0' || b > '9') && b != '-' {
			return false
		}
	}
	return true
}

// isUUID reports whether s is a 8-4-4-4-12 hex UUID in either case, which is
// what Swift's UUID(uuidString:) accepts.
func isUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i := 0; i < len(s); i++ {
		switch {
		case i == 8 || i == 13 || i == 18 || i == 23:
			if s[i] != '-' {
				return false
			}
		case !isHex(s[i]):
			return false
		}
	}
	return true
}

func isHex(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}
