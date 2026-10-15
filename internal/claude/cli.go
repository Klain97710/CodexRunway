package claude

import (
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var cliVersionPattern = regexp.MustCompile(`\d+\.\d+\.\d+`)

// ParseCLIVersion returns the first x.y.z in claude --version output.
// An empty result means the User-Agent should use claude-cli/2.1.299 (external, cli).
func ParseCLIVersion(output string) string {
	return cliVersionPattern.FindString(output)
}

// ClaudeExecutable looks up claude on PATH.
func ClaudeExecutable() (string, error) { return exec.LookPath("claude") }

// IsClaudeExecutable reports whether path's base name is the Claude Code CLI.
func IsClaudeExecutable(path string) bool {
	name := strings.ToLower(filepath.Base(path))
	return name == "claude" || name == "claude.exe"
}
