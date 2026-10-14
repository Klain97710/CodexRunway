package store

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/licoy/quosparr/internal/platform"
)

// DataDirName is the Quosparr directory in the user's home.
const DataDirName = ".quosparr"

// File names inside the data directory on platforms without UserDefaults. The
// general settings and the proxy configuration are separate files so that a
// damaged proxy file never touches the general settings.
const (
	preferencesFileName  = "preferences.json"
	networkProxyFileName = "network-proxy.json"
)

// DataDir returns ~/.quosparr.
func DataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, DataDirName), nil
}

// FileDomain keeps each key as one JSON file in Dir: KeyPreferences in
// preferences.json and KeyNetworkProxy in network-proxy.json. Files are 0600,
// a directory it creates is 0700, and writes are atomic. It is the storage on
// Windows and Linux.
type FileDomain struct {
	Dir string
}

func (d FileDomain) path(key string) (string, error) {
	switch key {
	case KeyPreferences:
		return filepath.Join(d.Dir, preferencesFileName), nil
	case KeyNetworkProxy:
		return filepath.Join(d.Dir, networkProxyFileName), nil
	}
	return "", fmt.Errorf("store: no file for key %q", key)
}

// Read implements Domain. A missing file is not found; any other error,
// including an empty file, is reported.
func (d FileDomain) Read(key string) ([]byte, bool, error) {
	path, err := d.path(key)
	if err != nil {
		return nil, false, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}

// Write implements Domain.
func (d FileDomain) Write(key string, data []byte) error {
	path, err := d.path(key)
	if err != nil {
		return err
	}
	return platform.WriteFileAtomic(path, data, 0o600)
}
