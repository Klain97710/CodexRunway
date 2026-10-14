//go:build windows

package app

import (
	"errors"
	"fmt"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"strings"
)

const previousRunKey = `Software\Microsoft\Windows\CurrentVersion\Run`
const previousApprovedKey = `Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run`

type windowsLegacyLogin struct{}

func legacyMigratingLoginBackend(_ string, current LoginBackend) LoginBackend {
	return &migratingLoginBackend{current: current, legacy: windowsLegacyLogin{}, currentDisabled: func() (bool, error) {
		enabled, err := previousStartupEnabled("com.quosparr.app")
		return !enabled, err
	}}
}
func (windowsLegacyLogin) visit(remove bool) (legacyLoginState, error) {
	state := legacyLoginState{}
	access := uint32(registry.QUERY_VALUE)
	if remove {
		access |= registry.SET_VALUE
	}
	key, err := registry.OpenKey(registry.CURRENT_USER, previousRunKey, access)
	if errors.Is(err, registry.ErrNotExist) {
		return state, nil
	}
	if err != nil {
		return state, err
	}
	defer key.Close()
	for _, id := range previousLoginIDs {
		command, _, err := key.GetStringValue(id)
		if errors.Is(err, registry.ErrNotExist) {
			continue
		}
		if err != nil {
			return state, err
		}
		args, err := windows.DecomposeCommandLine(command)
		if err != nil || len(args) != 2 || args[1] != "--mygo-login" || !previousExecutable(args[0]) || !strings.ContainsAny(args[0], `\/:`) {
			return state, fmt.Errorf("legacy startup command cannot be identified; retained")
		}
		enabled, err := previousStartupEnabled(id)
		if err != nil {
			return state, err
		}
		state.Exists = true
		state.Enabled = state.Enabled || enabled
		if remove {
			if err := key.DeleteValue(id); err != nil {
				return state, err
			}
		}
	}
	return state, nil
}
func previousStartupEnabled(id string) (bool, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, previousApprovedKey, registry.QUERY_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	defer key.Close()
	data, _, err := key.GetBinaryValue(id)
	if errors.Is(err, registry.ErrNotExist) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return len(data) == 0 || data[0]&1 == 0, nil
}
func (w windowsLegacyLogin) State() (legacyLoginState, error) { return w.visit(false) }
func (w windowsLegacyLogin) Remove() error                    { _, err := w.visit(true); return err }
