package platform

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// WriteFileAtomic writes data to a temporary file in the same directory, then
// renames it over path, so readers never see a partial file. The file gets
// perm; a directory it creates is 0700. On failure the original file is left
// untouched and the temporary file is removed.
func WriteFileAtomic(path string, data []byte, perm fs.FileMode) error {
	dir := filepath.Dir(path)
	private := perm.Perm()&0o077 == 0
	_, statErr := os.Stat(dir)
	if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return statErr
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if private && errors.Is(statErr, os.ErrNotExist) {
		if err := ProtectOwnerOnly(dir, 0o700); err != nil {
			return err
		}
	}
	tmp, err := createAtomicTemp(dir, "."+filepath.Base(path)+".*.tmp", private)
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if private {
		if err := restrictToOwner(name); err != nil {
			tmp.Close()
			return err
		}
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(name, perm); err != nil {
		return err
	}
	if private {
		// Verify before replacing: a permission-readback failure must leave
		// the original credential untouched. Same-volume replacement keeps
		// the temporary file's already protected security descriptor.
		if err := VerifyOwnerOnly(name, perm); err != nil {
			return err
		}
	}
	return replaceFile(name, path)
}
