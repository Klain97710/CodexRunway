package claude

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"

	"github.com/licoy/quosparr/internal/platform"
)

// Remove deletes a non-current account. The credential directory is moved
// aside, the index is saved, and only then is the directory deleted. A failed
// step puts the index and the credential bytes back. The official credential
// is not touched.
func (s *Store) Remove(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	original, err := s.loadIndex()
	if err != nil {
		return err
	}
	if _, ok := original.Account(id); !ok {
		return &AccountNotFoundError{ID: id}
	}
	if original.CurrentAccountID == id {
		return &CurrentAccountCannotBeRemovedError{ID: id}
	}
	next := original
	next.Accounts = make([]ManagedAccount, 0, len(original.Accounts))
	for _, a := range original.Accounts {
		if a.ID != id {
			next.Accounts = append(next.Accounts, a)
		}
	}
	dir := s.AccountDir(id)
	snapshot, had, err := s.credentialSnapshot(id)
	if err != nil {
		return err
	}
	tombstone := filepath.Join(s.root, ".claude-remove-"+randomToken())
	moved := false
	err = func() error {
		if _, statErr := os.Stat(dir); statErr == nil {
			if err := os.Rename(dir, tombstone); err != nil {
				return err
			}
			moved = true
		}
		if err := s.saveIndex(next); err != nil {
			return err
		}
		if moved {
			return os.RemoveAll(tombstone)
		}
		return nil
	}()
	if err == nil {
		return nil
	}
	if rbErr := s.rollbackRemoval(id, original, dir, tombstone, moved, snapshot, had); rbErr != nil {
		return errors.New("claude: account removal could not be rolled back")
	}
	return errors.New("claude: unable to remove the account")
}

func (s *Store) credentialSnapshot(id string) ([]byte, bool, error) {
	data, err := os.ReadFile(s.CredentialPath(id))
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	return data, err == nil, err
}

func (s *Store) rollbackRemoval(id string, original AccountIndex, dir, tombstone string, moved bool, snapshot []byte, had bool) error {
	if _, err := os.Stat(tombstone); moved && err == nil {
		if _, statErr := os.Stat(dir); statErr == nil {
			return errors.New("claude: credential directory already exists")
		}
		if err := os.Rename(tombstone, dir); err != nil {
			return err
		}
	}
	if err := s.saveIndex(original); err != nil {
		return err
	}
	if had {
		if err := s.restoreCredential(id, snapshot); err != nil {
			return err
		}
	}
	reloaded, err := s.loadIndex()
	if err != nil || !indexBytesEqual(reloaded, original) {
		return errors.New("claude: index rollback could not be verified")
	}
	return nil
}

func (s *Store) restoreCredential(id string, data []byte) error {
	path := s.CredentialPath(id)
	current, readErr := os.ReadFile(path)
	if readErr != nil || !bytes.Equal(current, data) || platform.VerifyOwnerOnly(path, storeFilePerm) != nil {
		if err := s.saveCredential(id, data); err != nil {
			return err
		}
	}
	restored, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(restored, data) || platform.VerifyOwnerOnly(path, storeFilePerm) != nil {
		return errors.New("claude: credential rollback could not be verified")
	}
	return nil
}

func indexBytesEqual(a, b AccountIndex) bool {
	x, errA := marshalIndex(a)
	y, errB := marshalIndex(b)
	return errA == nil && errB == nil && bytes.Equal(x, y)
}

func randomToken() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "fallback"
	}
	return hex.EncodeToString(b[:])
}
