package app

import (
	"errors"
	"slices"
	"testing"
)

type fakeLegacyLogin struct {
	count, reads, removals int
	readErr, removeErr     error
	keepOnRemove           bool
	disabled               bool
}

func (f *fakeLegacyLogin) State() (legacyLoginState, error) {
	f.reads++
	return legacyLoginState{Exists: f.count > 0, Enabled: f.count > 0 && !f.disabled}, f.readErr
}

func (f *fakeLegacyLogin) Remove() error {
	f.removals++
	if f.removeErr == nil && !f.keepOnRemove {
		f.count = 0
	}
	return f.removeErr
}

func TestLegacyLoginMigration(t *testing.T) {
	for _, tc := range []struct {
		name       string
		legacy     int
		current    LoginStatus
		wantSets   []bool
		wantRemove int
		wantStatus LoginStatus
	}{
		{"enabled in Swift", 1, LoginNotRegistered, []bool{true}, 1, LoginEnabled},
		{"off in Swift", 0, LoginNotRegistered, nil, 0, LoginNotRegistered},
		{"already migrated", 0, LoginEnabled, nil, 0, LoginEnabled},
		{"both registrations exist", 1, LoginEnabled, nil, 1, LoginEnabled},
		{"duplicate Swift records", 2, LoginNotRegistered, []bool{true}, 1, LoginEnabled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			current := &fakeLogin{status: tc.current}
			legacy := &fakeLegacyLogin{count: tc.legacy}
			backend := &migratingLoginBackend{current: current, legacy: legacy}
			for range 2 {
				got, err := backend.Status()
				if err != nil || got != tc.wantStatus {
					t.Fatalf("Status = %v, %v", got, err)
				}
			}
			if !slices.Equal(current.sets, tc.wantSets) || legacy.removals != tc.wantRemove || legacy.count != 0 {
				t.Fatalf("sets %v, removals %d, legacy count %d", current.sets, legacy.removals, legacy.count)
			}
		})
	}
}

func TestLegacyLoginMigrationFailuresRemainVisible(t *testing.T) {
	refused := errors.New("registration denied")
	unreadable := errors.New("list unavailable")
	removeFailed := errors.New("removal denied")
	for _, tc := range []struct {
		name    string
		current *fakeLogin
		legacy  *fakeLegacyLogin
		wantErr error
	}{
		{"unreadable list", &fakeLogin{}, &fakeLegacyLogin{count: 1, readErr: unreadable}, unreadable},
		{"registration fails", &fakeLogin{afterSet: status(LoginNotRegistered), setErr: refused}, &fakeLegacyLogin{count: 1}, ErrLoginRegister},
		{"removal fails after replacement", &fakeLogin{}, &fakeLegacyLogin{count: 1, removeErr: removeFailed}, removeFailed},
		{"removal leaves record", &fakeLogin{}, &fakeLegacyLogin{count: 1, keepOnRemove: true}, ErrLoginUnregister},
	} {
		t.Run(tc.name, func(t *testing.T) {
			backend := &migratingLoginBackend{current: tc.current, legacy: tc.legacy}
			_, err := backend.Status()
			if !errors.Is(err, tc.wantErr) || tc.legacy.count != 1 {
				t.Fatalf("error %v, legacy count %d", err, tc.legacy.count)
			}
			sets, reads, removals := len(tc.current.sets), tc.legacy.reads, tc.legacy.removals
			if err := backend.Set(false); !errors.Is(err, tc.wantErr) {
				t.Fatalf("Set hid migration failure: %v", err)
			}
			if len(tc.current.sets) != sets || tc.legacy.reads != reads || tc.legacy.removals != removals {
				t.Fatal("failed migration retried or changed a registration")
			}
		})
	}
}

func TestLegacyLoginMigrationUnavailableHostChangesNothing(t *testing.T) {
	current := &fakeLogin{}
	legacy := &fakeLegacyLogin{count: 1}
	item := LoginItem{Backend: &migratingLoginBackend{current: current, legacy: legacy}}
	if got, err := item.Status(); got != LoginUnavailable || err != nil {
		t.Fatalf("Status = %v, %v", got, err)
	}
	if _, err := item.Apply(true); err != nil || legacy.reads != 0 || len(current.sets) != 0 {
		t.Fatal("unsupported host touched login items")
	}
}

func TestLegacyLoginMigrationPreservesDisabledAndPendingApproval(t *testing.T) {
	for _, tc := range []struct {
		name        string
		disabled    bool
		status      LoginStatus
		wantRemoved int
	}{
		{"old explicitly disabled", true, LoginNotRegistered, 1},
		{"replacement requires approval", false, LoginRequiresApproval, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			old := &fakeLegacyLogin{count: 1, disabled: tc.disabled}
			current := &fakeLogin{status: tc.status}
			backend := &migratingLoginBackend{current: current, legacy: old}
			got, err := backend.Status()
			if err != nil || got != tc.status || len(current.sets) != 0 || old.removals != tc.wantRemoved {
				t.Fatalf("status %v err %v sets %v removals %v", got, err, current.sets, old.removals)
			}
		})
	}
}
func TestMigrationKeepsNativeLoginCallsAsync(t *testing.T) {
	if !loginBackendAsync(&migratingLoginBackend{current: nativeLoginBackend{}}) || loginBackendAsync(&migratingLoginBackend{current: mygoLoginBackend{}}) {
		t.Fatal("wrong UI dispatcher policy")
	}
}
func TestPreviousDesktopEntryValidatesOwnershipAndDisabledState(t *testing.T) {
	for _, tc := range []struct {
		name, command, extra string
		enabled, invalid     bool
	}{
		{"enabled", `"/opt/CodexRunway" --mygo-login`, "", true, false},
		{"space in path", `"/opt/My Apps/Quosparr-linux-arm64.AppImage" --mygo-login`, "", true, false},
		{"disabled", `"/opt/quosparr" --mygo-login`, "Hidden=true", false, false},
		{"gnome disabled", `"/opt/quosparr" --mygo-login`, "X-GNOME-Autostart-enabled=false", false, false},
		{"another app", `"/opt/other" --mygo-login`, "", false, true},
		{"shell", `sh -c /opt/quosparr`, "", false, true},
		{"duplicate exec", `"/opt/quosparr" --mygo-login`, `Exec="/opt/quosparr" --mygo-login`, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			enabled, err := previousDesktopEntry([]byte("[Desktop Entry]\nType=Application\nExec=" + tc.command + "\n" + tc.extra + "\n"))
			if (err != nil) != tc.invalid || enabled != tc.enabled {
				t.Fatalf("enabled=%v err=%v", enabled, err)
			}
		})
	}
}

func TestLegacyLoginMigrationHonorsNewSystemDisabledChoice(t *testing.T) {
	old := &fakeLegacyLogin{count: 1}
	current := &fakeLogin{status: LoginNotRegistered}
	backend := &migratingLoginBackend{current: current, legacy: old, currentDisabled: func() (bool, error) { return true, nil }}
	got, err := backend.Status()
	if err != nil || got != LoginNotRegistered || len(current.sets) != 0 || old.removals != 1 {
		t.Fatalf("new disabled choice lost: got=%v error=%v sets=%v removals=%v", got, err, current.sets, old.removals)
	}
}
