package sqliteruntime

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/licoy/quosparr/internal/platform"
	"github.com/licoy/quosparr/internal/sqliteruntime/assets/amd64"
)

func TestEmbeddedLibraryExtractionAndReuse(t *testing.T) {
	root := filepath.Join(t.TempDir(), ".quosparr")
	path, err := prepareLibrary(root, amd64.Library, amd64.SHA256)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(data, amd64.Library) {
		t.Fatalf("extracted runtime differs: %v", err)
	}
	if err := platform.VerifyOwnerOnly(path, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := platform.VerifyOwnerOnly(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if next, err := prepareLibrary(root, amd64.Library, amd64.SHA256); err != nil || next != path {
		t.Fatalf("reuse: %q %v", next, err)
	}
	after, err := os.Stat(path)
	if err != nil || !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) {
		t.Fatalf("existing runtime was replaced: %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareLibrary(root, amd64.Library, amd64.SHA256); err != nil {
		t.Fatalf("missing cache must be recreated offline: %v", err)
	}
}

func TestEmbeddedLibraryRejectsCorruptionAndDirectoryErrors(t *testing.T) {
	root := filepath.Join(t.TempDir(), ".quosparr")
	if _, err := prepareLibrary(root, []byte("wrong architecture or damaged bytes"), amd64.SHA256); err == nil {
		t.Fatal("unverified embedded bytes accepted")
	}
	path, err := prepareLibrary(root, amd64.Library, amd64.SHA256)
	if err != nil {
		t.Fatal(err)
	}
	if err := platform.WriteFileAtomic(path, []byte("corrupted"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareLibrary(root, amd64.Library, amd64.SHA256); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("corrupt cache accepted: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "corrupted" {
		t.Fatalf("corrupt cache silently replaced: %v", err)
	}
	blocked := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocked, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareLibrary(blocked, amd64.Library, amd64.SHA256); err == nil {
		t.Fatal("directory creation error swallowed")
	}
}

func TestEmbeddedLibraryConcurrentFirstExtraction(t *testing.T) {
	root := filepath.Join(t.TempDir(), ".quosparr")
	results := make(chan error, 8)
	for range 8 {
		go func() {
			_, err := prepareLibrary(root, amd64.Library, amd64.SHA256)
			results <- err
		}()
	}
	for range 8 {
		if err := <-results; err != nil {
			t.Error(err)
		}
	}
}

func TestPublishedLibraryCannotBeReplaced(t *testing.T) {
	path, err := prepareLibrary(filepath.Join(t.TempDir(), ".quosparr"), amd64.Library, amd64.SHA256)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := publishLibrary(path, []byte("another concurrent publisher")); !errors.Is(err, os.ErrExist) {
		t.Fatalf("existing library publication must report file exists: %v", err)
	}
	after, err := os.Stat(path)
	if err != nil || !os.SameFile(before, after) {
		t.Fatalf("published library was replaced: %v", err)
	}
	if err := verifyLibrary(path, amd64.Library); err != nil {
		t.Fatal(err)
	}
}
