package store

import (
	"bytes"
	"errors"
	"fmt"
	"sync"
)

// Source tells where LoadPreferences got the settings from.
type Source int

// Sources of loaded settings.
const (
	// SourceDefaults means nothing was stored; the defaults were returned and
	// nothing was written.
	SourceDefaults Source = iota
	// SourceCurrent means the current domain held the settings.
	SourceCurrent
	// SourceLegacy means the current domain had no settings, the legacy domain
	// did, and they were copied into the current domain.
	SourceLegacy
)

// Store reads and writes the preferences and the proxy configuration.
//
// Loading never replaces a damaged value with defaults. When a stored value
// cannot be decoded, the Load method returns an error wrapping ErrCorrupt (or
// ErrProxyInvalidConfiguration) and writes nothing; callers should then refuse
// to save settings over it.
type Store struct {
	mu      sync.Mutex
	current Domain
	legacy  Domain
}

// New returns a Store over current. legacy may be nil; when set, it is the old
// domain preferences are migrated from once. The proxy configuration is never
// migrated.
func New(current, legacy Domain) *Store {
	return &Store{current: current, legacy: legacy}
}

// Open returns the Store for this platform: the Swift app's UserDefaults
// domains on macOS, files in ~/.quosparr elsewhere.
func Open() (*Store, error) {
	current, legacy, err := defaultDomains()
	if err != nil {
		return nil, err
	}
	return New(current, legacy), nil
}

// LoadPreferences returns the stored settings.
//
//   - Current domain has the key: decode it. A damaged value is an error, with
//     no fallback to the legacy domain or the defaults.
//   - Current domain has no key and the legacy domain has none: return the
//     defaults and write nothing.
//   - Current domain has no key and the legacy domain has one: validate it,
//     write the original bytes into the current domain, read them back to
//     confirm, and return them. The legacy domain is never modified.
func (s *Store) LoadPreferences() (Preferences, Source, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, found, err := s.current.Read(KeyPreferences)
	if err != nil {
		return Preferences{}, 0, fmt.Errorf("read preferences: %w", err)
	}
	if found {
		p, err := DecodePreferences(data)
		if err != nil {
			return Preferences{}, 0, fmt.Errorf("stored preferences: %w", err)
		}
		return p, SourceCurrent, nil
	}
	if s.legacy == nil {
		return DefaultPreferences(), SourceDefaults, nil
	}
	return s.migrateLegacyPreferences()
}

func (s *Store) migrateLegacyPreferences() (Preferences, Source, error) {
	data, found, err := s.legacy.Read(KeyPreferences)
	if err != nil {
		return Preferences{}, 0, fmt.Errorf("read legacy preferences: %w", err)
	}
	if !found {
		return DefaultPreferences(), SourceDefaults, nil
	}
	if _, err := DecodePreferences(data); err != nil {
		return Preferences{}, 0, fmt.Errorf("legacy preferences: %w", err)
	}
	if err := s.current.Write(KeyPreferences, data); err != nil {
		return Preferences{}, 0, fmt.Errorf("migrate legacy preferences: %w", err)
	}
	back, found, err := s.current.Read(KeyPreferences)
	if err != nil || !found || !bytes.Equal(back, data) {
		return Preferences{}, 0, errors.New("migrate legacy preferences: read-back does not match what was written")
	}
	p, err := DecodePreferences(back)
	if err != nil {
		return Preferences{}, 0, err
	}
	return p, SourceLegacy, nil
}

// SavePreferences normalizes and writes p. It fails, writing nothing, if p
// holds a value the Swift app could not decode.
func (s *Store) SavePreferences(p Preferences) error {
	data, err := EncodePreferences(p)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.current.Write(KeyPreferences, data)
}

// LoadNetworkProxy returns the stored proxy configuration, validated. No key
// means the system proxy. A damaged or invalid value is an error: a broken
// proxy setting must never turn into a direct connection.
func (s *Store) LoadNetworkProxy() (NetworkProxyConfiguration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, found, err := s.current.Read(KeyNetworkProxy)
	if err != nil {
		return NetworkProxyConfiguration{}, fmt.Errorf("%w: %w", ErrProxyInvalidConfiguration, err)
	}
	if !found {
		return DefaultNetworkProxy(), nil
	}
	c, err := DecodeNetworkProxy(data)
	if err != nil {
		return NetworkProxyConfiguration{}, err
	}
	if c, err = c.Validated(); err != nil {
		return NetworkProxyConfiguration{}, err
	}
	if err := c.checkLoaded(); err != nil {
		return NetworkProxyConfiguration{}, err
	}
	return c, nil
}

// SaveNetworkProxy validates and writes c. It fails, writing nothing, if c is
// invalid or needs a credential reference it does not have.
func (s *Store) SaveNetworkProxy(c NetworkProxyConfiguration) error {
	data, err := EncodeNetworkProxy(c)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.current.Write(KeyNetworkProxy, data)
}
