//go:build !darwin

package netx

import (
	"errors"

	"github.com/zalando/go-keyring"
)

// keyringSecrets stores the JSON value in the Windows Credential Manager or
// the Linux Secret Service through go-keyring.
type keyringSecrets struct{ service string }

func newPlatformSecrets(service string) rawSecrets { return keyringSecrets{service: service} }

func (k keyringSecrets) get(id string, _ bool) ([]byte, bool, error) {
	v, err := keyring.Get(k.service, id)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return []byte(v), true, nil
}

func (k keyringSecrets) set(id string, data []byte) error {
	return keyring.Set(k.service, id, string(data))
}

func (k keyringSecrets) del(id string) error {
	err := keyring.Delete(k.service, id)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}
