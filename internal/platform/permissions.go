package platform

import (
	"io/fs"
	"os"
)

// EnsurePrivateDirectory creates an app-owned or credential directory and
// enforces 0700 on Unix or a protected, current-user-only DACL on Windows.
func EnsurePrivateDirectory(path string) error {
	return ensurePrivateDirectory(path)
}

// ProtectOwnerOnly enforces private permissions and verifies Windows' DACL.
func ProtectOwnerOnly(path string, perm fs.FileMode) error {
	if err := os.Chmod(path, perm); err != nil {
		return err
	}
	return restrictToOwner(path)
}

// VerifyOwnerOnly checks the effective platform permissions without changing them.
func VerifyOwnerOnly(path string, perm fs.FileMode) error {
	return verifyOwnerOnly(path, perm)
}
