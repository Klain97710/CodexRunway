package app

import (
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/store"
)

// Action results open a dialog titled with the tab's provider instead of a
// line that stayed under the other tabs.
func TestSettingsAccountResultOpensDialog(t *testing.T) {
	e := newEnv(t, nil)
	if err := e.s.Load(); err != nil {
		t.Fatal(err)
	}
	u := newSettingsUI(e.s, SettingsOptions{})
	f := accountFormOf(u)
	f.kind = accountKindClaude
	u.accountAct(f, "", func() string { return u.L().Text(l10n.KeyClaudeNotLoggedIn) })
	deadline := time.Now().Add(time.Second)
	for f.busy && time.Now().Before(deadline) {
		e.q.drain()
		time.Sleep(time.Millisecond)
	}
	if f.status != "" || !u.noticeOpen || u.noticeTitle != u.L().Text(l10n.KeyProviderClaude) || u.notice != u.L().Text(l10n.KeyClaudeNotLoggedIn) {
		t.Fatalf("status=%q open=%v title=%q notice=%q", f.status, u.noticeOpen, u.noticeTitle, u.notice)
	}
}

func TestPanelAccountResultRemembersItsProvider(t *testing.T) {
	q := &mainQueue{}
	t.Cleanup(setRunOnMain(q.hop))
	a := &App{}
	s := &a.panelState.accounts
	s.page = store.ProviderClaude
	a.accountsRun("", func() string { return "failed" })
	s.page = store.ProviderCodex
	deadline := time.Now().Add(time.Second)
	for s.busy && time.Now().Before(deadline) {
		q.drain()
		time.Sleep(time.Millisecond)
	}
	if !s.noticeOpen || s.notice != "failed" || s.noticeProvider != store.ProviderClaude {
		t.Fatalf("open=%v notice=%q provider=%q", s.noticeOpen, s.notice, s.noticeProvider)
	}
}
