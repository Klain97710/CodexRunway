package app

import (
	"context"
	"errors"
	"time"

	"github.com/licoy/quosparr/internal/codex"
	"github.com/licoy/quosparr/internal/netx"
	"github.com/licoy/quosparr/internal/runway"
	"github.com/licoy/quosparr/internal/usage"
)

// DevelopmentCredentials never opens the system Keychain.
func DevelopmentCredentials() *PlatformCredentials {
	return newPlatformCredentials(&netx.MemoryCredentialStore{})
}

type silentNotifications struct{}

func (silentNotifications) Supported() bool { return false }
func (silentNotifications) Show(Notification) error {
	return errors.New("development notifications disabled")
}
func DevelopmentNotifier() *Notifier { return &Notifier{backend: silentNotifications{}} }

// StartDevelopmentBackground deliberately constructs no production account,
// network, scanner, Widget bridge or reset provider. Empty fixture state is
// sufficient for settings/layout review; acceptance uses a separate VM.
func StartDevelopmentBackground(ctx context.Context, opts BackgroundOptions) func() {
	svc := runway.New(runway.Options{
		Preferences: opts.Settings.Prefs(),
		Sources: runway.Sources{
			LoadValidAuth: func(context.Context, bool, *codex.Auth) (codex.Auth, error) {
				return codex.Auth{}, &runway.AuthLoadError{Kind: runway.AuthFileInvalid}
			},
			ScanCost: func(context.Context, []usage.Query, time.Time, usage.RefreshPolicy, *usage.ProgressReporter) (map[string]usage.Summary, error) {
				return nil, nil
			},
		},
		OnSnapshot: func(s runway.Snapshot) {
			if opts.Host != nil {
				publishLive(opts.Host, s)
			}
		},
	})
	unsubscribe := opts.Settings.OnChange(func() { svc.SetPreferences(opts.Settings.Prefs()) })
	if opts.Host != nil {
		opts.Host.SetPanelActions(PanelActions{})
	}
	svc.Start(ctx)
	return func() { unsubscribe(); svc.Close() }
}
