//go:build !darwin

package claude

// OpenOfficialStore is the credentials file. Windows and Linux have no
// Claude Code keychain item.
func OpenOfficialStore(configDir, _, _, _ string) OfficialCredentialStore {
	return FileStore{Path: CredentialsFilePath(configDir)}
}
