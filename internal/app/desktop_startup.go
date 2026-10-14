package app

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/egoist/mygo"
	"github.com/licoy/quosparr/internal/l10n"
)

// RunDesktop also handles GTK load errors, which mygo returns before Ready
// or any dialog can exist. Main prints the localized error and exits with 1.
// Help and self-check do not call this method or initialize desktop APIs.
func (a *App) RunDesktop() error {
	return a.runDesktop(mygo.App.Run)
}

func (a *App) runDesktop(run func() error) error {
	if err := run(); err != nil {
		if runtime.GOOS == "linux" {
			return linuxDesktopStartupError(a.L(), err, os.ReadFile)
		}
		return err
	}
	return a.startupErr
}

func linuxDesktopStartupError(loc l10n.Localizer, cause error, readFile func(string) ([]byte, error)) error {
	if !missingDesktopLibrary(cause) {
		return cause
	}
	return fmt.Errorf("%s\n\n%w", linuxDependencyHint(loc, readFile), cause)
}

func trayStartupError(loc l10n.Localizer, cause error) error {
	message := tx(loc, xTrayUnavailable)
	if runtime.GOOS == "linux" && missingDesktopLibrary(cause) {
		message += "\n\n" + linuxDependencyHint(loc, os.ReadFile)
	}
	return fmt.Errorf("%s\n\n%w", message, cause)
}

func missingDesktopLibrary(err error) bool {
	for _, name := range []string{"libgtk-3", "libgdk-3", "libglib-2", "libgobject-2", "libgio-2", "libcairo", "libgdk_pixbuf-2", "libayatana-appindicator3", "libappindicator3"} {
		if strings.Contains(err.Error(), name) {
			return true
		}
	}
	return false
}

func linuxDependencyHint(loc l10n.Localizer, readFile func(string) ([]byte, error)) string {
	release, err := readFile("/etc/os-release")
	message := tx(loc, xLinuxDesktopDependencies, linuxInstallCommands(string(release)))
	if err != nil {
		message += "\n\n" + tx(loc, xLinuxReleaseUnreadable, err.Error())
	}
	return message
}

// Ayatana depends on the distribution's GTK 3 runtime, including the t64
// package on newer Debian/Ubuntu releases. Let their package manager select
// that dependency, rather than guessing its name for derivative releases.
func linuxInstallCommands(release string) string {
	fields := map[string]string{}
	for line := range strings.SplitSeq(release, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if ok {
			fields[key] = strings.Trim(value, "\"'")
		}
	}
	for _, id := range append([]string{fields["ID"]}, strings.Fields(fields["ID_LIKE"])...) {
		switch id {
		case "ubuntu", "debian":
			return "sudo apt install libayatana-appindicator3-1"
		case "fedora":
			return "sudo dnf install gtk3 libayatana-appindicator-gtk3"
		case "arch":
			return "sudo pacman -S gtk3 libayatana-appindicator"
		}
	}
	return "Debian / Ubuntu: sudo apt install libayatana-appindicator3-1\nFedora: sudo dnf install gtk3 libayatana-appindicator-gtk3\nArch Linux: sudo pacman -S gtk3 libayatana-appindicator"
}
