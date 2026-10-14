package app

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/runway"
)

var errRestartCodex = errors.New("Codex restart failed")

type restartableCodex struct {
	bundleID, name, path string
	terminate            func() bool
	stopped              func() bool
	release              func()
}

func isCodexDesktop(bundleID, name string) bool {
	return strings.ToLower(bundleID) == "com.openai.codex"
}

// runCodexRestart is also used by tests with fake processes and a fake opener.
// A termination request must actually finish before the login is relaunched.
func runCodexRestart(apps []restartableCodex, fallback string, open func(string) error) error {
	path := fallback
	for _, app := range apps {
		if app.release != nil {
			defer app.release()
		}
		if !isCodexDesktop(app.bundleID, app.name) {
			continue
		}
		if app.path != "" {
			path = app.path
		}
		if !app.terminate() {
			return errRestartCodex
		}
		deadline := time.Now().Add(3 * time.Second)
		for !app.stopped() {
			if time.Now().After(deadline) {
				return errRestartCodex
			}
			time.Sleep(25 * time.Millisecond)
		}
	}
	if path == "" {
		return errRestartCodex
	}
	if err := open(path); err != nil {
		return errRestartCodex
	}
	return nil
}

func switchAndRestartCodex(switchAccount func() error, restart func() error) error {
	if err := switchAccount(); err != nil {
		return err
	}
	if restart != nil {
		if err := restart(); err != nil {
			return errRestartCodex
		}
	}
	return nil
}

func rtSwitchCodex(l l10n.Localizer, id string, restartAfter bool) string {
	message, _ := rtSwitchCodexResult(l, id, restartAfter)
	return message
}

func rtSwitchCodexResult(l l10n.Localizer, id string, restartAfter bool) (string, bool) {
	rt := currentRuntime()
	if rt == nil {
		return l.Text(l10n.KeyAccountsSwitchFailed), false
	}
	var restart func() error
	if restartAfter {
		restart = restartCodexDesktop
	}
	err := switchAndRestartCodex(func() error { return rt.svc.SwitchAccount(context.Background(), id) }, restart)
	if errors.Is(err, errRestartCodex) {
		return l.Textf(l10n.KeyAccountsRestartCodexFailed, l.Text(l10n.KeyStatusError)), false
	}
	if err != nil {
		if key := runway.CredentialModeErrorKey(err); key != "" {
			return l.Text(key), false
		}
		return l.Text(l10n.KeyAccountsSwitchFailed), false
	}
	if restartAfter {
		return l.Text(l10n.KeyAccountsRestartCodexSucceeded), true
	}
	return "", true
}
