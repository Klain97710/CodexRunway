package store

import (
	"bytes"
	"testing"
)

func TestBrandSettingsMigrationPreservesAndValidates(t *testing.T) {
	for _, key := range []string{KeyPreferences, KeyNetworkProxy} {
		t.Run(key, func(t *testing.T) {
			current, previous := NewMemoryDomain(), NewMemoryDomain()
			data, err := EncodePreferences(DefaultPreferences())
			if key == KeyNetworkProxy {
				data, err = EncodeNetworkProxy(DefaultNetworkProxy())
			}
			if err != nil {
				t.Fatal(err)
			}
			previous.Put(key, data)
			domain := renamedDomain{current, []Domain{previous}}
			got, found, err := domain.Read(key)
			if err != nil || !found || !bytes.Equal(got, data) || current.Writes() != 1 || previous.Writes() != 0 {
				t.Fatal("settings were not copied once while preserving the old domain")
			}
			previous.Put(key, []byte("invalid"))
			if _, _, err := domain.Read(key); err != nil || current.Writes() != 1 {
				t.Fatal("existing new setting was not authoritative")
			}
			fresh := NewMemoryDomain()
			if _, _, err := (renamedDomain{fresh, []Domain{previous}}).Read(key); err == nil || fresh.Writes() != 0 {
				t.Fatal("corrupt legacy setting was copied or hidden")
			}
		})
	}
}

func TestBrandSettingsPreferLatestPreviewDomain(t *testing.T) {
	preview, older := NewMemoryDomain(), NewMemoryDomain()
	p := DefaultPreferences()
	p.Language = LanguageFrench
	latest, _ := EncodePreferences(p)
	original, _ := EncodePreferences(DefaultPreferences())
	preview.Put(KeyPreferences, latest)
	older.Put(KeyPreferences, original)
	current := NewMemoryDomain()
	got, _, err := (renamedDomain{current, []Domain{preview, older}}).Read(KeyPreferences)
	if err != nil || !bytes.Equal(got, latest) {
		t.Fatal("latest preview preferences were not preserved")
	}
	preview.Put(KeyPreferences, []byte("invalid"))
	if _, _, err := (renamedDomain{NewMemoryDomain(), []Domain{preview, older}}).Read(KeyPreferences); err == nil {
		t.Fatal("invalid preview preferences were hidden by older settings")
	}
	got, _, err = (renamedDomain{NewMemoryDomain(), []Domain{NewMemoryDomain(), older}}).Read(KeyPreferences)
	if err != nil || !bytes.Equal(got, original) {
		t.Fatal("missing preview prevented the original domain migration")
	}
}
