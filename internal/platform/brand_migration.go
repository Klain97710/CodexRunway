package platform

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

const legacyDataDirName = ".codex-runway"
const legacyLockFileName = "codex-runway.lock"

// AcquireLegacyLock excludes an installed pre-rename app while Quosparr is
// running. Fresh installations never create a directory under the old name.
func AcquireLegacyLock(home string, acquire func(string) (*Lock, error)) (*Lock, error) {
	root := filepath.Join(home, legacyDataDirName)
	info, err := os.Lstat(root)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, errors.New("legacy data path is not a directory")
	}
	return acquire(filepath.Join(root, legacyLockFileName))
}
