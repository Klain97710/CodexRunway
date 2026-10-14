package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func linuxBinary() []byte {
	b := make([]byte, 64)
	copy(b, "\x7fELF")
	b[4] = 2
	b[5] = 1
	b[6] = 1
	binary.LittleEndian.PutUint16(b[16:], 3)
	binary.LittleEndian.PutUint16(b[18:], 62)
	binary.LittleEndian.PutUint32(b[20:], 1)
	binary.LittleEndian.PutUint16(b[52:], 64)
	return b
}
func fixtureArchive(t *testing.T, file string, names []string) {
	t.Helper()
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	tw := tar.NewWriter(gz)
	for _, name := range names {
		data := linuxBinary()
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0755, Size: int64(len(data)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, b.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestDevelopmentPayloadValidationStillChecksArchitectureAndResources(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "mygo.json")
	if err := os.WriteFile(cfg, []byte(`{"version":"1.0.0"}`), 0600); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(dir, archiveName("1.0.0", "linux-amd64"))
	args := []string{"verify-payload", "-config", cfg, "-dir", dir, "-targets", "linux-amd64"}
	fixtureArchive(t, archive, []string{"quosparr", "libmygo-sqlite3.so"})
	if err := run(args); err != nil {
		t.Fatal(err)
	}
	production := append([]string(nil), args...)
	production[0] = "verify"
	if err := run(production); err == nil {
		t.Fatal("production verification accepted an unsigned development archive")
	}
	fixtureArchive(t, archive, []string{"quosparr"})
	if err := run(args); err == nil {
		t.Fatal("development archive accepted without SQLite")
	}
	wrongArch := filepath.Join(dir, archiveName("1.0.0", "linux-arm64"))
	fixtureArchive(t, wrongArch, []string{"quosparr", "libmygo-sqlite3.so"})
	args[len(args)-1] = "linux-arm64"
	if err := run(args); err == nil {
		t.Fatal("development archive accepted with wrong architecture")
	}
}
func TestManifestAuthenticatesFinalArchive(t *testing.T) {
	dir := t.TempDir()
	pub, key, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	var cfg config
	cfg.Version = "1.0.0"
	cfg.Updates.PublicKey = base64.StdEncoding.EncodeToString(pub)
	cfg.Updates.GitHub = "Klain97710/CodexRunway"
	cfg.Updates.TagPrefix = "personal-v"
	t.Setenv("MYGO_UPDATE_PRIVATE_KEY", base64.StdEncoding.EncodeToString(key))
	file := filepath.Join(dir, archiveName(cfg.Version, "linux-amd64"))
	fixtureArchive(t, file, []string{"quosparr", "libmygo-sqlite3.so"})
	if err := sign(cfg, dir, "linux-amd64"); err != nil {
		t.Fatal(err)
	}
	if err := verify(cfg, dir, "linux-amd64"); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(file, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.Write([]byte("tampered"))
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	if err := verify(cfg, dir, "linux-amd64"); err == nil {
		t.Fatal("tampered archive accepted")
	}
	other, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := signingKey(other); err == nil {
		t.Fatal("wrong signing key accepted")
	}
}
func TestArchiveRejectsDuplicatePayload(t *testing.T) {
	file := filepath.Join(t.TempDir(), "test.tar.gz")
	fixtureArchive(t, file, []string{"quosparr", "libmygo-sqlite3.so", "quosparr"})
	if err := verifyArchive(file, "linux-amd64"); err == nil {
		t.Fatal("duplicate payload accepted")
	}
}
func TestUnsafeArchivePaths(t *testing.T) {
	for _, name := range []string{"../escape", "..", "/absolute", "a/../../escape", "a\\escape"} {
		if err := safeEntry(&tar.Header{Name: name, Typeflag: tar.TypeReg}); err == nil {
			t.Errorf("unsafe entry accepted: %s", name)
		}
	}
	if err := safeEntry(&tar.Header{Name: "./quosparr", Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if err := safeEntry(&tar.Header{Name: "./", Typeflag: tar.TypeDir}); err != nil {
		t.Fatal(err)
	}
	if err := safeEntry(&tar.Header{Name: "inside/link", Linkname: "../../outside", Typeflag: tar.TypeSymlink}); err == nil {
		t.Fatal("unsafe link accepted")
	}
}
func TestSymlinkGraph(t *testing.T) {
	if err := verifyLinks(map[string]bool{"a": true, "a/file": true, "b": true}, map[string]string{"a": "b"}); err == nil {
		t.Fatal("symlink traversal accepted")
	}
	if err := verifyLinks(map[string]bool{"a": true, "b": true}, map[string]string{"a": "b", "b": "a"}); err == nil {
		t.Fatal("cycle accepted")
	}
	if err := verifyLinks(map[string]bool{"a": true, "b": true}, map[string]string{"a": "b"}); err != nil {
		t.Fatal(err)
	}
}

func TestManifestRejectsWrongDownloadHost(t *testing.T) {
	dir := t.TempDir()
	pub, key, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	var cfg config
	cfg.Version = "1.0.0"
	cfg.Updates.PublicKey = base64.StdEncoding.EncodeToString(pub)
	cfg.Updates.GitHub = "Klain97710/CodexRunway"
	cfg.Updates.TagPrefix = "personal-v"
	t.Setenv("MYGO_UPDATE_PRIVATE_KEY", base64.StdEncoding.EncodeToString(key))
	fixtureArchive(t, filepath.Join(dir, archiveName(cfg.Version, "linux-amd64")), []string{"quosparr", "libmygo-sqlite3.so"})
	if err := sign(cfg, dir, "linux-amd64"); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "update-linux-amd64.json")
	var m manifest
	if err := readJSON(file, &m); err != nil {
		t.Fatal(err)
	}
	m.URL = "https://wrong.example/Licoy/Quosparr/releases/download/v1.0.0/" + archiveName(cfg.Version, "linux-amd64")
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := verify(cfg, dir, "linux-amd64"); err == nil {
		t.Fatal("wrong download host accepted")
	}
}
