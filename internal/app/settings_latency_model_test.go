package app

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/store"
)

func TestSettingsLoginPendingPreservesConfirmedPreferences(t *testing.T) {
	for _, result := range []struct {
		name string
		st   LoginStatus
		err  error
	}{
		{"enabled", LoginEnabled, nil},
		{"requires approval", LoginRequiresApproval, nil},
		{"denied", LoginNotRegistered, errors.New("registration denied")},
	} {
		t.Run(result.name, func(t *testing.T) {
			e := newEnv(t, func(d *failDomain) {
				p := store.DefaultPreferences()
				p.LaunchAtLoginEnabled, p.LaunchAtLoginInitialized = false, true
				data, err := store.EncodePreferences(p)
				if err != nil {
					t.Fatal(err)
				}
				d.Put(store.KeyPreferences, data)
			})
			release, started := make(chan struct{}), make(chan struct{})
			var calls atomic.Int32
			sys := e.sys.system()
			sys.LoginAsync = true
			sys.ApplyLogin = func(on bool) (LoginStatus, error) {
				calls.Add(1)
				close(started)
				<-release
				return result.st, result.err
			}
			e.s = NewSettings(e.st, sys, e.creds)
			if err := e.s.Load(); err != nil {
				t.Fatal(err)
			}
			var prefsNotifications, loginNotifications int
			e.s.OnChange(func() { prefsNotifications++ })
			e.s.OnLoginChange(func() { loginNotifications++ })
			if !e.s.ChangeLaunchAtLogin(true) {
				t.Fatal("request refused")
			}
			<-started
			if !e.s.LoginPending() || !e.s.LoginSwitch() || e.s.Prefs().LaunchAtLoginEnabled || e.stored().LaunchAtLoginEnabled {
				t.Error("pending was either invisible or recorded as success")
			}
			if e.s.ChangeLaunchAtLogin(false) {
				t.Error("duplicate request accepted")
			}
			e.s.RefreshLogin()
			if calls.Load() != 1 || prefsNotifications != 0 || loginNotifications != 1 {
				t.Error("pending triggered backend reentry or preference subscribers")
			}
			close(release)
			e.settle()
			if e.s.LoginPending() || e.s.LoginStatus() != result.st || e.s.LoginSwitch() != result.st.Registered() {
				t.Error("completed UI does not show the observed system state")
			}
			if e.stored().LaunchAtLoginEnabled != (result.err == nil) || !errors.Is(e.s.LoginError(), result.err) {
				t.Error("backend success/failure was not preserved")
			}
		})
	}
}

func TestSettingsLoginRefreshObservesExternalChangeAndReadFailure(t *testing.T) {
	e := newEnv(t, nil)
	sys := e.sys.system()
	var calls atomic.Int32
	var status LoginStatus = LoginRequiresApproval
	var readErr error
	sys.LoginAsync = true
	sys.LoginStatus = func() (LoginStatus, error) { calls.Add(1); return status, readErr }
	e.s = NewSettings(e.st, sys, e.creds)
	if err := e.s.Load(); err != nil {
		t.Fatal(err)
	}
	var notified int
	e.s.OnChange(func() { notified++ })
	e.s.RefreshLogin()
	e.settle()
	if e.s.LoginStatus() != LoginRequiresApproval || !e.s.LoginSwitch() || notified != 0 {
		t.Fatal("approval refresh changed preferences or lost observed status")
	}
	// A user choice is already recorded, so an external removal must follow
	// the system without re-registering it.
	e.s.Update(func(p *store.Preferences) { p.LaunchAtLoginInitialized = true })
	e.settle()
	status = LoginNotRegistered
	e.s.RefreshLogin()
	e.settle()
	if e.stored().LaunchAtLoginEnabled || e.s.LoginSwitch() || len(e.sys.loginSets) != 0 {
		t.Fatal("external removal was undone or ignored")
	}
	readErr = errors.New("status service unavailable")
	status = LoginUnavailable
	e.s.RefreshLogin()
	e.settle()
	if !errors.Is(e.s.LoginError(), readErr) || e.s.LoginStatus() != LoginUnavailable || e.stored().LaunchAtLoginEnabled {
		t.Fatal("status failure was hidden or persisted")
	}
	if calls.Load() != 3 {
		t.Fatalf("queries=%d, want 3", calls.Load())
	}
}

func TestSettingsLoginReadOnlyDoesNotMutateSystem(t *testing.T) {
	e := newEnv(t, func(d *failDomain) { d.Put(store.KeyPreferences, []byte("{broken")) })
	_ = e.s.Load()
	if e.s.ChangeLaunchAtLogin(false) || len(e.sys.loginSets) != 0 {
		t.Fatal("read-only request changed system")
	}
}

func TestSettingsLoginRefreshDuringPendingObservesLatestSystemState(t *testing.T) {
	e := newEnv(t, nil)
	sys := e.sys.system()
	sys.LoginAsync = true
	var observed atomic.Int32
	observed.Store(int32(LoginEnabled))
	var calls atomic.Int32
	read, release := make(chan struct{}), make(chan struct{})
	sys.LoginStatus = func() (LoginStatus, error) {
		snapshot := LoginStatus(observed.Load())
		if calls.Add(1) == 1 {
			close(read)
			<-release
		}
		return snapshot, nil
	}
	e.s = NewSettings(e.st, sys, e.creds)
	if err := e.s.Load(); err != nil {
		t.Fatal(err)
	}
	e.s.Update(func(p *store.Preferences) { p.LaunchAtLoginInitialized = true })
	e.settle()
	e.s.RefreshLogin()
	<-read // the first worker has already captured the old snapshot
	observed.Store(int32(LoginNotRegistered))
	e.s.RefreshLogin() // returning from System Settings while still pending
	e.s.RefreshLogin() // multiple focus events coalesce into one extra read
	close(release)
	deadline := time.Now().Add(time.Second)
	for calls.Load() != 2 && time.Now().Before(deadline) {
		e.q.drain()
		time.Sleep(time.Millisecond)
	}
	e.settle()
	if calls.Load() != 2 || e.s.LoginStatus() != LoginNotRegistered || e.s.LoginSwitch() || e.stored().LaunchAtLoginEnabled {
		t.Fatalf("pending focus refresh lost the latest state: calls=%d status=%v switch=%v", calls.Load(), e.s.LoginStatus(), e.s.LoginSwitch())
	}
}

func TestSettingsLoginSaveFailureKeepsObservedSystemStatus(t *testing.T) {
	e := newEnv(t, nil)
	sys := e.sys.system()
	sys.LoginAsync = true
	sys.ApplyLogin = func(bool) (LoginStatus, error) { return LoginNotRegistered, nil }
	e.s = NewSettings(e.st, sys, e.creds)
	if err := e.s.Load(); err != nil {
		t.Fatal(err)
	}
	e.mem.fail.Store(true)
	e.s.ChangeLaunchAtLogin(false)
	e.settle()
	if e.s.SaveError() == nil || !e.s.Prefs().LaunchAtLoginEnabled || e.s.LoginSwitch() || e.s.LoginStatus() != LoginNotRegistered {
		t.Fatal("save rollback replaced actual system status with stored preferences")
	}
}

func TestSettingsLoginFlushDoesNotWaitForSynchronousUIDispatcher(t *testing.T) {
	e := newEnv(t, nil)
	sys := e.sys.system()
	sys.LoginAsync = true
	sys.ApplyLogin = func(bool) (LoginStatus, error) { return LoginNotRegistered, nil }
	e.s = NewSettings(e.st, sys, e.creds)
	if err := e.s.Load(); err != nil {
		t.Fatal(err)
	}
	queued := make(chan func(), 4)
	release := make(chan struct{})
	dispatched := make(chan struct{}, 4)
	restore := setRunOnMain(func(fn func()) { queued <- fn; <-release; dispatched <- struct{}{} })
	defer restore()
	e.s.ChangeLaunchAtLogin(false)
	var callback func()
	select {
	case callback = <-queued:
	case <-time.After(time.Second):
		t.Fatal("backend did not reach dispatcher")
	}
	// The callback is blocked waiting for this thread. Flush must consume its
	// stored result itself, then complete the resulting disk save.
	watchdog := time.AfterFunc(time.Second, func() { close(release) })
	e.s.Flush()
	if !watchdog.Stop() {
		t.Fatal("Flush waited for UI delivery")
	}
	if e.s.LoginPending() || e.stored().LaunchAtLoginEnabled {
		t.Fatal("Flush lost confirmed login result")
	}
	callback()
	close(release)
	<-dispatched
}

func TestNativeLoginBackendUnavailableFailsWithoutMyGoDispatch(t *testing.T) {
	if nativeLoginAvailable() {
		t.Skip("test must not manage a real app bundle")
	}
	backend := nativeLoginBackend{}
	if _, err := backend.Status(); err == nil {
		t.Fatal("unavailable native status reported success")
	}
	if err := backend.Set(true); err == nil {
		t.Fatal("unavailable native registration reported success")
	}
}
