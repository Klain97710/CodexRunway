//go:build darwin

package store

import (
	"errors"
	"reflect"
	"testing"
)

// testDomainID names the throwaway UserDefaults domains these tests use. Fixed
// names keep at most two leftover plists in ~/Library/Preferences: the
// system writes it asynchronously after the keys are gone. These tests never
// touch DomainName or LegacyDomainName, and must not run in parallel.
const testDomainID = "com.quosparr.app.gotest"

// testDomain returns a throwaway domain ("" or ".legacy" suffix) emptied, and
// empties it again when the test ends.
func testDomain(t *testing.T, suffix string) UserDefaultsDomain {
	t.Helper()
	d := NewUserDefaultsDomain(testDomainID + suffix)
	clear := func() {
		_ = d.Remove(KeyPreferences)
		_ = d.Remove(KeyNetworkProxy)
	}
	clear()
	t.Cleanup(clear)
	return d
}

func TestUserDefaultsReadAbsent(t *testing.T) {
	d := testDomain(t, "")
	data, found, err := d.Read(KeyPreferences)
	if err != nil || found || data != nil {
		t.Errorf("got %q %v %v", data, found, err)
	}
}

func TestUserDefaultsWriteReadRemove(t *testing.T) {
	d := testDomain(t, "")
	for _, payload := range [][]byte{
		[]byte(`{"language":"english"}`),
		[]byte(`{"s":"é 日本語 \u0000"}`),
		{0x00, 0xff, 0x7f, 0x80}, // arbitrary bytes survive: it is Data, not a string
		{},
	} {
		if err := d.Write(KeyPreferences, payload); err != nil {
			t.Fatal(err)
		}
		got, found, err := d.Read(KeyPreferences)
		if err != nil || !found || string(got) != string(payload) {
			t.Errorf("wrote %q, read %q %v %v", payload, got, found, err)
		}
	}
	if err := d.Remove(KeyPreferences); err != nil {
		t.Fatal(err)
	}
	if _, found, err := d.Read(KeyPreferences); err != nil || found {
		t.Errorf("after Remove: found=%v err=%v", found, err)
	}
}

func TestUserDefaultsKeysAreIndependent(t *testing.T) {
	d := testDomain(t, "")
	if err := d.Write(KeyPreferences, []byte("a")); err != nil {
		t.Fatal(err)
	}
	if err := d.Write(KeyNetworkProxy, []byte("b")); err != nil {
		t.Fatal(err)
	}
	a, _, _ := d.Read(KeyPreferences)
	b, _, _ := d.Read(KeyNetworkProxy)
	if string(a) != "a" || string(b) != "b" {
		t.Errorf("a=%q b=%q", a, b)
	}
}

func TestUserDefaultsRefusesNonDataValue(t *testing.T) {
	d := testDomain(t, "")
	c, err := loadCoreFoundation()
	if err != nil {
		t.Fatal(err)
	}
	app, key, str := c.newString(d.appID), c.newString(KeyPreferences), c.newString("not data")
	defer c.release(app)
	defer c.release(key)
	defer c.release(str)
	c.setAppValue(key, str, app) // a CFString, as `defaults write` would leave
	c.synchronize(app)

	data, found, err := d.Read(KeyPreferences)
	if !errors.Is(err, ErrUnexpectedType) || !found || data != nil {
		t.Errorf("got %q %v %v, want found with ErrUnexpectedType", data, found, err)
	}
	// Through the Store this is an error too, never defaults.
	if _, _, err := New(d, nil).LoadPreferences(); !errors.Is(err, ErrUnexpectedType) {
		t.Errorf("LoadPreferences: err = %v", err)
	}
	if _, err := New(d, nil).LoadNetworkProxy(); err != nil {
		t.Errorf("the proxy key is separate and still absent: %v", err)
	}
}

func TestUserDefaultsStoreRoundTripAndMigration(t *testing.T) {
	current, legacy := testDomain(t, ""), testDomain(t, ".legacy")
	s := New(current, legacy)

	if _, src, err := s.LoadPreferences(); err != nil || src != SourceDefaults {
		t.Fatalf("empty domains: %v %v", src, err)
	}
	const old = `{"language":"french","future":{"x":[1]}}`
	if err := legacy.Write(KeyPreferences, []byte(old)); err != nil {
		t.Fatal(err)
	}
	p, src, err := s.LoadPreferences()
	if err != nil || src != SourceLegacy || p.Language != LanguageFrench {
		t.Fatalf("migration: %+v %v %v", p, src, err)
	}
	if got, found, _ := current.Read(KeyPreferences); !found || string(got) != old {
		t.Errorf("current domain holds %q", got)
	}
	if got, found, _ := legacy.Read(KeyPreferences); !found || string(got) != old {
		t.Errorf("legacy domain changed to %q", got)
	}

	p.AccountPrivacyModeEnabled = true
	if err := s.SavePreferences(p); err != nil {
		t.Fatal(err)
	}
	back, src, err := s.LoadPreferences()
	if err != nil || src != SourceCurrent || !reflect.DeepEqual(back, p) {
		t.Errorf("reload: %+v %v %v", back, src, err)
	}
	if got, _, _ := legacy.Read(KeyPreferences); string(got) != old {
		t.Errorf("legacy domain changed after save: %q", got)
	}

	want := NetworkProxyConfiguration{Mode: ProxyModeHTTP, Host: "proxy.local", Port: 8080}
	if err := s.SaveNetworkProxy(want); err != nil {
		t.Fatal(err)
	}
	if got, err := s.LoadNetworkProxy(); err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("proxy: %+v %v", got, err)
	}
}

func TestOpenUsesRealDomainNamesOnly(t *testing.T) {
	// Open must be constructible without reading or writing anything.
	if _, err := Open(); err != nil {
		t.Fatal(err)
	}
	if DomainName != "com.quosparr.app" || LegacyDomainName != "dev.licoy.codex-runway" {
		t.Error("domain names changed")
	}
}
