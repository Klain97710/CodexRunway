//go:build !darwin

package app

import (
	"os"

	"github.com/licoy/quosparr/internal/store"
)

// DevDomainEnv names a scratch data directory for manual runs: the files
// of this OS live under $HOME already, so a temp HOME is enough, but the
// variable also selects the System that does not touch the machine.
const DevDomainEnv = "QUOSPARR_PREFS_DOMAIN"

// OpenStore returns the preferences store, and whether dev mode is on.
func OpenStore() (*store.Store, bool, error) {
	st, err := store.Open()
	return st, os.Getenv(DevDomainEnv) != "", err
}
