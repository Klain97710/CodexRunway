//go:build linux

package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type linuxLegacyLogin struct{ directory string }

func legacyMigratingLoginBackend(_ string, current LoginBackend) LoginBackend {
	return &migratingLoginBackend{current: current, legacy: linuxLegacyLogin{}, currentDisabled: currentDesktopDisabled}
}

func currentDesktopDisabled() (bool, error) {
	config, err := os.UserConfigDir()
	if err != nil {
		return false, err
	}
	data, err := os.ReadFile(filepath.Join(config, "autostart", "com.quosparr.app.desktop"))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	enabled, err := previousDesktopEntry(data)
	return !enabled, err
}
func (l linuxLegacyLogin) visit(remove bool) (legacyLoginState, error) {
	directory := l.directory
	if directory == "" {
		config, err := os.UserConfigDir()
		if err != nil {
			return legacyLoginState{}, err
		}
		directory = filepath.Join(config, "autostart")
	}
	state := legacyLoginState{}
	for _, id := range previousLoginIDs {
		path := filepath.Join(directory, id+".desktop")
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return state, err
		}
		if !info.Mode().IsRegular() {
			return state, fmt.Errorf("previous login item is not a regular file")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return state, err
		}
		enabled, err := previousDesktopEntry(data)
		if err != nil {
			return state, err
		}
		state.Exists = true
		state.Enabled = state.Enabled || enabled
		if remove {
			if err := os.Remove(path); err != nil {
				return state, err
			}
		}
	}
	return state, nil
}
func (l linuxLegacyLogin) State() (legacyLoginState, error) { return l.visit(false) }
func (l linuxLegacyLogin) Remove() error                    { _, err := l.visit(true); return err }
