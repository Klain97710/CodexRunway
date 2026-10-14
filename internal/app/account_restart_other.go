//go:build !darwin

package app

const codexRestartSupported = false

func restartCodexDesktop() error { return errRestartCodex }
