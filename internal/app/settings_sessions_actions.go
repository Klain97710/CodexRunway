package app

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/sessions"
)

func (u *settingsUI) sessionNotice(c *ui.Context) {
	s := u.sessionsState()
	if s.notice == "" {
		return
	}
	t := c.Theme()
	color, icon := t.Success, icCheckCircle
	if s.noticeErr {
		color, icon = t.Danger, icWarnCircle
	}
	ui.Row(c).Gap(8).Padding(8, 10).Radius(8).Background(color.Alpha(0.10)).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, icon).Size(13, 13).TextColor(color)
		ui.Text(c, s.notice).FontSize(12).TextColor(color).Grow(1).Basis(0)
	})
}

func (u *settingsUI) sessionDeleteDialog(c *ui.Context) {
	s := u.sessions
	if s == nil || !s.confirm {
		return
	}
	items := s.selectedItems()
	l := u.L()
	message := tx(l, xSessionsDeleteConfirmMessage, l.Text(providerKey(s.provider)), len(items))
	if ui.AlertDialog(c.Key("session-delete-confirm"), &s.confirm, tx(l, xSessionsDeleteConfirmTitle), message,
		l.Text(l10n.KeyCancel), tx(l, xSessionsDeleteSelected)) == 1 {
		u.deleteSessions(u.sessionActions(), items)
	}
}

func (u *settingsUI) deleteSessions(actions SessionActions, items []sessions.ManagedSession) {
	s := u.sessionsState()
	if len(items) == 0 || actions.Delete == nil || s.work.Running {
		return
	}
	provider := s.provider
	var results []sessions.DeleteResult
	runTask(&s.work, u.post, func() error {
		var err error
		results, err = actions.Delete(context.Background(), items)
		return err
	}, func(err error) {
		s.clearSelection()
		s.open = ""
		s.notice, s.noticeErr = sessionDeleteResultText(u.L(), results, err)
		if s.provider == provider {
			u.startSessionsScan(actions)
		}
	})
}

func sessionDeleteResultText(l l10n.Localizer, results []sessions.DeleteResult, overall error) (string, bool) {
	if len(results) == 0 {
		return tx(l, xSessionsDeleteFailed, overall), true
	}
	failed, removed, total := 0, 0, 0
	var details []string
	for _, result := range results {
		removed += result.Removed
		total += result.Total
		if result.Err == nil {
			continue
		}
		failed++
		name := result.Session.Title
		if name == "" {
			name = filepath.Base(result.Session.Path)
		}
		details = append(details, fmt.Sprintf("%s (%d/%d): %s", name, result.Removed, result.Total, result.Err))
	}
	if failed > 0 || overall != nil {
		if overall != nil && len(details) == 0 {
			details = append(details, overall.Error())
		}
		return tx(l, xSessionsDeletePartial, len(results)-failed, failed, removed, total, strings.Join(details, "; ")), true
	}
	return tx(l, xSessionsDeleteSuccess, len(results), removed), false
}
