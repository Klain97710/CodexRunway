//go:build !windows

package platform

import (
	"fmt"
	"io/fs"
	"os"
)

func restrictToOwner(string) error { return nil }

func ensurePrivateDirectory(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	return ProtectOwnerOnly(path, 0o700)
}

func verifyOwnerOnly(path string, perm fs.FileMode) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Mode().Perm() != perm.Perm() {
		return fmt.Errorf("platform: owner-only permissions were not applied")
	}
	return nil
}
