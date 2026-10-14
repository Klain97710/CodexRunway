//go:build darwin

package store

import (
	"errors"
	"fmt"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

const (
	coreFoundationPath    = "/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation"
	kCFStringEncodingUTF8 = 0x08000100
)

// coreFoundation holds the few CoreFoundation functions needed, bound with
// purego so that no cgo is required.
type coreFoundation struct {
	stringCreate func(alloc uintptr, cstr string, encoding uint32) uintptr
	dataCreate   func(alloc uintptr, bytes *byte, length int) uintptr
	dataLength   func(data uintptr) int
	dataBytes    func(data uintptr) unsafe.Pointer
	dataTypeID   func() uintptr
	typeID       func(ref uintptr) uintptr
	release      func(ref uintptr)
	copyAppValue func(key, appID uintptr) uintptr
	setAppValue  func(key, value, appID uintptr)
	synchronize  func(appID uintptr) bool
}

var (
	cfOnce sync.Once
	cf     coreFoundation
	cfErr  error
)

// loadCoreFoundation binds the functions once.
func loadCoreFoundation() (*coreFoundation, error) {
	cfOnce.Do(func() { cfErr = cf.bind() })
	return &cf, cfErr
}

func (c *coreFoundation) bind() error {
	lib, err := purego.Dlopen(coreFoundationPath, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return fmt.Errorf("load CoreFoundation: %w", err)
	}
	symbols := []struct {
		fptr any
		name string
	}{
		{&c.stringCreate, "CFStringCreateWithCString"},
		{&c.dataCreate, "CFDataCreate"},
		{&c.dataLength, "CFDataGetLength"},
		{&c.dataBytes, "CFDataGetBytePtr"},
		{&c.dataTypeID, "CFDataGetTypeID"},
		{&c.typeID, "CFGetTypeID"},
		{&c.release, "CFRelease"},
		{&c.copyAppValue, "CFPreferencesCopyAppValue"},
		{&c.setAppValue, "CFPreferencesSetAppValue"},
		{&c.synchronize, "CFPreferencesAppSynchronize"},
	}
	for _, s := range symbols {
		addr, err := purego.Dlsym(lib, s.name)
		if err != nil {
			return fmt.Errorf("load CoreFoundation symbol %s: %w", s.name, err)
		}
		purego.RegisterFunc(s.fptr, addr)
	}
	return nil
}

// newString makes a CFString the caller must release.
func (c *coreFoundation) newString(s string) uintptr {
	return c.stringCreate(0, s, kCFStringEncodingUTF8)
}

// UserDefaultsDomain is a Domain backed by a macOS UserDefaults (CFPreferences)
// application domain, for the current user and any host: the same place the
// Swift app's UserDefaults.standard reads and writes. Values are CFData holding
// the JSON bytes, as UserDefaults.set(Data, forKey:) stores them. It goes
// through the system API, never the plist file.
type UserDefaultsDomain struct {
	appID string
}

// NewUserDefaultsDomain returns the domain named appID, such as DomainName.
func NewUserDefaultsDomain(appID string) UserDefaultsDomain {
	return UserDefaultsDomain{appID: appID}
}

// Read implements Domain. A value that is not CFData is an error wrapping
// ErrUnexpectedType.
func (d UserDefaultsDomain) Read(key string) ([]byte, bool, error) {
	c, err := loadCoreFoundation()
	if err != nil {
		return nil, false, err
	}
	app := c.newString(d.appID)
	defer c.release(app)
	// Pick up changes made by another process (the Swift app) since last read.
	c.synchronize(app)
	k := c.newString(key)
	defer c.release(k)
	value := c.copyAppValue(k, app)
	if value == 0 {
		return nil, false, nil
	}
	defer c.release(value)
	if c.typeID(value) != c.dataTypeID() {
		return nil, true, fmt.Errorf("%s in %s: %w", key, d.appID, ErrUnexpectedType)
	}
	n := c.dataLength(value)
	if n == 0 {
		return []byte{}, true, nil
	}
	return append([]byte(nil), unsafe.Slice((*byte)(c.dataBytes(value)), n)...), true, nil
}

// Write implements Domain.
func (d UserDefaultsDomain) Write(key string, data []byte) error {
	c, err := loadCoreFoundation()
	if err != nil {
		return err
	}
	var first *byte
	if len(data) > 0 {
		first = &data[0]
	}
	value := c.dataCreate(0, first, len(data))
	if value == 0 {
		return errors.New("create CFData failed")
	}
	defer c.release(value)
	return d.set(c, key, value)
}

// Remove deletes key from the domain. The Go app itself never calls it; it
// exists so tests can clean up a throwaway domain.
func (d UserDefaultsDomain) Remove(key string) error {
	c, err := loadCoreFoundation()
	if err != nil {
		return err
	}
	return d.set(c, key, 0)
}

func (d UserDefaultsDomain) set(c *coreFoundation, key string, value uintptr) error {
	app := c.newString(d.appID)
	defer c.release(app)
	k := c.newString(key)
	defer c.release(k)
	c.setAppValue(k, value, app)
	if !c.synchronize(app) {
		return fmt.Errorf("write %s in %s: synchronize failed", key, d.appID)
	}
	return nil
}
