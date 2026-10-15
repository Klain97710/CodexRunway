package claude

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"

	"github.com/licoy/quosparr/internal/platform"
)

// errOAuthConflict is one attempt that found .claude.json changing under us.
var errOAuthConflict = errors.New("claude: oauthAccount file changed")

// OAuthAccountReplacer replaces only the oauthAccount key of .claude.json.
// Other keys are preserved. A size or mtime change between the read and the
// write retries, at most three times.
//
// Public account-switch tools report that /status keeps showing the previous
// account unless oauthAccount is updated. That behavior was not verified
// against a live subscription login: this machine reaches Claude through an
// API gateway and has no subscription account.
type OAuthAccountReplacer struct {
	Path string
	// BeforeWrite runs after the file is read and before the final mtime
	// check. Tests use it to simulate a concurrent write. Production leaves
	// it nil.
	BeforeWrite func(path string) error
}

// Replace writes account as oauthAccount. A missing file is created mode 0600.
func (w OAuthAccountReplacer) Replace(account map[string]any) error {
	var last error
	for range 3 {
		err := w.once(account)
		if err == nil {
			return nil
		}
		if !errors.Is(err, errOAuthConflict) {
			return err
		}
		last = err
	}
	if last == nil {
		return ErrOAuthAccountConflict
	}
	return ErrOAuthAccountConflict
}

func (w OAuthAccountReplacer) once(account map[string]any) error {
	info, err := os.Stat(w.Path)
	if errors.Is(err, os.ErrNotExist) {
		return w.create(account)
	}
	if err != nil {
		return err
	}
	data, err := os.ReadFile(w.Path)
	if err != nil {
		return err
	}
	info2, err := os.Stat(w.Path)
	if err != nil {
		return err
	}
	if !sameFileStat(info, info2) {
		return errOAuthConflict
	}
	if w.BeforeWrite != nil {
		if err := w.BeforeWrite(w.Path); err != nil {
			return err
		}
	}
	info3, err := os.Stat(w.Path)
	if err != nil {
		return err
	}
	if !sameFileStat(info, info3) {
		return errOAuthConflict
	}
	next, err := replaceOAuthAccountBytes(data, account)
	if err != nil {
		return err
	}
	if err := platform.WriteFileAtomic(w.Path, next, info.Mode().Perm()); err != nil {
		return err
	}
	written, err := os.ReadFile(w.Path)
	if err != nil {
		return err
	}
	if !oauthAccountMatches(written, account) {
		return errOAuthConflict
	}
	return nil
}

func (w OAuthAccountReplacer) create(account map[string]any) error {
	body, err := replaceOAuthAccountBytes(nil, account)
	if err != nil {
		return err
	}
	if err := platform.WriteFileAtomic(w.Path, body, storeFilePerm); err != nil {
		return err
	}
	written, err := os.ReadFile(w.Path)
	if err != nil {
		return err
	}
	if !oauthAccountMatches(written, account) {
		return errOAuthConflict
	}
	return nil
}

func sameFileStat(a, b os.FileInfo) bool {
	return a.Size() == b.Size() && a.ModTime().Equal(b.ModTime())
}

func replaceOAuthAccountBytes(file []byte, account map[string]any) ([]byte, error) {
	root, err := rawObject(file)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(account)
	if err != nil {
		return nil, err
	}
	root["oauthAccount"] = encoded
	return marshalRawObject(root)
}

func oauthAccountMatches(file []byte, account map[string]any) bool {
	root, err := rawObject(file)
	if err != nil {
		return false
	}
	got, err := canonicalJSON(root["oauthAccount"])
	if err != nil {
		return false
	}
	want, err := json.Marshal(account)
	if err != nil {
		return false
	}
	want, err = canonicalJSON(want)
	if err != nil {
		return false
	}
	return bytes.Equal(got, want)
}

func canonicalJSON(raw []byte) ([]byte, error) {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	return json.Marshal(v)
}
