package personal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSandboxBoundary(t *testing.T) {
	real, _ := filepath.EvalSymlinks(t.TempDir())
	root, _ := filepath.EvalSymlinks(t.TempDir())
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateSandbox(root, real); err == nil {
		t.Fatal("unmarked directory accepted")
	}
	if err := os.WriteFile(filepath.Join(root, SandboxMarker), []byte(SandboxMarkerValue), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateSandbox(root, real); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateSandbox(root, root); err == nil {
		t.Fatal("real home accepted")
	}
	if err := os.Symlink(real, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateSandbox(root, real); err == nil {
		t.Fatal("symlink escape accepted")
	}
}

func TestSandboxRejectsProductionSubtree(t *testing.T) {
	real, _ := filepath.EvalSymlinks(t.TempDir())
	root := filepath.Join(real, ".codex", "fixtures")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, SandboxMarker), []byte(SandboxMarkerValue), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateSandbox(root, real); err == nil {
		t.Fatal("production subtree accepted")
	}
}

func TestDevelopmentHomeRejectsExternalClaudeConfig(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, SandboxMarker), []byte(SandboxMarkerValue), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(SandboxEnv, root)
	t.Setenv("CODEX_HOME", filepath.Join(root, ".codex"))
	t.Setenv("GROK_HOME", filepath.Join(root, ".grok"))
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	if _, err := DevelopmentHome(); err == nil {
		t.Fatal("external Claude config accepted")
	}
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(root, ".claude"))
	if got, err := DevelopmentHome(); err != nil || got != root {
		t.Fatalf("isolated provider directories rejected: %v", err)
	}
}
