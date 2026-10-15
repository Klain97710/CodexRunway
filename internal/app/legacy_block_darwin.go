//go:build darwin

package app

// previousApps returns the running apps of a previous identity.
func previousApps() []runningApp {
	return runningApplications(func(bundleID, _ string) bool { return previousLoginID(bundleID) })
}
