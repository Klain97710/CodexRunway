package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/mcp"
	"github.com/licoy/quosparr/internal/store"
)

func mcpFixtureServer(id, name string) mcp.Server {
	return mcp.Server{ID: id, Name: name, Provider: mcp.ProviderCodex, Path: "/synthetic/config.toml", Transport: "stdio",
		Command: "server", Details: `{"command":"server","args":["<hidden>"]}`, Enabled: true, Deletable: true, Fingerprint: "v1"}
}

func settingsMCPView(t *testing.T, actions MCPActions) (*settingsUI, *ui.Tester, chan func()) {
	t.Helper()
	e := newEnv(t, nil)
	if err := e.s.Load(); err != nil {
		t.Log("load:", err)
	}
	e.s.Apply()
	queue := detailMainQueue(t)
	u := newSettingsUI(e.s, SettingsOptions{Version: "9.9.9", MCP: func() MCPActions { return actions }})
	u.tab = tabMCP
	s := u.mcpState()
	s.loaded = true
	s.snapshot = mcp.Snapshot{Path: "/synthetic/config.toml", Fingerprint: "v1", Servers: []mcp.Server{
		mcpFixtureServer("codex\x00alpha", "alpha"),
		{ID: "codex\x00remote", Name: "remote", Provider: mcp.ProviderCodex, Path: "/synthetic/config.toml", Transport: "http", URL: "https://mcp.example", Details: `{"url":"https://mcp.example"}`, Enabled: true, Deletable: true, Fingerprint: "v1"},
		{ID: "codex\x00managed", Name: "managed", Provider: mcp.ProviderCodex, Path: "/synthetic/config.toml", Problem: "multipleTransports", Enabled: true, Deletable: false, Fingerprint: "v1"},
	}}
	tt := ui.NewTester(u.view, settingsWidth, settingsHeight)
	tt.Frame()
	return u, tt, queue
}

func TestMCPScanDropsStaleProviderResult(t *testing.T) {
	e := newEnv(t, nil)
	queue := detailMainQueue(t)
	u := newSettingsUI(e.s, SettingsOptions{})
	codex, claude := make(chan struct{}), make(chan struct{})
	actions := MCPActions{Query: func(_ context.Context, provider store.Provider) (mcp.Snapshot, error) {
		if provider == store.ProviderCodex {
			<-codex
			return mcp.Snapshot{Path: "codex"}, nil
		}
		<-claude
		return mcp.Snapshot{Path: "claude"}, nil
	}}
	u.startMCPScan(actions)
	u.mcpState().switchProvider(store.ProviderClaude)
	u.startMCPScan(actions)
	close(codex)
	finishDetail(t, queue)
	if u.mcp.loaded || u.mcp.snapshot.Path != "" {
		t.Fatalf("stale result landed: %+v", u.mcp.snapshot)
	}
	close(claude)
	finishDetail(t, queue)
	if !u.mcp.loaded || u.mcp.snapshot.Path != "claude" {
		t.Fatalf("current result missing: %+v", u.mcp.snapshot)
	}
}

func TestCancelMCPTasksStopsScanAndDropsCallback(t *testing.T) {
	e := newEnv(t, nil)
	queue := detailMainQueue(t)
	u := newSettingsUI(e.s, SettingsOptions{})
	started := make(chan struct{})
	u.startMCPScan(MCPActions{Query: func(ctx context.Context, _ store.Provider) (mcp.Snapshot, error) {
		close(started)
		<-ctx.Done()
		return mcp.Snapshot{Path: "late"}, ctx.Err()
	}})
	<-started
	u.cancelMCPTasks()
	finishDetail(t, queue)
	if u.mcp.loaded || u.mcp.snapshot.Path != "" || u.mcp.cancel != nil {
		t.Fatalf("cancelled scan wrote state: %+v", u.mcp)
	}
}

func TestMCPSelectFilteredReplacesHiddenSelection(t *testing.T) {
	u, tt, _ := settingsMCPView(t, MCPActions{})
	u.mcp.selected["codex\x00alpha"] = true
	u.mcp.search = "http"
	tt.Frame()
	if err := tt.Click(tx(u.L(), xMCPSelectFiltered, 1)); err != nil {
		t.Fatal(err)
	}
	if u.mcp.selected["codex\x00alpha"] || !u.mcp.selected["codex\x00remote"] || len(u.mcp.selected) != 1 {
		t.Fatalf("selection = %#v", u.mcp.selected)
	}
}

func TestMCPSearchMatchesLocalizedProblemText(t *testing.T) {
	u, _, _ := settingsMCPView(t, MCPActions{})
	u.mcp.search = tx(u.L(), xMCPMultipleTransports)
	got := u.mcp.filtered(u.L())
	if len(got) != 1 || got[0].Name != "managed" {
		t.Fatalf("localized problem search = %+v", got)
	}
}

func TestMCPDeleteConfirmationCancelAndBackground(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	deleted := 0
	actions := MCPActions{
		Query: func(context.Context, store.Provider) (mcp.Snapshot, error) {
			return mcp.Snapshot{Path: "/synthetic/config.toml", Fingerprint: "v2"}, nil
		},
		Delete: func(_ context.Context, _ store.Provider, selected []mcp.Server) ([]mcp.DeleteResult, error) {
			deleted++
			close(started)
			<-release
			return []mcp.DeleteResult{{Server: selected[0]}}, nil
		},
	}
	u, tt, queue := settingsMCPView(t, actions)
	u.mcp.selected["codex\x00alpha"] = true
	u.mcp.confirm = true
	tt.Frame()
	message := tx(u.L(), xMCPDeleteConfirmMessage, "Codex", 1)
	if !tt.HasText(message) || !strings.Contains(message, "automatic backup") {
		t.Fatalf("confirmation = %q", tt.Texts())
	}
	if err := tt.Click(u.text(l10n.KeyCancel)); err != nil {
		t.Fatal(err)
	}
	if deleted != 0 || u.mcp.confirm {
		t.Fatal("cancelled confirmation started deletion")
	}
	u.mcp.confirm = true
	tt.Frame()
	cancel, ok := tt.Find(u.text(l10n.KeyCancel))
	if !ok {
		t.Fatal("cancel button missing")
	}
	tt.ClickAt(cancel.X+cancel.W+28, cancel.Y+cancel.H/2)
	<-started
	if !u.mcp.work.Running {
		t.Fatal("delete did not stay on background task")
	}
	before := u.mcp.provider
	_ = tt.Click("Claude")
	if u.mcp.provider != before {
		t.Fatal("provider changed during deletion")
	}
	close(release)
	finishDetail(t, queue)
	finishDetail(t, queue)
	if u.mcp.work.Running || u.mcp.stale || !u.mcp.loaded || u.mcp.snapshot.Fingerprint != "v2" {
		t.Fatalf("delete/refresh state = %+v", u.mcp)
	}
}

func TestMCPRefreshAndDeleteFailuresKeepExistingData(t *testing.T) {
	refreshErr := errors.New("refresh failed")
	u, _, queue := settingsMCPView(t, MCPActions{})
	want := len(u.mcp.snapshot.Servers)
	u.startMCPScan(MCPActions{Query: func(context.Context, store.Provider) (mcp.Snapshot, error) { return mcp.Snapshot{}, refreshErr }})
	finishDetail(t, queue)
	if len(u.mcp.snapshot.Servers) != want || !u.mcp.loaded || !errors.Is(u.mcp.task.Err, refreshErr) {
		t.Fatal("refresh failure discarded existing snapshot")
	}
	server := u.mcp.snapshot.Servers[0]
	u.deleteMCPServers(MCPActions{Delete: func(context.Context, store.Provider, []mcp.Server) ([]mcp.DeleteResult, error) {
		return []mcp.DeleteResult{{Server: server, Err: errors.New("changed")}}, errors.New("delete failed")
	}}, []mcp.Server{server})
	finishDetail(t, queue)
	if len(u.mcp.snapshot.Servers) != want || !u.mcp.noticeErr {
		t.Fatal("delete failure discarded existing snapshot")
	}
}

func TestMCPConsecutiveDeletesUseRefreshedFingerprint(t *testing.T) {
	servers := []mcp.Server{mcpFixtureServer("codex\x00one", "one"), mcpFixtureServer("codex\x00two", "two")}
	fingerprint, deletes := "v1", 0
	query := func(context.Context, store.Provider) (mcp.Snapshot, error) {
		copyServers := append([]mcp.Server(nil), servers...)
		for i := range copyServers {
			copyServers[i].Fingerprint = fingerprint
		}
		return mcp.Snapshot{Servers: copyServers, Fingerprint: fingerprint}, nil
	}
	deleteAction := func(_ context.Context, _ store.Provider, selected []mcp.Server) ([]mcp.DeleteResult, error) {
		if len(selected) != 1 || selected[0].Fingerprint != fingerprint {
			return nil, errors.New("stale fingerprint")
		}
		deletes++
		result := []mcp.DeleteResult{{Server: selected[0]}}
		servers = servers[1:]
		fingerprint = "v" + string(rune('1'+deletes))
		return result, nil
	}
	actions := MCPActions{Query: query, Delete: deleteAction}
	u, _, queue := settingsMCPView(t, actions)
	u.mcp.snapshot.Servers = append([]mcp.Server(nil), servers...)
	for range 2 {
		server := u.mcp.snapshot.Servers[0]
		u.deleteMCPServers(actions, []mcp.Server{server})
		finishDetail(t, queue)
		finishDetail(t, queue)
	}
	if deletes != 2 || len(u.mcp.snapshot.Servers) != 0 || u.mcp.stale {
		t.Fatalf("deletes=%d state=%+v", deletes, u.mcp)
	}
}

func TestMCPProblemCodesAreLocalizedInAllLanguages(t *testing.T) {
	for _, language := range l10n.Languages() {
		l := l10n.New(language)
		for _, problem := range []string{"missingTransport", "multipleTransports", "invalidTransport", "missingType", "transportMismatch", "futureProblem"} {
			got := mcpProblemText(l, problem)
			if got == "" || got == problem {
				t.Fatalf("language %s problem %s = %q", language, problem, got)
			}
		}
	}
}

func TestSettingsMCPListAndRedactedDetail(t *testing.T) {
	u, tt, _ := settingsMCPView(t, MCPActions{})
	for _, text := range []string{"Codex", "Claude", "Grok", "alpha", "remote", tx(u.L(), xMCPScopeInfo)} {
		if !tt.HasText(text) {
			t.Fatalf("MCP list missing %q; texts %q", text, tt.Texts())
		}
	}
	u.mcp.open = "codex\x00managed"
	tt.Frame()
	for _, text := range []string{tx(u.L(), xMCPPath), tx(u.L(), xMCPMultipleTransports), tx(u.L(), xMCPReadOnly)} {
		if !tt.HasText(text) {
			t.Fatalf("MCP detail missing %q; texts %q", text, tt.Texts())
		}
	}
	if tt.HasText("multipleTransports") {
		t.Fatalf("raw problem code leaked; texts %q", tt.Texts())
	}
}

func TestExportSettingsMCPReview(t *testing.T) {
	if os.Getenv("RUNWAY_MCP_QA") == "" {
		t.Skip("set RUNWAY_MCP_QA=1 to export offscreen PNGs")
	}
	u, _, _ := settingsMCPView(t, MCPActions{})
	u.s.SetLanguage(store.LanguageSimplifiedChinese)
	dir := filepath.Join("..", "..", ".build", "v1-mcp-qa")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, width := range []int{settingsWidth, settingsMinWidth} {
		for _, dark := range []bool{false, true} {
			name := "light"
			if dark {
				name = "dark"
			}
			view := ui.NewTester(u.view, width, settingsHeight)
			view.SetDark(dark)
			exportReviewPNG(t, view, filepath.Join(dir, "mcp-list-"+name+"-"+strconv.Itoa(width)+".png"))
		}
	}
	u.mcp.open = "codex\x00alpha"
	detail := ui.NewTester(u.view, settingsWidth, settingsHeight)
	exportReviewPNG(t, detail, filepath.Join(dir, "mcp-detail-zh-Hans.png"))
	u.mcp.open, u.mcp.selected["codex\x00alpha"], u.mcp.confirm = "", true, true
	confirm := ui.NewTester(u.view, settingsMinWidth, settingsHeight)
	exportReviewPNG(t, confirm, filepath.Join(dir, "mcp-confirm-zh-Hans.png"))
}
