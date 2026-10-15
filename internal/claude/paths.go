// Package claude reads Claude Code subscription credentials, usage and local
// session logs for Quosparr. It does not implement OAuth login and it never
// refreshes the account that is currently installed in Claude Code.
//
// Credential boundaries:
//
//   - Official credentials are the macOS keychain item or <config>/.credentials.json.
//     Only claudeAiOauth is interpreted. mcpOAuth is copied through unchanged
//     on an explicit account switch and is not stored in the account library.
//   - ~/.claude.json is read only for oauthAccount. A switch replaces that one key.
//   - Managed copies live in ~/.quosparr/accounts/claude-<id>/auth.json
//     (directory 0700, file 0600, atomic write). claude-index.json has no tokens.
//   - The current account's tokens are never refreshed. A non-current account
//     refresh writes only its managed copy.
package claude

import (
	"os"
	"path/filepath"
	"strings"
)

const credentialsFileName = ".credentials.json"

// ResolveConfigDir interprets CLAUDE_CONFIG_DIR. An empty value is ~/.claude.
// "~" and "~/x" expand against home. Anything else is cleaned and made absolute.
func ResolveConfigDir(envValue, home string) string {
	envValue = strings.TrimSpace(envValue)
	switch {
	case envValue == "":
		return filepath.Join(home, ".claude")
	case envValue == "~":
		return home
	case strings.HasPrefix(envValue, "~/"):
		return filepath.Join(home, envValue[2:])
	}
	if abs, err := filepath.Abs(envValue); err == nil {
		return filepath.Clean(abs)
	}
	return filepath.Clean(envValue)
}

// CredentialsFilePath is <config>/.credentials.json.
func CredentialsFilePath(configDir string) string {
	return filepath.Join(configDir, credentialsFileName)
}

// GlobalConfigPath is the .claude.json that holds oauthAccount.
// Claude Code documents the global file as <home>/.claude.json. When
// CLAUDE_CONFIG_DIR points elsewhere, a copy may live at <config>/.claude.json.
// An existing file wins. If neither exists, the default config dir uses the
// home file and a custom config dir uses the file inside that directory.
func GlobalConfigPath(configDir, home string) string {
	inConfig := filepath.Join(configDir, ".claude.json")
	inHome := filepath.Join(home, ".claude.json")
	switch {
	case fileExists(inConfig):
		return inConfig
	case fileExists(inHome):
		return inHome
	case filepath.Clean(configDir) == filepath.Clean(filepath.Join(home, ".claude")):
		return inHome
	default:
		return inConfig
	}
}

// GlobalConfigCandidates is GlobalConfigPath first, then the other well-known
// location when it is different. Identity reads try them in this order.
func GlobalConfigCandidates(configDir, home string) []string {
	primary := GlobalConfigPath(configDir, home)
	out := []string{primary}
	for _, p := range []string{filepath.Join(configDir, ".claude.json"), filepath.Join(home, ".claude.json")} {
		if filepath.Clean(p) != filepath.Clean(primary) {
			out = append(out, p)
		}
	}
	return out
}

// ProjectsDirs is the session roots to scan. <config>/projects is always
// listed. ~/.config/claude/projects is added only when that directory exists
// and is not the same path.
func ProjectsDirs(configDir, home string) []string {
	primary := filepath.Join(configDir, "projects")
	out := []string{primary}
	extra := filepath.Join(home, ".config", "claude", "projects")
	if filepath.Clean(extra) == filepath.Clean(primary) || !dirExists(extra) {
		return out
	}
	return append(out, extra)
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

func dirExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}
