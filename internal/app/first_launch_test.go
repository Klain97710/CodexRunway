package app

import (
	"bytes"
	"slices"
	"testing"

	"github.com/licoy/quosparr/internal/store"
)

func TestFirstLaunchShowsSettingsOnlyUntilPreferencesExist(t *testing.T) {
	dir := t.TempDir()
	for run := 0; run < 3; run++ {
		settings := NewSettings(store.New(store.FileDomain{Dir: dir}, nil), System{}, NewMemoryCredentials())
		if err := settings.Load(); err != nil {
			t.Fatal(err)
		}
		first, err := settings.InitializeFirstLaunch()
		if err != nil || first != (run == 0) {
			t.Fatalf("run %d: first=%t, err=%v", run, first, err)
		}
		var calls []string
		actions := launchRecorder(&calls)
		actions.FirstLaunch = first
		actions.InitialReady(CommandRun)
		want := []string{"ready"}
		if run == 0 {
			want = append(want, "settings")
		}
		if !slices.Equal(calls, want) {
			t.Fatalf("run %d: called %v, want %v", run, calls, want)
		}
	}
}

func TestFirstLaunchPreservesExistingAndMigratedPreferences(t *testing.T) {
	for _, migrated := range []bool{false, true} {
		current, legacy := store.NewMemoryDomain(), store.NewMemoryDomain()
		original := []byte(`{"language":"japanese","launchAtLoginEnabled":false,"futureKey":"kept"}`)
		if migrated {
			legacy.Put(store.KeyPreferences, original)
		} else {
			current.Put(store.KeyPreferences, original)
		}
		settings := NewSettings(store.New(current, legacy), System{}, NewMemoryCredentials())
		if err := settings.Load(); err != nil {
			t.Fatal(err)
		}
		writes := current.Writes()
		first, err := settings.InitializeFirstLaunch()
		if first || err != nil || current.Writes() != writes {
			t.Fatalf("migrated=%t: first=%t, err=%v, writes=%d", migrated, first, err, current.Writes())
		}
		data, found, err := current.Read(store.KeyPreferences)
		if err != nil || !found || !bytes.Equal(data, original) || legacy.Writes() != 0 {
			t.Fatal("initial launch changed existing preferences")
		}
	}
}

func TestFirstLaunchDoesNotHidePreferenceFailures(t *testing.T) {
	e := newEnv(t, nil)
	if first, err := e.s.InitializeFirstLaunch(); first || err == nil {
		t.Fatal("initial launch accepted preferences before Load")
	}
	if err := e.s.Load(); err != nil {
		t.Fatal(err)
	}
	e.mem.fail.Store(true)
	if first, err := e.s.InitializeFirstLaunch(); first || err == nil || e.mem.Writes() != 0 {
		t.Fatal("failed first-launch write was treated as success")
	}
	e.mem.fail.Store(false)
	if first, err := e.s.InitializeFirstLaunch(); !first || err != nil {
		t.Fatalf("failed initialization consumed first launch: first=%t, err=%v", first, err)
	}
	if first, err := e.s.InitializeFirstLaunch(); first || err != nil || e.mem.Writes() != 1 {
		t.Fatal("successful initialization was repeated")
	}
	broken := newEnv(t, func(d *failDomain) { d.Put(store.KeyPreferences, []byte("broken")) })
	if err := broken.s.Load(); err == nil {
		t.Fatal("expected corrupt preferences")
	}
	if first, err := broken.s.InitializeFirstLaunch(); first || err == nil || broken.mem.Writes() != 0 {
		t.Fatal("initial launch overwrote corrupt preferences")
	}
}
