package app

import (
	"github.com/licoy/quosparr/internal/personal"
	"os"
	"path/filepath"

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
