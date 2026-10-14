package store

import (
	"bytes"
	"errors"
	"fmt"
	"log"
)

const PreviousDomainName = "com.github.codex-runway"
const PreviewDomainName = "com.github.quosparr"

// renamedDomain imports a missing setting from the previous product identity.
// Unlike the older v0.0.1 preferences import, this also preserves proxy config.
// A corrupt current or previous value is never replaced by a default.
type renamedDomain struct {
	current  Domain
	previous []Domain
}

func (d renamedDomain) Read(key string) ([]byte, bool, error) {
	data, found, err := d.current.Read(key)
	if err != nil || found {
		return data, found, err
	}
	for _, previous := range d.previous {
		data, found, err = previous.Read(key)
		if err != nil || found {
			break
		}
	}
	if err != nil || !found {
		return data, found, err
	}
	if err := validateMigratedSetting(key, data); err != nil {
		return nil, true, fmt.Errorf("previous app settings: %w", err)
	}
	if err := d.current.Write(key, data); err != nil {
		return nil, true, err
	}
	back, found, err := d.current.Read(key)
	if err != nil || !found || !bytes.Equal(back, data) {
		return nil, true, errors.New("renamed settings migration failed read-back verification")
	}
	log.Printf("migrated %s settings to Quosparr", key)
	return back, true, nil
}

func (d renamedDomain) Write(key string, data []byte) error { return d.current.Write(key, data) }

func validateMigratedSetting(key string, data []byte) error {
	switch key {
	case KeyPreferences:
		_, err := DecodePreferences(data)
		return err
	case KeyNetworkProxy:
		p, err := DecodeNetworkProxy(data)
		if err != nil {
			return err
		}
		p, err = p.Validated()
		if err != nil {
			return err
		}
		return p.checkLoaded()
	default:
		return errors.New("unknown setting during app rename")
	}
}
