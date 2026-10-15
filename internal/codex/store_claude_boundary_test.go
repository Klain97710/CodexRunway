package codex

import (
	"os"
	"path/filepath"
	"testing"
)

// Codex account loading reads only index.json. A claude- directory and
// claude-index.json beside it are not Codex accounts.
func TestLoadIndexDoesNotScanClaudeDirectories(t *testing.T) {
	home := t.TempDir()
	s := NewStore(filepath.Join(home, "codex"), home)
	root := s.Root()
	if err := os.MkdirAll(filepath.Join(root, "claude-abc"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "claude-index.json"), []byte(`{"version":1,"accounts":[{"id":"claude-abc","sortIndex":0}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "claude-abc", "auth.json"), []byte(`{"claudeAiOauth":{"accessToken":"test-access-token"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	idx, err := s.LoadIndex()
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Accounts) != 0 || idx.ActiveAccountID != "" {
		t.Fatalf("claude files were treated as codex accounts: %+v", idx.Accounts)
	}
}
