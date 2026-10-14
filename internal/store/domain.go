package store

import (
	"errors"
	"sync"
)

// Names shared with the Swift app.
const (
	// DomainName is the UserDefaults domain of the current Swift app.
	DomainName = "com.quosparr.app"
	// LegacyDomainName is the domain v0.0.1 and v0.0.2 used.
	LegacyDomainName = "dev.licoy.codex-runway"

	// KeyPreferences holds the general settings JSON.
	KeyPreferences = "runway.preferences"
	// KeyNetworkProxy holds the network proxy JSON.
	KeyNetworkProxy = "runway.networkProxy"
)

// ErrUnexpectedType is returned when a stored value is not Data (for example a
// string or a dictionary written by another tool). The value is not guessed at.
var ErrUnexpectedType = errors.New("stored value is not data")

// Domain is a key/value store of raw JSON bytes: a UserDefaults domain on
// macOS, a directory of files elsewhere.
type Domain interface {
	// Read returns the bytes stored under key. found is false when the key
	// does not exist at all; an existing but empty or unreadable value is
	// found (or an error), never "absent".
	Read(key string) (data []byte, found bool, err error)
	// Write replaces the value under key.
	Write(key string, data []byte) error
}

// MemoryDomain is an in-memory Domain for tests.
type MemoryDomain struct {
	mu     sync.Mutex
	values map[string][]byte
	writes int
}

// NewMemoryDomain returns an empty MemoryDomain.
func NewMemoryDomain() *MemoryDomain {
	return &MemoryDomain{values: map[string][]byte{}}
}

// Read implements Domain.
func (d *MemoryDomain) Read(key string) ([]byte, bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	v, ok := d.values[key]
	if !ok {
		return nil, false, nil
	}
	return append([]byte(nil), v...), true, nil
}

// Write implements Domain.
func (d *MemoryDomain) Write(key string, data []byte) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.values[key] = append([]byte(nil), data...)
	d.writes++
	return nil
}

// Put seeds a value without counting it as a write.
func (d *MemoryDomain) Put(key string, data []byte) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.values[key] = append([]byte(nil), data...)
}

// Writes returns how many times Write was called.
func (d *MemoryDomain) Writes() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.writes
}
