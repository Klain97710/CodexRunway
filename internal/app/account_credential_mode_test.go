package app

import (
	"context"
	"github.com/licoy/quosparr/internal/personal"
	"os"
	"path/filepath"
	"testing"

	"github.com/licoy/quosparr/internal/codex"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/runway"
)

func TestAccountActionsExplainUnsupportedCredentialMode(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte("cli_auth_credentials_store = 'keyring'"), 0600); err != nil {
		t.Fatal(err)
	}
	st := codex.NewStore(home, t.TempDir())
	svc := runway.New(runway.Options{Features: personal.Features{Grok: true}, Sources: runway.Sources{
		SwitchAccount: func(context.Context, string) (codex.SwitchResult, error) {
			return codex.SwitchResult{}, codex.ErrCredentialStoreUnsupported
		},
	}})
	defer svc.Close()
	rt := &liveApp{codex: st, svc: svc}
	previous := currentRuntime()
	setLive(rt)
	defer setLive(previous)
	for _, lang := range l10n.Languages() {
		l := l10n.New(lang)
		want := l.Text(l10n.KeyCodexCredentialStoreUnsupported)
		if got := rt.importCodexOfficial(l); got != want {
			t.Fatalf("%s import did not explain credential mode: %q", lang, got)
		}
		if got, ok := rtSwitchCodexResult(l, "fixture", false); ok || got != want {
			t.Fatalf("%s switch did not explain credential mode: %q", lang, got)
		}
	}
	if _, err := os.Stat(st.OfficialAuthPath()); !os.IsNotExist(err) {
		t.Fatal("unsupported account action wrote official credentials")
	}
}
