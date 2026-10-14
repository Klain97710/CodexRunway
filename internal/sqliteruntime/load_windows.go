//go:build windows && (amd64 || arm64)

package sqliteruntime

import (
	"os"
	"sync"

	"github.com/egoist/mygo/plugins/sqlite"
	"github.com/licoy/quosparr/internal/platform"
)

var load = sync.OnceValue(func() error {
	home, err := platform.Home()
	if err != nil {
		return err
	}
	path, err := prepareLibrary(platform.RunwayDir(home), embeddedLibrary, embeddedSHA256)
	if err != nil {
		return err
	}
	// SQLite loads once per process, after the embedded bytes are verified.
	// Always use this absolute path, never a DLL beside the EXE or on PATH.
	if err := os.Setenv("MYGO_SQLITE_LIBRARY", path); err != nil {
		return err
	}
	return sqlite.Load()
})

// Load prepares the embedded runtime offline and loads its native ABI once.
func Load() error { return load() }
