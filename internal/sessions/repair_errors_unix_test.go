//go:build !windows

package sessions

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRepairRefusesUnreadableSessionTree(t *testing.T) {
	for _, folder := range []string{"sessions", "archived_sessions/nested"} {
		t.Run(folder, func(t *testing.T) {
			home := t.TempDir()
			dir := filepath.Join(home, folder)
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := os.Chmod(dir, 0o700); err != nil {
					t.Error(err)
				}
			})
			if err := os.Chmod(dir, 0); err != nil {
				t.Fatal(err)
			}
			if _, err := os.ReadDir(dir); !os.IsPermission(err) {
				t.Fatalf("fixture must deny enumeration: %v", err)
			}
			assertRepairFailsWithoutWriting(t, home)
		})
	}
}
