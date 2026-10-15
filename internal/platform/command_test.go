package platform

import (
	"os/exec"
	"runtime"
	"testing"
)

func TestHideConsoleWindow(t *testing.T) {
	cmd := exec.Command("echo", "test")
	HideConsoleWindow(cmd)
	if runtime.GOOS == "windows" {
		if cmd.SysProcAttr == nil {
			t.Fatal("expected non-nil SysProcAttr on Windows")
		}
		if !cmd.SysProcAttr.HideWindow {
			t.Error("expected HideWindow to be true")
		}
		if cmd.SysProcAttr.CreationFlags&0x08000000 == 0 {
			t.Error("expected CREATE_NO_WINDOW flag to be set")
		}
	}
}
