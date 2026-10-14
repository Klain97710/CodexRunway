// Package platform holds the small operating-system boundaries of
// Quosparr: the single-instance lock and the app-owned directory.
package platform

import (
	"github.com/licoy/quosparr/internal/personal"
	"os"
	"path/filepath"
)

const (
	// RunwayDirName is the app-owned directory under the user's home.
	RunwayDirName = personal.DataDir
	// LockFileName matches SingleInstanceGuard.lockFileName of the Swift app,
	// so the Swift and Go builds exclude each other.
	LockFileName = personal.LockFile
	// CodexDirName is the official Codex directory (read only for us).
	CodexDirName = ".codex"
)

// RunwayDir returns ~/.quosparr under home.
func RunwayDir(home string) string { return filepath.Join(home, RunwayDirName) }

// LockPath returns ~/.quosparr/quosparr.lock under home.
func LockPath(home string) string { return filepath.Join(RunwayDir(home), LockFileName) }

// CodexDir follows the CLI's CODEX_HOME override, otherwise ~/.codex.
// Relative overrides stay relative to the process working directory.
func CodexDir(home string) string {
	if configured := os.Getenv("CODEX_HOME"); configured != "" {
		return filepath.Clean(configured)
	}
	return filepath.Join(home, CodexDirName)
}

// Home returns the current user's home directory.
func Home() (string, error) { return os.UserHomeDir() }
