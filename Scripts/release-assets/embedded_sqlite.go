package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/licoy/quosparr/internal/sqliteruntime/assets/amd64"
	"github.com/licoy/quosparr/internal/sqliteruntime/assets/arm64"
)

func verifyEmbeddedSQLite(executable []byte, arch string) error {
	library, expected := amd64.Library, amd64.SHA256
	if arch == "arm64" {
		library, expected = arm64.Library, arm64.SHA256
	}
	sum := sha256.Sum256(library)
	if hex.EncodeToString(sum[:]) != expected {
		return errors.New("embedded SQLite reference checksum mismatch")
	}
	if err := verifyArchitecture(library, "windows", arch); err != nil {
		return err
	}
	if !bytes.Contains(executable, library) {
		return errors.New("Windows executable is missing its pinned embedded SQLite runtime")
	}
	return nil
}
