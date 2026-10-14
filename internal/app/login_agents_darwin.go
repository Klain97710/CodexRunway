//go:build darwin

package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/ebitengine/purego/objc"
)

type macLegacyLogin struct {
	session sessionLoginItems
	home    string // fixture override; production uses the current user's home
}

func (m macLegacyLogin) State() (legacyLoginState, error) {
	state, err := m.session.State()
	if err != nil {
		return state, err
	}
	agents, err := m.agents()
	if err != nil {
		return state, err
	}
	for _, agent := range agents {
		state.Exists = true
		state.Enabled = state.Enabled || agent.enabled
	}
	return state, nil
}

func (m macLegacyLogin) Remove() error {
	agents, err := m.agents()
	if err != nil {
		return err
	}
	if err := m.session.Remove(); err != nil {
		return err
	}
	// These are mygo's macOS 12 RunAtLoad files; it never bootstraps them.
	// The running old app is excluded by the legacy data lock during startup.
	for _, agent := range agents {
		if err := os.Remove(agent.path); err != nil {
			return err
		}
	}
	return nil
}

type previousLaunchAgent struct {
	path    string
	enabled bool
}

func (m macLegacyLogin) agents() ([]previousLaunchAgent, error) {
	home := m.home
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return nil, err
		}
	}
	var agents []previousLaunchAgent
	for _, id := range previousLoginIDs {
		path := filepath.Join(home, "Library", "LaunchAgents", id+".plist")
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("legacy login item is not a regular file")
		}
		values, err := readLoginPlist(path)
		if err != nil {
			return nil, err
		}
		args, ok := values["ProgramArguments"].([]any)
		if !ok || len(args) != 5 || values["Label"] != id || args[0] != "/usr/bin/open" || args[1] != "-a" || args[3] != "--args" || args[4] != "--mygo-login" {
			return nil, fmt.Errorf("previous login item has an unrecognised command; retained")
		}
		app, ok := args[2].(string)
		if !ok || !previousLoginBundle(app) {
			return nil, fmt.Errorf("previous login item target cannot be identified; retained")
		}
		enabled := values["RunAtLoad"] == true && values["Disabled"] != true
		if m.home == "" && enabled {
			enabled = !legacyAgentNeedsApproval(path)
		}
		agents = append(agents, previousLaunchAgent{path, enabled})
	}
	return agents, nil
}

// A system-disabled background item must not be re-enabled just because its
// original RunAtLoad plist still says true. Read the public macOS 13+ status.
func legacyAgentNeedsApproval(path string) bool {
	cls := nativeLoginClass()
	if cls == 0 {
		return false
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pool := objc.ID(objc.GetClass("NSAutoreleasePool")).Send(objc.RegisterName("new"))
	defer pool.Send(objc.RegisterName("drain"))
	url := objc.ID(objc.GetClass("NSURL")).Send(objc.RegisterName("fileURLWithPath:"), nsStr(path))
	return objc.Send[int](objc.ID(cls), objc.RegisterName("statusForLegacyURL:"), url) == smRequiresApproval
}
