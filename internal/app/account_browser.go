package app

import (
	"context"
	"os/exec"
	"runtime"
)

func openAccountBrowser(ctx context.Context, rawURL string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.CommandContext(ctx, "/usr/bin/open", rawURL)
	case "windows":
		cmd = exec.CommandContext(ctx, "rundll32", "url.dll,FileProtocolHandler", rawURL)
	default:
		cmd = exec.CommandContext(ctx, "xdg-open", rawURL)
	}
	// Wait for the URL handler's exit status; none of its output is logged.
	return cmd.Run()
}
