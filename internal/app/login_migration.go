package app

import (
	"fmt"
	"sync"
)

// legacyLoginItems handles only registrations belonging to previous app identities.
type legacyLoginState struct{ Exists, Enabled bool }
type legacyLoginItems interface {
	State() (legacyLoginState, error)
	Remove() error
}

type migratingLoginBackend struct {
	current LoginBackend
	legacy  legacyLoginItems
	// Windows/Linux can distinguish an existing, explicitly disabled entry
	// from a never-created one even though mygo reports both as not registered.
	currentDisabled func() (bool, error)
	once            sync.Once
	err             error
}

func (b *migratingLoginBackend) Status() (LoginStatus, error) {
	b.once.Do(func() { b.err = b.migrate() })
	if b.err != nil {
		return LoginUnavailable, b.err
	}
	return b.current.Status()
}

func (b *migratingLoginBackend) Set(on bool) error {
	if _, err := b.Status(); err != nil {
		return err
	}
	return b.current.Set(on)
}

// Register and verify the replacement before removing old records, so a
// failure cannot silently disable login. Already migrated registrations are
// not added again. A failed migration stays visible until the next launch.
func (b *migratingLoginBackend) migrate() error {
	previous, err := b.legacy.State()
	if err != nil {
		return fmt.Errorf("read legacy login item: %w", err)
	}
	if !previous.Exists {
		return nil
	}
	if previous.Enabled {
		if b.currentDisabled != nil {
			disabled, err := b.currentDisabled()
			if err != nil {
				return fmt.Errorf("read current login choice: %w", err)
			}
			previous.Enabled = !disabled
		}
	}
	if previous.Enabled {
		status, err := (LoginItem{Backend: b.current, CanMutateSys: true}).Apply(true)
		if err != nil {
			return fmt.Errorf("migrate legacy login item: %w", err)
		}
		// Pending approval is not a working replacement. Keep the old entry;
		// the settings page exposes approval and migration resumes next launch.
		if status != LoginEnabled {
			return nil
		}
	}
	if err := b.legacy.Remove(); err != nil {
		return fmt.Errorf("remove migrated login item: %w", err)
	}
	previous, err = b.legacy.State()
	if err != nil {
		return fmt.Errorf("verify legacy login item removal: %w", err)
	}
	if previous.Exists {
		return fmt.Errorf("remove migrated login item: %w", ErrLoginUnregister)
	}
	return nil
}

func loginBackendAsync(backend LoginBackend) bool {
	if wrapper, ok := backend.(*migratingLoginBackend); ok {
		return loginBackendAsync(wrapper.current)
	}
	_, ok := backend.(nativeLoginBackend)
	return ok
}
