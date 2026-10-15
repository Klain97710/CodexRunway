//go:build !darwin

package app

// previousApps finds no apps: only macOS can ask them to quit, elsewhere
// the user quits the previous app by hand.
func previousApps() []runningApp { return nil }
