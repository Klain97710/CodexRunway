package app

import (
	"path/filepath"
	"testing"

	"github.com/egoist/mygo"
)

func TestDirectMygoPaths(t *testing.T) {
	home := t.TempDir()
	if err := DirectMygoPaths(mygo.App, home); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(home, ".codex-runway")
	for name, want := range map[mygo.PathName]string{
		mygo.PathUserData: root,
		mygo.PathCache:    filepath.Join(root, "cache"),
		mygo.PathLogs:     filepath.Join(root, "logs"),
	} {
		got, err := mygo.App.Path(name)
		if err != nil || got != want {
			t.Errorf("Path(%s) = %q, %v; want %q", name, got, err, want)
		}
	}
}
