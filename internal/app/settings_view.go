package app

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/platform"
	"github.com/licoy/quosparr/internal/store"
)

// Settings window size in DIPs.
const (
	settingsWidth        = 980
	settingsHeight       = 720
	settingsMinWidth     = 820
	settingsMinHeight    = 560
	settingsSidebarWidth = 240
)

// Actions are the operations of other packages that the settings buttons
// start. A nil one disables its button.
type Actions struct {
	Refresh func()
	// SelfCheck builds the redacted local report on a worker.
	SelfCheck func() string
	// RepairIndex rebuilds the session index (after the user confirmed).
	RepairIndex func()
	// TestNotification sends a test notification and says what happened.
	TestNotification func() string
	CheckForUpdates  func()
	// OpenFolder opens a folder in the file manager.
	OpenFolder func(path string)
}

// SettingsOptions configures the settings window.
type SettingsOptions struct {
	Version string
	Actions Actions
}

type settingsTab int

const (
	tabGeneral settingsTab = iota
	tabAccounts
	tabDisplay
	tabAdvanced
	tabAbout
)

var settingsTabs = []settingsTab{tabGeneral, tabAccounts, tabDisplay, tabAdvanced, tabAbout}

func (t settingsTab) key() l10n.Key {
	return [...]l10n.Key{l10n.KeyGeneral, l10n.KeyAccounts, l10n.KeyDisplay, l10n.KeyAdvanced, l10n.KeyAbout}[t]
}

func (t settingsTab) icon() *ui.SVG {
	return [...]*ui.SVG{icGear, icUsers, icEye, icSliders, icInfo}[t]
}

func (t settingsTab) tint() Tint {
	return [...]Tint{TintGray, TintGreen, TintBlue, TintOrange, TintIndigo}[t]
}

//go:embed assets/app-icon.png
var appIconPNG []byte

var (
	appIconOnce sync.Once
	appIcon     *ui.Bitmap
)

func appIconBitmap() *ui.Bitmap {
	appIconOnce.Do(func() { appIcon, _ = ui.DecodeBitmap(appIconPNG) })
	return appIcon
}

const (
	githubURL   = "https://github.com/Licoy/Quosparr"
	feedbackURL = "https://github.com/Licoy/Quosparr/issues/new"

	// loginItemsSettingsURL opens System Settings > General > Login Items
	// (macOS 13+), where a login item waiting for approval is allowed.
	loginItemsSettingsURL = "x-apple.systempreferences:com.apple.LoginItems-Settings.extension"
)

// settingsUI is the state of the settings window's view: which tab, the
// proxy form being edited, and the dialogs. Preferences live in Settings.
type settingsUI struct {
	s    *Settings
	opts SettingsOptions
	win  *mygo.Window // nil in tests

	tab      settingsTab
	accounts *accountForm

	draft         ProxyDraft
	draftReady    bool
	proxyTask     Task
	proxyResult   ProxySaveResult
	testTask      Task
	selfCheckTask Task
	proxyMsg      string
	proxyMsgErr   bool

	confirmRepair bool
	noticeTitle   string
	notice        string
	noticeOpen    bool
}

func newSettingsUI(s *Settings, opts SettingsOptions) *settingsUI {
	return &settingsUI{s: s, opts: opts}
}

// post runs fn on the main thread and draws a new frame.
func (u *settingsUI) post(fn func()) {
	if u.win != nil && !u.win.IsDestroyed() {
		u.win.Update(fn)
		return
	}
	runOnMain(fn)
}

func (u *settingsUI) L() l10n.Localizer { return u.s.L() }

func (u *settingsUI) text(k l10n.Key) string { return u.L().Text(k) }

// view builds one frame of the settings window.
func (u *settingsUI) view(c *ui.Context) {
	applyTheme(c)
	if !u.draftReady {
		u.draft, u.draftReady = NewProxyDraft(u.s.Proxy()), true
	}
	u.shell(c, u.pageContent)
	u.dialogs(c)
}

func (u *settingsUI) pageContent(c *ui.Context) {
	ui.Scroll(c.Key("page-" + fmt.Sprint(int(u.tab)))).Grow(1).Basis(0).Padding(16).Gap(14).Children(func() {
		if u.s.ReadOnly() {
			u.readOnlyBanner(c)
		}
		if err := u.s.SaveError(); err != nil {
			ui.Text(c, tx(u.L(), xSettingsSaveFailed, err.Error())).FontSize(12).TextColor(c.Theme().Danger)
		}
		switch u.tab {
		case tabGeneral:
			u.generalPage(c)
		case tabAccounts:
			u.accountsPage(c)
		case tabDisplay:
			u.displayPage(c)
		case tabAdvanced:
			u.advancedPage(c)
		case tabAbout:
			u.aboutPage(c)
		}
	})
}

// readOnlyBanner explains why nothing can be changed.
func (u *settingsUI) readOnlyBanner(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Gap(4).Padding(12).Radius(12).Background(t.Danger.Alpha(0.10)).Children(func() {
		ui.Text(c, tx(u.L(), xSettingsUnreadable)).FontSize(13).FontWeight(600).TextColor(t.Danger)
		ui.Text(c, u.s.LoadError().Error()).FontSize(11).TextColor(t.Danger)
		ui.Text(c, tx(u.L(), xSettingsReadOnly)).FontSize(11).TextColor(t.TextMuted)
	})
}

// dialogs draws the alerts that are open.
func (u *settingsUI) dialogs(c *ui.Context) {
	if u.confirmRepair {
		l := u.L()
		if ui.AlertDialog(c.Key("repair-confirm"), &u.confirmRepair,
			l.Text(l10n.KeyRepairConfirmTitle), l.Text(l10n.KeyRepairConfirmMessage),
			l.Text(l10n.KeyCancel), l.Text(l10n.KeyRepair)) == 1 && u.opts.Actions.RepairIndex != nil {
			u.opts.Actions.RepairIndex()
		}
	}
	if u.noticeOpen {
		ui.AlertDialog(c.Key("notice"), &u.noticeOpen, u.noticeTitle, u.notice, u.text(l10n.KeyOk))
	}
}

func (u *settingsUI) showNotice(title, message string) {
	u.noticeTitle, u.notice, u.noticeOpen = title, message, true
}

// ---- Labels of intervals ----

func (u *settingsUI) minutes(seconds int) string {
	return fmt.Sprintf("%d %s", seconds/60, u.text(l10n.KeyMinutes))
}

func (u *settingsUI) widgetInterval(seconds int) string {
	if seconds == 60 {
		return "60 " + u.text(l10n.KeySeconds)
	}
	return u.minutes(seconds)
}

func (u *settingsUI) resetTodayInterval(seconds int) string {
	if seconds < 3_600 {
		return u.minutes(seconds)
	}
	return fmt.Sprintf("%d %s", seconds/3_600, u.text(l10n.KeyHours))
}

// ---- General ----

func (u *settingsUI) generalPage(c *ui.Context) {
	s, l := u.s, u.L()
	p := s.Prefs()
	ro := s.ReadOnly()
	sys := s.System()
	settingsGroup(c, icGear, TintBlue, u.text(l10n.KeyGeneral), "", nil, func(row rowFunc) {
		row(l.Text(l10n.KeyLanguage), "", func() {
			langs := store.Languages()
			if v, ok := choose(c, "language", l.Text(l10n.KeyLanguage), p.Language, langs, func(v store.Language) string {
				return l10n.MenuTitle(string(v), l.Language())
			}, ro); ok {
				s.SetLanguage(v)
			}
		})
		detail := l.Text(l10n.KeyLaunchAtLoginDescription)
		switch {
		case s.LoginError() != nil:
			detail = l.Text(l10n.KeyLaunchAtLoginFailed)
		case !s.LoginAvailable():
			detail = l.Text(l10n.KeyLaunchAtLoginUnavailable)
		case s.LoginStatus() == LoginRequiresApproval:
			detail = l.Text(l10n.KeyLaunchAtLoginRequiresApproval)
		}
		row(l.Text(l10n.KeyLaunchAtLogin), detail, func() {
			if s.LoginPending() {
				ui.Spinner(c).Size(14, 14)
			}
			if s.LoginStatus() == LoginRequiresApproval && PillButton(c, icGear, l.Text(l10n.KeyOpenLoginItemsSettings), false).Clicked() {
				c.OpenURL(loginItemsSettingsURL)
			}
			if v, ok := toggle(c, "login", l.Text(l10n.KeyLaunchAtLogin), s.LoginSwitch(), ro || !s.LoginAvailable() || s.LoginPending()); ok {
				s.ChangeLaunchAtLogin(v)
			}
		})
		row(l.Text(l10n.KeyRefreshInterval), "", func() {
			opts := []int{60, 300, 600, 900, 1_800}
			if v, ok := choose(c, "refresh", l.Text(l10n.KeyRefreshInterval), p.RefreshIntervalSeconds, opts, u.minutes, ro); ok {
				s.SetRefreshInterval(v)
			}
		})
		detail = l.Text(l10n.KeyWidgetRefreshIntervalDescription)
		if !sys.Widgets {
			detail = tx(l, xWidgetMac14)
		}
		row(l.Text(l10n.KeyWidgetRefreshInterval), detail, func() {
			if v, ok := choose(c, "widget-refresh", l.Text(l10n.KeyWidgetRefreshInterval), p.WidgetRefreshIntervalSeconds,
				store.WidgetRefreshIntervalOptions(), u.widgetInterval, ro || !sys.Widgets); ok {
				s.SetWidgetRefreshInterval(v)
			}
		})
		row(l.Text(l10n.KeyCodexFolder), "~/.codex", func() {
			if PillButton(c, icFolder, l.Text(l10n.KeyCodexFolder), u.opts.Actions.OpenFolder == nil).Clicked() {
				u.opts.Actions.OpenFolder(codexDir())
			}
		})
	})
	u.proxyGroup(c)
}

// codexDir is ~/.codex, for opening in the file manager.
func codexDir() string {
	home, err := platform.Home()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".codex")
}

// openFolder opens a folder in the file manager.
func openFolder(path string) {
	if path == "" {
		return
	}
	if _, err := os.Stat(path); err != nil {
		return
	}
	_ = mygo.Shell.OpenPath(path)
}

// ---- Network proxy ----

func (u *settingsUI) proxyGroup(c *ui.Context) {
	s, l := u.s, u.L()
	t := c.Theme()
	busy := u.proxyTask.Running || u.testTask.Running
	settingsGroup(c, icNetwork, TintTeal, l.Text(l10n.KeyNetworkProxy), "", nil, func(row rowFunc) {
		if err := s.ProxyError(); err != nil {
			row("", "", func() {
				ui.Column(c).Grow(1).Basis(0).Gap(3).Children(func() {
					ui.Text(c, l.Text(proxyErrorKey(err))).FontSize(12).TextColor(t.Danger)
					ui.Text(c, tx(l, xProxyBlocked)).FontSize(11).TextColor(t.TextMuted)
				})
			})
		}
		row(l.Text(l10n.KeyNetworkProxyMode), "", func() {
			modes := store.ProxyModes()
			if v, ok := pills(c, "px-mode", u.draft.Mode, modes, func(m store.ProxyMode) string {
				switch m {
				case store.ProxyModeHTTP:
					return l.Text(l10n.KeyNetworkProxyHTTP)
				case store.ProxyModeSOCKS5:
					return l.Text(l10n.KeyNetworkProxySOCKS5)
				}
				return l.Text(l10n.KeyNetworkProxySystem)
			}, busy); ok {
				u.draft.Mode = v
			}
		})
		if u.draft.Mode != store.ProxyModeSystem {
			u.proxyFields(c, row, busy)
		}
		u.proxyActions(c, busy)
	})
}

// proxyFields draws the manual proxy form, including optional authentication.
func (u *settingsUI) proxyFields(c *ui.Context, row rowFunc, busy bool) {
	l := u.L()
	row(l.Text(l10n.KeyNetworkProxyHost), "", func() {
		ui.TextInput(c.Key("px-host"), &u.draft.Host).Width(230).Placeholder("127.0.0.1").Label(l.Text(l10n.KeyNetworkProxyHost)).Disabled(busy)
	})
	row(l.Text(l10n.KeyNetworkProxyPort), "", func() {
		ui.TextInput(c.Key("px-port"), &u.draft.Port).Width(230).Placeholder("8080").Label(l.Text(l10n.KeyNetworkProxyPort)).Disabled(busy)
	})
	row(l.Text(l10n.KeyNetworkProxyAuthentication), "", func() {
		if v, ok := toggle(c, "px-auth", l.Text(l10n.KeyNetworkProxyAuthentication), u.draft.UsesAuth, busy); ok {
			u.draft.UsesAuth = v
		}
	})
	if !u.draft.UsesAuth {
		return
	}
	row(l.Text(l10n.KeyNetworkProxyUsername), "", func() {
		ui.TextInput(c.Key("px-user"), &u.draft.Username).Width(230).Label(l.Text(l10n.KeyNetworkProxyUsername)).Disabled(busy)
	})
	detail := ""
	if u.draft.CredentialID != "" {
		detail = l.Text(l10n.KeyNetworkProxySavedCredentials)
	}
	row(l.Text(l10n.KeyNetworkProxyPassword), detail, func() {
		ui.TextInput(c.Key("px-pass"), &u.draft.Password).Password().Width(230).Label(l.Text(l10n.KeyNetworkProxyPassword)).Disabled(busy)
	})
	if _, ok := u.s.creds.(UnavailableCredentials); ok {
		row("", tx(l, xProxyLocalSaveWarning), nil)
	}
	if u.draft.CredentialID != "" {
		row("", "", func() {
			if PillButton(c, nil, l.Text(l10n.KeyNetworkProxyLoadCredentials), busy).Clicked() {
				u.loadCredentials()
			}
		})
	}
}

// proxyActions is the final row inside the network card.
func (u *settingsUI) proxyActions(c *ui.Context, busy bool) {
	l := u.L()
	t := c.Theme()
	ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(8, 0, 0, 0).Children(func() {
		col := t.Success
		if u.proxyMsgErr {
			col = t.Danger
		}
		ui.Text(c, u.proxyMsg).FontSize(12).TextColor(col).Grow(1).Basis(0)
		if u.s.proxyHooks.Test != nil {
			label := l.Text(l10n.KeyNetworkProxyTest)
			if u.testTask.Running {
				label = l.Text(l10n.KeyNetworkProxyTesting)
			}
			if PillButton(c, nil, label, busy).Clicked() {
				u.testProxy()
			}
		}
		if PillButton(c, icCheck, l.Text(l10n.KeyNetworkProxySave), busy).Clicked() {
			u.saveProxy()
		}
	})
}

func (u *settingsUI) setProxyMsg(msg string, isErr bool) { u.proxyMsg, u.proxyMsgErr = msg, isErr }

func (u *settingsUI) saveProxy() {
	d, prev := u.draft, u.s.Proxy()
	u.setProxyMsg("", false)
	runTask(&u.proxyTask, u.post, func() error {
		r, err := u.s.SaveProxy(d, prev)
		u.proxyResult = r
		return err
	}, func(err error) {
		l := u.L()
		if err != nil {
			u.setProxyMsg(l.Text(proxyErrorKey(err)), true)
			return
		}
		r := u.proxyResult
		u.s.ProxySaved(r)
		u.draft = NewProxyDraft(r.Config)
		if r.CleanupFailed {
			u.setProxyMsg(l.Text(l10n.KeyNetworkProxyCleanupFailed), true)
			return
		}
		u.setProxyMsg(l.Text(l10n.KeyNetworkProxySaved), false)
	})
}

func (u *settingsUI) loadCredentials() {
	id := u.draft.CredentialID
	var got ProxyCredentials
	runTask(&u.proxyTask, u.post, func() error {
		c, err := u.s.LoadProxyCredentials(id)
		got = c
		return err
	}, func(err error) {
		if err != nil {
			u.setProxyMsg(u.L().Text(proxyErrorKey(err)), true)
			return
		}
		u.draft.Username, u.draft.Password = got.Username, got.Password
		u.setProxyMsg("", false)
	})
}

func (u *settingsUI) testProxy() {
	d := u.draft
	u.setProxyMsg("", false)
	runTask(&u.testTask, u.post, func() error {
		cfg, err := d.Configuration()
		if err != nil {
			return err
		}
		creds, err := credentialsFor(cfg, d.entered(), u.s.creds)
		if err != nil {
			return err
		}
		return u.s.proxyHooks.Test(context.Background(), cfg, creds)
	}, func(err error) {
		if err != nil {
			u.setProxyMsg(u.L().Text(proxyErrorKey(err)), true)
			return
		}
		u.setProxyMsg(u.L().Text(l10n.KeyNetworkProxyTestSucceeded), false)
	})
}

// ---- Accounts ----

func (u *settingsUI) accountsPage(c *ui.Context) {
	renderAccountsPage(c, u)
}

// ---- Advanced ----

func (u *settingsUI) advancedPage(c *ui.Context) {
	s, l := u.s, u.L()
	p := s.Prefs()
	a := u.opts.Actions
	settingsGroup(c, icSliders, TintOrange, l.Text(l10n.KeyAdvanced), "", nil, func(row rowFunc) {
		row(l.Text(l10n.KeyRefresh), l.Text(l10n.KeyQuota), func() {
			if PillButton(c, icRefresh, l.Text(l10n.KeyRefresh), a.Refresh == nil).Clicked() {
				a.Refresh()
			}
		})
		row(l.Text(l10n.KeySelfCheck), l.Text(l10n.KeySessionRepair), func() {
			if u.selfCheckTask.Running {
				ui.Spinner(c).Size(14, 14)
			}
			if PillButton(c, icSelfCheck, l.Text(l10n.KeySelfCheck), a.SelfCheck == nil || u.selfCheckTask.Running).Clicked() {
				var report string
				runTask(&u.selfCheckTask, u.post, func() error {
					report = a.SelfCheck()
					return nil
				}, func(error) { u.showNotice(u.text(l10n.KeySelfCheck), report) })
			}
		})
		row(l.Text(l10n.KeyExportStatusJSON), "~/.codex-runway/status.json", func() {
			if v, ok := toggle(c, "export-json", l.Text(l10n.KeyExportStatusJSON), p.ExportsStatusJSON, s.ReadOnly()); ok {
				s.SetExportsStatusJSON(v)
			}
		})
		row(l.Text(l10n.KeyRepairIndex), l.Text(l10n.KeyBackup), func() {
			if PillButton(c, icRepair, l.Text(l10n.KeyRepair), a.RepairIndex == nil).Clicked() {
				u.confirmRepair = true
			}
		})
	})
}

// ---- About ----

func (u *settingsUI) aboutPage(c *ui.Context) {
	s, l := u.s, u.L()
	p := s.Prefs()
	t := c.Theme()
	a := u.opts.Actions
	ui.Row(c).Justify(ui.Center).Padding(4, 0).Children(func() {
		if bm := appIconBitmap(); bm != nil {
			ui.Image(c, bm).Size(96, 96).Label("CodexRunway")
		}
	})
	settingsGroup(c, icInfo, TintIndigo, l.Text(l10n.KeyAbout), "", nil, func(row rowFunc) {
		row(l.Text(l10n.KeyVersion), valueOr(u.opts.Version, "dev"), func() {
			ui.Text(c, tx(l, xPersonalEdition)).FontSize(13).TextColor(t.TextMuted)
		})
		row(l.Text(l10n.KeyAutomaticallyCheckForUpdates), l.Text(l10n.KeyCheckForUpdates), func() {
			if v, ok := toggle(c, "auto-update", l.Text(l10n.KeyAutomaticallyCheckForUpdates), p.AutomaticallyChecksForUpdates, s.ReadOnly()); ok {
				s.SetAutomaticUpdateChecks(v)
			}
		})
		row(l.Text(l10n.KeyCheckForUpdates), l.Text(l10n.KeyVersion), func() {
			if PillButton(c, icDownload, l.Text(l10n.KeyCheckForUpdates), a.CheckForUpdates == nil).Clicked() {
				a.CheckForUpdates()
			}
		})
		row(tx(l, xOriginalProject), "Licoy/Quosparr · MIT", func() {
			if PillButton(c, icGithub, "Licoy/Quosparr", false).Clicked() {
				c.OpenURL("https://github.com/Licoy/Quosparr")
			}
		})
		row("GitHub", "github.com/Klain97710/CodexRunway", func() {
			if PillButton(c, icGithub, l.Text(l10n.KeyOpenGithub), false).Clicked() {
				c.OpenURL(githubURL)
			}
		})
		row(l.Text(l10n.KeyFeedbackIssue), "GitHub Issues", func() {
			if PillButton(c, icBug, l.Text(l10n.KeyFeedbackIssue), false).Clicked() {
				c.OpenURL(feedbackURL)
			}
		})
	})
}
