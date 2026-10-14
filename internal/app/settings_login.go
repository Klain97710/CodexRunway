package app

import (
	"fmt"

	"github.com/licoy/quosparr/internal/store"
)

type loginResult struct {
	status      LoginStatus
	target      *bool // nil for a status refresh
	err         error
	previousErr error // retain a failed change while observing a queued refresh
}

// OnLoginChange observes transient login state without running preference
// subscribers, which may rebuild disk-backed background services.
func (s *Settings) OnLoginChange(fn func()) func() {
	index := len(s.onLoginChange)
	s.onLoginChange = append(s.onLoginChange, fn)
	return func() { s.onLoginChange[index] = nil }
}

func (s *Settings) loginChanged() {
	for _, fn := range s.onLoginChange {
		if fn != nil {
			fn()
		}
	}
}

func (s *Settings) LoginPending() bool { return s.loginPending }

func (s *Settings) readLogin() (LoginStatus, error) {
	if s.sys.LoginStatus != nil {
		return s.sys.LoginStatus()
	}
	if s.sys.OpenAtLogin == nil {
		return LoginUnavailable, fmt.Errorf("login status is unavailable")
	}
	on, err := s.sys.OpenAtLogin()
	if err != nil {
		return LoginUnavailable, err
	}
	if on {
		return LoginEnabled, nil
	}
	return LoginNotRegistered, nil
}

func (s *Settings) applyLogin(on bool) (LoginStatus, error) {
	if s.sys.ApplyLogin != nil {
		return s.sys.ApplyLogin(on)
	}
	if err := s.sys.SetOpenAtLogin(on); err != nil {
		return LoginUnavailable, err
	}
	return s.readLogin()
}

// ChangeLaunchAtLogin accepts a UI request. Pending is displayed immediately,
// but preferences are only changed after the system confirms the operation.
// Backends that require MyGo's UI dispatcher retain their synchronous path.
func (s *Settings) ChangeLaunchAtLogin(on bool) bool {
	if s.ReadOnly() || !s.LoginAvailable() || s.loginPending {
		return false
	}
	if !s.sys.LoginAsync {
		ok := s.SetLaunchAtLogin(on)
		s.loginChanged()
		return ok
	}
	s.startLogin(&on, nil)
	return true
}

// RefreshLogin is called when settings open or regain focus, so changes made
// in System Settings are observed without putting OS I/O inside rendering.
func (s *Settings) RefreshLogin() {
	if !s.LoginAvailable() {
		return
	}
	if s.loginPending {
		s.loginRefreshAgain = true
		return
	}
	if !s.sys.LoginAsync {
		st, err := s.readLogin()
		s.finishLogin(loginResult{status: st, err: err})
		return
	}
	s.startLogin(nil, nil)
}

func (s *Settings) startLogin(target *bool, previousErr error) {
	s.loginPending, s.loginTarget = true, target
	s.loginErr = previousErr
	s.loginIdle.Add(1)
	post := mainDispatcher()
	s.loginChanged()
	go func() {
		var result loginResult
		result.target = target
		result.previousErr = previousErr
		if target == nil {
			result.status, result.err = s.readLogin()
		} else {
			result.status, result.err = s.applyLogin(*target)
		}
		s.mu.Lock()
		s.loginResult = &result
		s.mu.Unlock()
		// Quit may be waiting inside Flush on the UI thread. Store the result
		// and release the worker before a dispatcher can wait for that thread.
		s.loginIdle.Done()
		post(s.deliverLoginResult)
	}()
}

func (s *Settings) deliverLoginResult() {
	s.mu.Lock()
	result := s.loginResult
	s.loginResult = nil
	s.mu.Unlock()
	if result != nil {
		s.finishLogin(*result)
		if s.loginRefreshAgain {
			s.loginRefreshAgain = false
			var previousErr error
			if result.target != nil {
				previousErr = result.err
			}
			s.startLogin(nil, previousErr)
		}
	}
}

func (s *Settings) finishLogin(result loginResult) {
	s.loginPending, s.loginTarget = false, nil
	s.loginErr = result.err
	if result.err == nil {
		s.loginErr = result.previousErr
	}
	s.loginStatus = result.status
	s.loginKnown = result.status != LoginUnavailable
	if result.err == nil && !s.ReadOnly() {
		if result.target != nil {
			s.Update(func(p *store.Preferences) {
				p.LaunchAtLoginEnabled = result.status.Registered()
				p.LaunchAtLoginInitialized = true
			})
		} else if s.prefs.LaunchAtLoginInitialized && s.prefs.LaunchAtLoginEnabled != result.status.Registered() {
			s.Update(func(p *store.Preferences) { p.LaunchAtLoginEnabled = result.status.Registered() })
		}
	}
	s.loginChanged()
}
