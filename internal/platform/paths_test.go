package platform

import (
	"path/filepath"
	"testing"
)

func TestCodexHomeOverride(t *testing.T) {
	home := t.TempDir()
	for _, configured := range []string{"", filepath.Join(t.TempDir(), "CLI data"), filepath.Join("relative", "codex")} {
		t.Run(configured, func(t *testing.T) {
			t.Setenv("CODEX_HOME", configured)
			want := configured
			if want == "" {
				want = filepath.Join(home, ".codex")
			}
			if got := CodexDir(home); got != want {
				t.Fatalf("CodexDir = %q, want %q", got, want)
			}
			if RunwayDir(home) != filepath.Join(home, ".codex-runway") {
				t.Fatal("CLI override changed app data directory")
			}
		})
	}
}
