package grok

import (
	"os/exec"
	"runtime"
)

// OpenInDefaultBrowser opens rawURL with the system's URL handler, without
// attaching the child process to the app's standard streams.
func OpenInDefaultBrowser(rawURL string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("/usr/bin/open", rawURL)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL)
	default:
		cmd = exec.Command("xdg-open", rawURL)
	}
	if err := cmd.Start(); err != nil {
		return &OAuthError{Kind: OAuthBrowserOpenFailed, Message: shortError(err)}
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
