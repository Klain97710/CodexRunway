package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/personal"
	"github.com/licoy/quosparr/internal/store"
)

func TestLinuxInstallCommands(t *testing.T) {
	for _, tc := range []struct{ release, want string }{
		{"ID=ubuntu\nVERSION_ID=24.04", "sudo apt install libayatana-appindicator3-1"},
		{"ID=debian\nVERSION_ID=13", "sudo apt install libayatana-appindicator3-1"},
		{"ID=linuxmint\nID_LIKE=\"ubuntu debian\"", "sudo apt install libayatana-appindicator3-1"},
		{"ID=fedora", "sudo dnf install gtk3 libayatana-appindicator-gtk3"},
		{"ID=manjaro\nID_LIKE=arch", "sudo pacman -S gtk3 libayatana-appindicator"},
		{"ID=unknown", "Debian / Ubuntu: sudo apt install libayatana-appindicator3-1\nFedora: sudo dnf install gtk3 libayatana-appindicator-gtk3\nArch Linux: sudo pacman -S gtk3 libayatana-appindicator"},
	} {
		if got := linuxInstallCommands(tc.release); got != tc.want {
			t.Errorf("release %q: %q", tc.release, got)
		}
	}
}

func TestLinuxDesktopStartupErrors(t *testing.T) {
	read := func(path string) ([]byte, error) {
		if path != "/etc/os-release" {
			t.Fatalf("unexpected file %s", path)
		}
		return []byte("ID=fedora"), nil
	}
	for _, library := range []string{"libgtk-3.so.0", "libgdk-3.so.0", "libayatana-appindicator3", "libappindicator3.so.1"} {
		cause := fmt.Errorf("mygo: %s not found", library)
		err := linuxDesktopStartupError(l10n.ForPreference("simplifiedChinese", nil), cause, read)
		if !errors.Is(err, cause) || !strings.Contains(err.Error(), "sudo dnf install gtk3 libayatana-appindicator-gtk3") || !strings.Contains(err.Error(), "需要 GTK 3") {
			t.Fatalf("missing localized install hint: %v", err)
		}
	}
	cause := errors.New("mygo: cannot open display")
	if got := linuxDesktopStartupError(l10n.ForPreference("english", nil), cause, func(string) ([]byte, error) {
		t.Fatal("a display error must not read distribution metadata")
		return nil, nil
	}); got != cause {
		t.Fatal("non-dependency error was changed")
	}
	err := linuxDesktopStartupError(l10n.ForPreference("english", nil), errors.New("libgtk-3.so.0 missing"), func(string) ([]byte, error) {
		return nil, errors.New("metadata unavailable")
	})
	if !strings.Contains(err.Error(), "metadata unavailable") || !strings.Contains(err.Error(), "Debian / Ubuntu:") {
		t.Fatal("unreadable release metadata was hidden")
	}
}

func TestTrayFailureStopsStartup(t *testing.T) {
	cause := errors.New("tray unavailable")
	started := false
	a := New(Options{Seams: Seams{StartBackground: func(context.Context) func() {
		started = true
		return nil
	}}})
	if err := a.readyWithTray(func() error { return cause }); err != cause {
		t.Fatalf("readyWithTray = %v", err)
	}
	if a.backgroundReady || a.cancel != nil || started {
		t.Fatal("tray failure started background work")
	}
	a.OpenSettings() // the --settings ready callback must not open a window
	if a.setWin != nil || a.runDesktop(func() error { return nil }) != cause {
		t.Fatal("tray failure opened UI or returned success")
	}
}

func TestDesktopFailureExitProcess(t *testing.T) {
	if os.Getenv("QUOSPARR_TEST_DESKTOP_FAILURE") == "1" {
		loc := l10n.ForPreference("english", nil)
		cause := linuxDesktopStartupError(loc, errors.New("mygo: libgtk-3.so.0 missing"), func(string) ([]byte, error) {
			return []byte("ID=arch"), nil
		})
		a := &App{settings: NewSettings(store.New(store.NewMemoryDomain(), nil), System{Features: personal.Features{Grok: true, Claude: true}}, UnavailableCredentials{})}
		if err := a.runDesktop(func() error { return cause }); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestDesktopFailureExitProcess$")
	cmd.Env = append(os.Environ(), "QUOSPARR_TEST_DESKTOP_FAILURE=1", "HOME="+t.TempDir(), DevDomainEnv+"=com.quosparr.app.desktop-failure-test")
	output, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 || ctx.Err() != nil {
		t.Fatalf("child did not exit cleanly with failure: %v, %s", err, output)
	}
	if !strings.Contains(string(output), "sudo pacman -S gtk3 libayatana-appindicator") {
		t.Fatalf("child did not print installation command: %s", output)
	}
}
