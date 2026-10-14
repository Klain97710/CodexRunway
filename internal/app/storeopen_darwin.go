package app

import (
	"os"

	"github.com/licoy/quosparr/internal/store"
)

// DevDomainEnv names a scratch UserDefaults domain for manual runs. On
// macOS UserDefaults ignores $HOME, so a temp HOME alone would still read and
// write the real preferences; with this set the app uses only that domain
// (no legacy import) and the System that does not touch the machine.
const DevDomainEnv = "QUOSPARR_PREFS_DOMAIN"

// OpenStore returns the preferences store, and whether a dev domain is in
// use.
func OpenStore() (*store.Store, bool, error) {
	if name := os.Getenv(DevDomainEnv); name != "" {
		return store.New(store.NewUserDefaultsDomain(name), nil), true, nil
	}
	st, err := store.Open()
	return st, false, err
}
