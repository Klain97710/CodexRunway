//go:build !darwin

package app

// nativeLoginStatus: only macOS has a state beyond on and off (a login item
// waiting for the user's approval).
func nativeLoginStatus() (LoginStatus, bool) { return 0, false }

func nativeLoginAvailable() bool { return false }

func nativeLoginSet(bool) (error, bool) { return nil, false }
