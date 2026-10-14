package codex

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

var (
	ErrCredentialStoreUnsupported = errors.New("codex: credential storage mode is not supported; Quosparr requires cli_auth_credentials_store = file")
	ErrCredentialConfigInvalid    = errors.New("codex: cannot read credential storage mode from config.toml")
)

// CheckOfficialCredentialStore prevents treating a stale auth.json as the
// CLI login when the CLI uses a keyring, auto selection, or ephemeral auth.
// Re-read on every operation because the CLI config can change while running.
// No system credential store is accessed and the user's config is never edited.
func (s *Store) CheckOfficialCredentialStore() error {
	data, err := os.ReadFile(filepath.Join(filepath.Dir(s.officialAuth), "config.toml"))
	if errors.Is(err, os.ErrNotExist) {
		return nil // Codex defaults to file storage when unset.
	}
	if err != nil {
		return ErrCredentialConfigInvalid
	}
	var config struct {
		Mode *string `toml:"cli_auth_credentials_store"`
	}
	if err := toml.Unmarshal(data, &config); err != nil {
		// Parser errors may echo configuration values, including secrets.
		return ErrCredentialConfigInvalid
	}
	if config.Mode == nil || *config.Mode == "file" {
		return nil
	}
	switch *config.Mode {
	case "keyring", "auto", "ephemeral":
		return ErrCredentialStoreUnsupported
	default:
		return ErrCredentialConfigInvalid
	}
}
