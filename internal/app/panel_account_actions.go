package app

import "context"

// AccountActions are the controls of the multi-account pages. Each one runs
// to completion and returns the message to show, "" for none; the panel
// calls them off the main thread. A nil func disables its control.
type AccountActions struct {
	// Codex accounts.
	RefreshAll func() string
	Refresh    func(id string) string
	Switch     func(id string, restartCodex bool) string
	Delete     func(id string) string

	// Grok accounts. GrokSwitch reports running Grok processes, which the
	// user must confirm before force is set.
	GrokRefreshAll     func() string
	GrokRefresh        func(id string) string
	GrokSwitch         func(id string, force bool) (running bool, message string)
	GrokDelete         func(id string) string
	GrokAlias          func(id, alias string) string
	GrokMove           func(id string, direction int) string
	GrokLogin          func(ctx context.Context) string
	GrokPaste          func(text string) (message string, ok bool)
	GrokImportOfficial func() string
}

// allRefreshing marks "every account" as the refresh in flight.
const allRefreshing = "*"

// accountsState is the view state of the multi-account pages.
type accountsState struct {
	// busy is true while an action runs; refreshing names the account (or
	// allRefreshing) whose quota it polls.
	busy       bool
	refreshing string
	notice     string

	// Pending confirmations, by account id.
	switchID  string
	restart   bool
	deleteID  string
	runningID string

	// Grok alias editor and paste import.
	aliasID, alias string
	pasteOpen      bool
	paste          string
	pasteErr       string

	login context.CancelFunc
}

// closePage drops the pending confirmations and editors of the page.
func (s *accountsState) closePage() {
	s.switchID, s.deleteID, s.runningID, s.aliasID = "", "", "", ""
	s.pasteOpen, s.paste, s.pasteErr, s.notice = false, "", "", ""
}

// loggingIn reports a Grok browser login that can still be cancelled.
func (s *accountsState) loggingIn() bool { return s.login != nil }

// accountsRun starts fn unless another action runs, and shows its message
// when it ends. refreshing is the id the action polls, "" for none.
func (a *App) accountsRun(refreshing string, fn func() string) {
	s := &a.panelState.accounts
	if s.busy {
		return
	}
	s.busy, s.refreshing, s.notice = true, refreshing, ""
	go func() {
		msg := fn()
		runOnMain(func() {
			s.busy, s.refreshing, s.notice = false, "", msg
			a.redrawPanel()
		})
	}()
}

// accountsLogin runs the Grok browser login with a cancel handle.
func (a *App) accountsLogin(acts AccountActions) {
	s := &a.panelState.accounts
	if s.busy || acts.GrokLogin == nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.login = cancel
	a.accountsRun("", func() string {
		defer cancel()
		msg := acts.GrokLogin(ctx)
		runOnMain(func() { s.login = nil })
		return msg
	})
}

// accountsCancelLogin stops a running Grok browser login.
func (a *App) accountsCancelLogin() {
	if cancel := a.panelState.accounts.login; cancel != nil {
		cancel()
	}
}
