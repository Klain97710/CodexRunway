package codex

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCredentialModeParsing(t *testing.T) {
	for _, tc := range []struct {
		name, config string
		want         error
	}{
		{"missing", "", nil},
		{"file", "cli_auth_credentials_store = 'file' # explicit\n", nil},
		{"table is not root", "[profiles.test]\ncli_auth_credentials_store = 'keyring'\n", nil},
		{"multiline is not setting", "instructions = '''\ncli_auth_credentials_store = 'keyring'\n'''\n", nil},
		{"quoted root key", "\"cli_auth_credentials_store\" = \"keyring\"\n", ErrCredentialStoreUnsupported},
		{"auto", "cli_auth_credentials_store = 'auto'", ErrCredentialStoreUnsupported},
		{"ephemeral", "cli_auth_credentials_store = 'ephemeral'", ErrCredentialStoreUnsupported},
		{"unknown", "cli_auth_credentials_store = 'unknown-secret'", ErrCredentialConfigInvalid},
		{"empty", "cli_auth_credentials_store = ''", ErrCredentialConfigInvalid},
		{"wrong type", "cli_auth_credentials_store = 1", ErrCredentialConfigInvalid},
		{"duplicate", "cli_auth_credentials_store = 'file'\ncli_auth_credentials_store = 'keyring'", ErrCredentialConfigInvalid},
		{"malformed", "api_key = 'do-not-echo\n", ErrCredentialConfigInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			if err := os.MkdirAll(e.codexHome, 0o700); err != nil {
				t.Fatal(err)
			}
			if tc.config != "" {
				noErr(t, os.WriteFile(filepath.Join(e.codexHome, "config.toml"), []byte(tc.config), 0o600))
			}
			if err := e.store.CheckOfficialCredentialStore(); !errors.Is(err, tc.want) {
				t.Fatalf("mode error = %v, want %v", err, tc.want)
			}
		})
	}
}

// Every public official writer/reader must fail before treating a stale file
// as the active login; the managed copy remains available for quota polling.
func TestCredentialModeBlocksOfficialReadsAndWrites(t *testing.T) {
	for _, mode := range []string{"keyring", "auto", "ephemeral"} {
		t.Run(mode, func(t *testing.T) {
			e := newEnv(t)
			auth := sampleAuth(t, sampleOpt{accountID: "one", email: "a@example.com", refresh: "mode-fixture-refresh"})
			acc := upsert(t, e.store, auth, true)
			e.writeOfficial(t, auth)
			before := e.officialBytes(t)
			noErr(t, os.WriteFile(filepath.Join(e.codexHome, "config.toml"), []byte("cli_auth_credentials_store = '"+mode+"'"), 0o600))
			_, err := e.store.LoadOfficialAuth()
			assertUnsupportedMode(t, err)
			_, err = e.store.ImportOfficialAuth(true, testNow)
			assertUnsupportedMode(t, err)
			_, err = e.store.InstallCredentialAsOfficial(acc.ID, testNow)
			assertUnsupportedMode(t, err)
			_, err = e.store.RepairOfficialAuth(testNow)
			assertUnsupportedMode(t, err)
			sw := Switcher{Store: e.store, Hooks: SwitchHooks{Validate: func(context.Context, Auth) error { t.Fatal("unsupported switch reached network"); return nil }}}
			_, err = sw.SwitchTo(context.Background(), acc.ID)
			assertUnsupportedMode(t, err)
			auth.Tokens.RefreshToken = "rotated-mode-fixture-refresh"
			result, err := e.store.WriteRefreshedCredential(acc.ID, auth, testNow)
			assertUnsupportedMode(t, err)
			if !result.Managed || result.Official {
				t.Fatal("refresh did not preserve managed-only write")
			}
			idx, _, err := e.store.SyncFromOfficialAuth(testNow)
			assertUnsupportedMode(t, err)
			if idx.ActiveAccountID != "" || len(idx.Accounts) != 1 {
				t.Fatal("unsupported mode retained false current login or lost managed account")
			}
			if !bytes.Equal(e.officialBytes(t), before) {
				t.Fatal("unsupported mode modified official auth")
			}
			noErr(t, os.WriteFile(filepath.Join(e.codexHome, "config.toml"), []byte("cli_auth_credentials_store = 'file'"), 0o600))
			if _, err := e.store.LoadOfficialAuth(); err != nil {
				t.Fatal("mode change was not picked up", err)
			}
		})
	}
}

func assertUnsupportedMode(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, ErrCredentialStoreUnsupported) {
		t.Fatalf("expected explicit unsupported mode, got %v", err)
	}
}
