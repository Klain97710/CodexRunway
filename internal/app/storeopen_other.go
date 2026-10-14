//go:build !darwin

package app

import (
	"github.com/licoy/quosparr/internal/personal"
	"os"
	"path/filepath"

	"github.com/licoy/quosparr/internal/store"
)

// DevDomainEnv names a scratch data directory for manual runs: the files
// of this OS live under $HOME already, so a temp HOME is enough, but the
// variable also selects the System that does not touch the machine.
const DevDomainEnv = "QUOSPARR_PREFS_DOMAIN"

// OpenStore returns the preferences store, and whether dev mode is on.
func OpenStore() (*store.Store, bool, error) {
	if personal.Development || os.Getenv(personal.SandboxEnv) != "" {
		home, err := personal.DevelopmentHome()
		if err != nil {
			return nil, true, err
		}
		return store.New(store.FileDomain{Dir: filepath.Join(home, "preferences")}, nil), true, nil
	}
	if os.Getenv(DevDomainEnv) != "" {
		return nil, true, personal.ErrSandbox
	}
	st, err := store.Open()
	return st, false, err
}
