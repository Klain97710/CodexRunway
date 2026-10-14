//go:build !windows

package sqliteruntime

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/licoy/quosparr/internal/sqliteruntime/assets/amd64"
)

func TestEmbeddedLibraryRejectsSymlinks(t *testing.T) {
	directory := t.TempDir()
	root := filepath.Join(directory, "cache")
	if err := os.Symlink(directory, root); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareLibrary(root, amd64.Library, amd64.SHA256); err == nil {
		t.Fatal("symlink runtime directory accepted")
	}
	root = filepath.Join(directory, ".quosparr")
	path, err := prepareLibrary(root, amd64.Library, amd64.SHA256)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path, path+".real"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(path+".real", path); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareLibrary(root, amd64.Library, amd64.SHA256); err == nil {
		t.Fatal("symlink cached DLL accepted")
	}
}
