package store

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestFileDomainReadMissing(t *testing.T) {
	d := FileDomain{Dir: filepath.Join(t.TempDir(), "none")}
	data, found, err := d.Read(KeyPreferences)
	if err != nil || found || data != nil {
		t.Errorf("got %q %v %v", data, found, err)
	}
	if _, err := os.Stat(d.Dir); !os.IsNotExist(err) {
		t.Error("Read must not create the directory")
	}
}

func TestFileDomainWriteRead(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".codex-runway")
	d := FileDomain{Dir: dir}
	if err := d.Write(KeyPreferences, []byte(`{"a":1}`)); err != nil {
		t.Fatal(err)
	}
	if err := d.Write(KeyNetworkProxy, []byte(`{"b":2}`)); err != nil {
		t.Fatal(err)
	}
	if err := d.Write(KeyPreferences, []byte(`{"a":3}`)); err != nil { // overwrite
		t.Fatal(err)
	}
	got, found, err := d.Read(KeyPreferences)
	if err != nil || !found || string(got) != `{"a":3}` {
		t.Errorf("preferences = %q %v %v", got, found, err)
	}
	got, _, _ = d.Read(KeyNetworkProxy)
	if string(got) != `{"b":2}` {
		t.Errorf("proxy = %q", got)
	}
	entries, _ := os.ReadDir(dir)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if !reflect.DeepEqual(names, []string{"network-proxy.json", "preferences.json"}) {
		t.Errorf("directory holds %v (no temp files may remain)", names)
	}
	if runtime.GOOS == "windows" {
		return
	}
	if fi, _ := os.Stat(dir); fi.Mode().Perm() != 0o700 {
		t.Errorf("dir mode = %v, want 0700", fi.Mode().Perm())
	}
	if fi, _ := os.Stat(filepath.Join(dir, "preferences.json")); fi.Mode().Perm() != 0o600 {
		t.Errorf("file mode = %v, want 0600", fi.Mode().Perm())
	}
}

func TestFileDomainKeepsExistingDirectoryMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX modes")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := (FileDomain{Dir: dir}).Write(KeyPreferences, []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(dir); fi.Mode().Perm() != 0o755 {
		t.Errorf("an existing directory's mode was changed to %v", fi.Mode().Perm())
	}
}

func TestFileDomainUnknownKey(t *testing.T) {
	d := FileDomain{Dir: t.TempDir()}
	if _, _, err := d.Read("runway.other"); err == nil {
		t.Error("read of an unknown key should fail")
	}
	if err := d.Write("../escape", []byte("x")); err == nil {
		t.Error("write of an unknown key should fail")
	}
}

func TestFileDomainReadError(t *testing.T) {
	dir := t.TempDir()
	// A directory where the file should be is an error, not "absent".
	if err := os.Mkdir(filepath.Join(dir, "preferences.json"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, found, err := (FileDomain{Dir: dir}).Read(KeyPreferences); err == nil || found {
		t.Errorf("found=%v err=%v", found, err)
	}
}

func TestStoreOnFiles(t *testing.T) {
	dir := t.TempDir()
	s := New(FileDomain{Dir: dir}, nil)
	p, src, err := s.LoadPreferences()
	if err != nil || src != SourceDefaults || !reflect.DeepEqual(p, DefaultPreferences()) {
		t.Fatalf("first load: %v %v", src, err)
	}
	p.Language, p.ShowsQuotaSummary = LanguageRussian, false
	if err := s.SavePreferences(p); err != nil {
		t.Fatal(err)
	}
	got, src, err := New(FileDomain{Dir: dir}, nil).LoadPreferences()
	if err != nil || src != SourceCurrent || !reflect.DeepEqual(got, p) {
		t.Errorf("reload: %+v %v %v", got, src, err)
	}

	// A hand-damaged file is an error and stays byte-for-byte as it was.
	path := filepath.Join(dir, "preferences.json")
	if err := os.WriteFile(path, []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.LoadPreferences(); !errors.Is(err, ErrCorrupt) {
		t.Errorf("damaged file: err = %v", err)
	}
	if b, _ := os.ReadFile(path); string(b) != "{broken" {
		t.Errorf("damaged file was rewritten: %q", b)
	}
	// An empty file is corrupt, not absent.
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.LoadPreferences(); !errors.Is(err, ErrCorrupt) {
		t.Errorf("empty file: err = %v", err)
	}
}

func TestProxyOnFilesIsSeparate(t *testing.T) {
	dir := t.TempDir()
	s := New(FileDomain{Dir: dir}, nil)
	if err := s.SaveNetworkProxy(NetworkProxyConfiguration{Mode: ProxyModeSOCKS5, Host: "h", Port: 1080}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "preferences.json")); !os.IsNotExist(err) {
		t.Error("saving the proxy created the general settings file")
	}
	b, err := os.ReadFile(filepath.Join(dir, "network-proxy.json"))
	if err != nil || !strings.Contains(string(b), `"socks5"`) {
		t.Errorf("proxy file = %q, %v", b, err)
	}
	// Damaging the proxy file does not affect the general settings.
	if err := os.WriteFile(filepath.Join(dir, "network-proxy.json"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.LoadNetworkProxy(); !errors.Is(err, ErrProxyInvalidConfiguration) {
		t.Errorf("err = %v", err)
	}
	if _, _, err := s.LoadPreferences(); err != nil {
		t.Errorf("preferences affected by the proxy file: %v", err)
	}
}

func TestDataDir(t *testing.T) {
	dir, err := DataDir()
	if err != nil || filepath.Base(dir) != ".codex-runway" {
		t.Errorf("DataDir = %q, %v", dir, err)
	}
}
