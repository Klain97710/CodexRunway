package sqliteruntime

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/licoy/quosparr/internal/platform"
)

// Each version has an immutable, hash-named directory, so an update never
// replaces a DLL that another process still has loaded. A concurrent first
// extraction can win publication; only an identical, private file is accepted.
func prepareLibrary(root string, data []byte, expected string) (string, error) {
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != expected || len(data) == 0 {
		return "", errors.New("sqlite runtime: embedded library checksum mismatch")
	}
	if !filepath.IsAbs(root) {
		return "", errors.New("sqlite runtime: data directory must be absolute")
	}
	for _, name := range []string{"", "runtime", "sqlite", expected} {
		root = filepath.Join(root, name)
		if err := privateDirectory(root); err != nil {
			return "", fmt.Errorf("sqlite runtime directory: %w", err)
		}
	}
	path := filepath.Join(root, "mygo-sqlite3.dll")
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		if err := publishLibrary(path, data); err != nil {
			if !errors.Is(err, os.ErrExist) {
				return "", fmt.Errorf("sqlite runtime extraction: %w", err)
			}
			log.Printf("sqlite runtime: concurrent publication detected, validating existing library: %v", err)
		}
	} else if err != nil {
		return "", err
	}
	if err := verifyLibrary(path, data); err != nil {
		return "", err
	}
	return path, nil
}

func publishLibrary(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".sqlite-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := platform.WriteFileAtomic(name, data, 0o600); err != nil {
		return err
	}
	// Publish without replacing: another caller may already be reading/loading
	// the immutable DLL, which Windows must never attempt to overwrite.
	return publishFile(name, path)
}

func privateDirectory(path string) error {
	info, err := os.Lstat(path)
	if err == nil && (!info.IsDir() || info.Mode()&os.ModeSymlink != 0) {
		return errors.New("expected a directory, not a file or symbolic link")
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return platform.EnsurePrivateDirectory(path)
}

func verifyLibrary(path string, expected []byte) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("sqlite runtime: cached library is not a regular file")
	}
	if err := platform.VerifyOwnerOnly(path, 0o600); err != nil {
		return fmt.Errorf("sqlite runtime permissions: %w", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !bytes.Equal(data, expected) {
		return errors.New("sqlite runtime: cached library checksum mismatch")
	}
	return nil
}
