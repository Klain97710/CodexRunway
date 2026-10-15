package app

import (
	"time"

	"github.com/egoist/mygo"
	"github.com/licoy/quosparr/internal/l10n"
)

// RunLegacyBlocked runs when an app of the previous name (CodexRunway) holds
// the lock on ~/.codex-runway: both refresh and switch the same official
// credentials, so Quosparr must not start next to it. Instead of exiting
// silently it explains this in a minimal app; where the previous app can be
// asked to quit (macOS) it offers that, then relaunches Quosparr, which
// waits for the released lock (RelaunchEnv).
func RunLegacyBlocked() error {
	loc := l10n.ForPreference("", l10n.SystemPreferredLanguages())
	mg := mygo.App
	mg.SetActivationPolicy(mygo.ActivationPolicyAccessory)
	mg.OnWindowAllClosed(func() {})
	mg.WhenReady(func() {
		go func() {
			if resolveLegacyBlock(loc, previousApps(), askMessage, 5*time.Second) {
				mg.Relaunch()
				return
			}
			mg.Quit()
		}()
	})
	return mg.Run()
}

// resolveLegacyBlock asks the user about the running previous apps and
// reports whether they all quit, so Quosparr can start.
func resolveLegacyBlock(loc l10n.Localizer, apps []runningApp, ask func(mygo.MessageOptions) int, limit time.Duration) bool {
	for _, app := range apps {
		if app.release != nil {
			defer app.release()
		}
	}
	ok := []string{loc.Text(l10n.KeyOk)}
	if len(apps) == 0 {
		ask(mygo.MessageOptions{Type: mygo.MessageWarning, Message: tx(loc, xLegacyRunningTitle), Detail: tx(loc, xLegacyRunningManual), Buttons: ok})
		return false
	}
	choice := ask(mygo.MessageOptions{
		Type: mygo.MessageWarning, Message: tx(loc, xLegacyRunningTitle), Detail: tx(loc, xLegacyRunningDetail),
		Buttons: []string{tx(loc, xLegacyQuitAndContinue), loc.Text(l10n.KeyCancel)}, CancelButton: 1,
	})
	if choice != 0 {
		return false
	}
	for _, app := range apps {
		if !stopApp(app, limit) {
			ask(mygo.MessageOptions{Type: mygo.MessageWarning, Message: tx(loc, xLegacyQuitFailed), Detail: tx(loc, xLegacyRunningManual), Buttons: ok})
			return false
		}
	}
	return true
}

// askMessage shows a modal message and returns the clicked button; a failed
// dialog counts as the cancel button. Call it off the main thread.
func askMessage(opts mygo.MessageOptions) int {
	res, err := mygo.Dialog.Message(opts)
	if err != nil {
		return opts.CancelButton
	}
	return res.Button
}
