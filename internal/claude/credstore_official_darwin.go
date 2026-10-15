//go:build darwin

package claude

import "os"

// OpenOfficialStore is the keychain item, then the credentials file when the
// item does not exist (security exit 44). envValue is the raw CLAUDE_CONFIG_DIR
// string, not the resolved path: an explicit value keeps the service suffix
// even when the directory is ~/.claude.
func OpenOfficialStore(configDir, home, account, envValue string) OfficialCredentialStore {
	return fallbackStore{
		primary: KeychainStore{
			Service: KeychainServiceName(envValue, home),
			Account: KeychainAccountName(account, os.Getenv("USER")),
		},
		secondary: FileStore{Path: CredentialsFilePath(configDir)},
	}
}
