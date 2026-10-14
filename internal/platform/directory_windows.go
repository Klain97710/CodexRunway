//go:build windows

package platform

import (
	"errors"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

func ensurePrivateDirectory(path string) error {
	if err := mkdirPrivate(path); err != nil {
		return err
	}
	// Reapplying an inheritable ACL on every visit propagates it into children
	// while other callers create/protect them. Leave an already-private ACL alone.
	if err := verifyOwnerOnly(path, 0o700); err == nil {
		return nil
	}
	return ProtectOwnerOnly(path, 0o700)
}

// mkdirPrivate attaches the protected DACL during creation, so even concurrent
// callers never observe a newly created directory with inherited permissions.
// Existing ancestors are not modified, including the user's home directory.
func mkdirPrivate(path string) error {
	info, err := os.Stat(path)
	if err == nil {
		if !info.IsDir() {
			return &os.PathError{Op: "mkdir", Path: path, Err: windows.ERROR_DIRECTORY}
		}
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) || path == "" {
		return err
	}
	parent := filepath.Dir(path)
	if parent == path {
		return err
	}
	if err := mkdirPrivate(parent); err != nil {
		return err
	}
	sd, err := ownerSecurityDescriptor()
	if err != nil {
		return err
	}
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	attrs := windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), SecurityDescriptor: sd}
	err = windows.CreateDirectory(p, &attrs)
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		info, statErr := os.Stat(path)
		if statErr == nil && info.IsDir() {
			return nil // another caller created this directory atomically
		}
	}
	return err
}
