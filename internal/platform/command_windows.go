//go:build windows

package platform

import (
	"os/exec"
	"syscall"
)

// HideConsoleWindow prevents Windows from flashing a console window when a
// GUI application spawns a console binary or a batch script (.cmd / .bat).
func HideConsoleWindow(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags |= 0x08000000 // CREATE_NO_WINDOW
}
