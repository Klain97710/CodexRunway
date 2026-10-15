package claude

import (
	"context"
	"errors"
)

// Official credential errors. They never include credential text.
var (
	// ErrNotLoggedIn means the keychain item is missing (security exit 44)
	// or the credentials file does not exist.
	ErrNotLoggedIn = errors.New("claude: not logged in")
	// ErrCredentialStore means the keychain or credentials file could not be used.
	ErrCredentialStore = errors.New("claude: credential store failed")
	// ErrKeychainPayloadTooLarge means the hex stdin line for security -i
	// would not fit its 4096-byte line buffer. The password is not passed on
	// argv as a fallback.
	ErrKeychainPayloadTooLarge = errors.New("claude: keychain payload is too large")
	// ErrKeychainVerify means the read-back access token did not match.
	ErrKeychainVerify = errors.New("claude: keychain read-back mismatch")
)

// securityLineLimit is the longest stdin command line security -i reads in
// one piece, without the newline. A longer line is cut at 4096 bytes and the
// rest runs as another command, so the write would store a truncated hex
// payload. Claude Code falls back to argv above this; Quosparr refuses.
const securityLineLimit = 4095

// OfficialCredentialStore reads and writes the Claude Code credential blob.
// Load returns ErrNotLoggedIn when the entry is absent.
type OfficialCredentialStore interface {
	Load(ctx context.Context) ([]byte, error)
	Save(ctx context.Context, data []byte) error
	Exists(ctx context.Context) (bool, error)
}

// fallbackStore uses primary (the keychain) and, only when that item is
// absent, secondary (the credentials file). A save goes where the login
// already is, and to the keychain when there is none, so no new plaintext
// file is created on macOS.
type fallbackStore struct {
	primary   OfficialCredentialStore
	secondary OfficialCredentialStore
}

func (f fallbackStore) Load(ctx context.Context) ([]byte, error) {
	if f.primary != nil {
		data, err := f.primary.Load(ctx)
		if err == nil {
			return data, nil
		}
		if !errors.Is(err, ErrNotLoggedIn) {
			return nil, err
		}
	}
	if f.secondary == nil {
		return nil, ErrNotLoggedIn
	}
	return f.secondary.Load(ctx)
}

func (f fallbackStore) Save(ctx context.Context, data []byte) error {
	target, err := f.target(ctx)
	if err != nil {
		return err
	}
	return target.Save(ctx, data)
}

// Remove deletes the entry Save would write. It is used to undo a switch
// that installed the first login.
func (f fallbackStore) Remove(ctx context.Context) error {
	target, err := f.target(ctx)
	if err != nil {
		return err
	}
	if r, ok := target.(interface{ Remove(context.Context) error }); ok {
		return r.Remove(ctx)
	}
	return ErrCredentialStore
}

func (f fallbackStore) target(ctx context.Context) (OfficialCredentialStore, error) {
	if f.primary != nil {
		exists, err := f.primary.Exists(ctx)
		if err != nil {
			return nil, err
		}
		if exists {
			return f.primary, nil
		}
	}
	if f.secondary != nil {
		exists, err := f.secondary.Exists(ctx)
		if err != nil {
			return nil, err
		}
		if exists || f.primary == nil {
			return f.secondary, nil
		}
	}
	if f.primary == nil {
		return nil, ErrCredentialStore
	}
	return f.primary, nil
}

func (f fallbackStore) Exists(ctx context.Context) (bool, error) {
	if f.primary != nil {
		ok, err := f.primary.Exists(ctx)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	if f.secondary == nil {
		return false, nil
	}
	return f.secondary.Exists(ctx)
}
