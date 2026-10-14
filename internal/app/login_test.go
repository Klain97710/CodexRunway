package app

import (
	"errors"
	"slices"
	"testing"

	"github.com/licoy/quosparr/internal/store"
)

// fakeLogin is a system login item.
type fakeLogin struct {
	status    LoginStatus
	statusErr error
	setErr    error
	sets      []bool
	// afterSet is the status Set leaves behind; nil follows the request.
	afterSet *LoginStatus
}

func (f *fakeLogin) Status() (LoginStatus, error) { return f.status, f.statusErr }

func (f *fakeLogin) Set(on bool) error {
	f.sets = append(f.sets, on)
	switch {
	case f.afterSet != nil:
		f.status = *f.afterSet
	case on:
		f.status = LoginEnabled
	default:
		f.status = LoginNotRegistered
	}
	return f.setErr
}

func status(s LoginStatus) *LoginStatus { return &s }

func TestLoginItemUnavailableNeverTouchesTheSystem(t *testing.T) {
	f := &fakeLogin{status: LoginNotRegistered}
	item := LoginItem{Backend: f, CanMutateSys: false}
	if st, err := item.Status(); st != LoginUnavailable || err != nil {
		t.Errorf("status %v %v", st, err)
	}
	if st, err := item.Apply(true); st != LoginUnavailable || err != nil || len(f.sets) != 0 {
		t.Errorf("apply %v %v sets %v", st, err, f.sets)
	}
}

func TestLoginItemApply(t *testing.T) {
	mutating := func(f *fakeLogin) LoginItem { return LoginItem{Backend: f, CanMutateSys: true} }

	t.Run("registers", func(t *testing.T) {
		f := &fakeLogin{status: LoginNotRegistered}
		st, err := mutating(f).Apply(true)
		if st != LoginEnabled || err != nil || !slices.Equal(f.sets, []bool{true}) {
			t.Errorf("%v %v %v", st, err, f.sets)
		}
	})
	t.Run("already as wanted asks nothing of the system", func(t *testing.T) {
		// Registering again would clear the user's Task Manager mark on Windows.
		for _, c := range []struct {
			cur LoginStatus
			on  bool
		}{{LoginEnabled, true}, {LoginRequiresApproval, true}, {LoginNotRegistered, false}} {
			f := &fakeLogin{status: c.cur}
			if _, err := mutating(f).Apply(c.on); err != nil || len(f.sets) != 0 {
				t.Errorf("%v on=%v: err %v sets %v", c.cur, c.on, err, f.sets)
			}
		}
	})
	t.Run("waiting for approval counts as registered", func(t *testing.T) {
		f := &fakeLogin{status: LoginNotRegistered, afterSet: status(LoginRequiresApproval), setErr: errors.New("allow it in System Settings")}
		st, err := mutating(f).Apply(true)
		if st != LoginRequiresApproval || err != nil {
			t.Errorf("%v %v", st, err)
		}
	})
	t.Run("a refused registration is an error with its reason", func(t *testing.T) {
		f := &fakeLogin{status: LoginNotRegistered, afterSet: status(LoginNotRegistered), setErr: errors.New("no bundle")}
		_, err := mutating(f).Apply(true)
		if !errors.Is(err, ErrLoginRegister) || err.Error() == ErrLoginRegister.Error() {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("removes", func(t *testing.T) {
		f := &fakeLogin{status: LoginEnabled}
		if st, err := mutating(f).Apply(false); st != LoginNotRegistered || err != nil {
			t.Errorf("%v %v", st, err)
		}
	})
	t.Run("a login item that stays is an error", func(t *testing.T) {
		f := &fakeLogin{status: LoginEnabled, afterSet: status(LoginEnabled)}
		if _, err := mutating(f).Apply(false); !errors.Is(err, ErrLoginUnregister) {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("an unreadable status is an error", func(t *testing.T) {
		f := &fakeLogin{statusErr: errors.New("unreadable")}
		if _, err := mutating(f).Apply(true); err == nil || len(f.sets) != 0 {
			t.Errorf("err %v sets %v", err, f.sets)
		}
	})
}

func TestLoginHostSupported(t *testing.T) {
	for _, c := range []struct {
		dev       bool
		goos, exe string
		want      bool
	}{
		{true, "linux", "/home/u/quosparr", false}, // go run / mygo dev
		{false, "linux", "/home/u/.local/quosparr", true},
		{false, "windows", `C:\Users\u\AppData\Local\Programs\Quosparr\Quosparr.exe`, true},
		{false, "darwin", "/Applications/Quosparr.app/Contents/MacOS/Quosparr", true},
		{false, "darwin", "/tmp/quosparr", false}, // not a bundle
		{true, "darwin", "/Applications/Quosparr.app/Contents/MacOS/Quosparr", false},
	} {
		if got := loginHostSupported(c.dev, c.goos, c.exe); got != c.want {
			t.Errorf("%+v: got %v", c, got)
		}
	}
}

// loginEnv is a settings environment whose login item is f.
func loginEnv(t *testing.T, f *fakeLogin, canMutate bool, p store.Preferences) *env {
	t.Helper()
	e := newEnv(t, func(d *failDomain) {
		data, err := store.EncodePreferences(p)
		if err != nil {
			t.Fatal(err)
		}
		d.Put(store.KeyPreferences, data)
	})
	e.s = NewSettings(e.st, LoginSystem(LoginItem{Backend: f, CanMutateSys: canMutate}, e.sys.system()), e.creds)
	if err := e.s.Load(); err != nil {
		t.Fatal(err)
	}
	return e
}

// TestLoginReconcileTable is the behaviour table: the saved choice
// (launchAtLoginEnabled/Initialized) against the real state of the login item.
func TestLoginReconcileTable(t *testing.T) {
	for _, c := range []struct {
		name        string
		enabled     bool
		initialized bool
		system      LoginStatus
		canMutate   bool
		wantSets    []bool
		wantEnabled bool
		wantInit    bool
	}{
		{"first run registers the default (on)", true, false, LoginNotRegistered, true, []bool{true}, true, true},
		{"first run with the item already there", true, false, LoginEnabled, true, nil, true, true},
		{"user turned it off in the system settings: not enabled again", true, true, LoginNotRegistered, true, nil, false, true},
		{"user turned it on in the system settings: choice follows", false, true, LoginEnabled, true, nil, true, true},
		{"waiting for approval keeps the choice on", true, true, LoginRequiresApproval, true, nil, true, true},
		{"choice off and item off stays so", false, true, LoginNotRegistered, true, nil, false, true},
		{"development run changes nothing", true, false, LoginNotRegistered, false, nil, true, false},
		{"development run, initialized, changes nothing", true, true, LoginNotRegistered, false, nil, true, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := &fakeLogin{status: c.system}
			p := store.DefaultPreferences()
			p.LaunchAtLoginEnabled, p.LaunchAtLoginInitialized = c.enabled, c.initialized
			e := loginEnv(t, f, c.canMutate, p)
			if err := e.s.ReconcileLogin(); err != nil {
				t.Fatalf("ReconcileLogin: %v", err)
			}
			e.settle()
			got := e.stored()
			if !slices.Equal(f.sets, c.wantSets) {
				t.Errorf("system was asked %v, want %v", f.sets, c.wantSets)
			}
			if got.LaunchAtLoginEnabled != c.wantEnabled || got.LaunchAtLoginInitialized != c.wantInit {
				t.Errorf("stored enabled=%v initialized=%v, want %v/%v", got.LaunchAtLoginEnabled, got.LaunchAtLoginInitialized, c.wantEnabled, c.wantInit)
			}
		})
	}
}

func TestLoginReconcileFailureIsShownAndNotRecorded(t *testing.T) {
	f := &fakeLogin{status: LoginNotRegistered, afterSet: status(LoginNotRegistered), setErr: errors.New("denied")}
	p := store.DefaultPreferences()
	p.LaunchAtLoginInitialized = false
	e := loginEnv(t, f, true, p)
	if err := e.s.ReconcileLogin(); err == nil || e.s.LoginError() == nil {
		t.Fatal("the failure was not reported")
	}
	e.settle()
	if e.stored().LaunchAtLoginInitialized {
		t.Error("a failed registration was recorded as the user's choice")
	}
}

func TestLoginStatusForTheSettingsPage(t *testing.T) {
	f := &fakeLogin{status: LoginRequiresApproval}
	e := loginEnv(t, f, true, store.DefaultPreferences())
	if err := e.s.ReconcileLogin(); err != nil {
		t.Fatal(err)
	}
	e.settle()
	if e.s.LoginStatus() != LoginRequiresApproval || !e.s.LoginSwitch() {
		t.Errorf("status %v switch %v", e.s.LoginStatus(), e.s.LoginSwitch())
	}
	e = loginEnv(t, f, false, store.DefaultPreferences())
	if e.s.LoginAvailable() || e.s.LoginStatus() != LoginUnavailable {
		t.Error("an unsupported host must report unavailable")
	}
}
