package personal

import (
	"errors"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

var ErrSandbox = errors.New("migration development requires a marked, private synthetic-fixture sandbox without symlinks; use Scripts/run-personal-migration.py")

// DevelopmentHome checks the boundary before settings, locks or credentials
// are opened. It never changes HOME or trusts HOME to identify the real user.
func DevelopmentHome() (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", ErrSandbox
	}
	root, err := ValidateSandbox(os.Getenv(SandboxEnv), u.HomeDir)
	if err != nil {
		return "", err
	}
	for key, dir := range map[string]string{"CODEX_HOME": ".codex", "GROK_HOME": ".grok", "CLAUDE_CONFIG_DIR": ".claude"} {
		if value := os.Getenv(key); value != "" && filepath.Clean(value) != filepath.Join(root, dir) {
			return "", ErrSandbox
		}
	}
	return root, nil
}

// ValidateSandbox rejects symlinks anywhere in the fixture tree, including
// parent components. This is a local test boundary, not a hostile-user jail.
func ValidateSandbox(root, realHome string) (string, error) {
	if root == "" || !filepath.IsAbs(root) {
		return "", ErrSandbox
	}
	root = filepath.Clean(root)
	canonical, err := filepath.EvalSymlinks(root)
	if err != nil || canonical != root {
		return "", ErrSandbox
	}
	realHome, err = filepath.EvalSymlinks(realHome)
	if err != nil || within(root, realHome) {
		return "", ErrSandbox
	}
	for _, dir := range []string{".codex", ".codex-runway", ".quosparr", ".grok", ".claude", "Library"} {
		protected := filepath.Join(realHome, dir)
		if within(protected, root) || within(root, protected) {
			return "", ErrSandbox
		}
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() || info.Mode().Perm()&0o077 != 0 {
		return "", ErrSandbox
	}
	err = filepath.WalkDir(root, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.Type()&os.ModeSymlink != 0 {
			return ErrSandbox
		}
		return nil
	})
	if err != nil {
		return "", ErrSandbox
	}
	marker, err := os.ReadFile(filepath.Join(root, SandboxMarker))
	if err != nil || string(marker) != SandboxMarkerValue {
		return "", ErrSandbox
	}
	return root, nil
}

func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
