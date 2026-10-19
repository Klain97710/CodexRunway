package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/personal"
	"github.com/licoy/quosparr/internal/sessions"
	"github.com/licoy/quosparr/internal/skills"
	"github.com/licoy/quosparr/internal/store"
)

func TestPersonalManagementScansOnlyCodex(t *testing.T) {
	rt, _ := sessionTestRuntime(t)
	rt.features = personal.Features{}
	for _, p := range store.Providers() {
		path := filepath.Join(rt.home, "."+string(p), "skills", "fixture", "SKILL.md")
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("---\nname: fixture\ndescription: Synthetic skill\n---\nTest body\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	got, err := rt.querySkills(context.Background())
	if err != nil || len(got) != 1 || len(got[store.ProviderCodex].Skills) != 1 {
		t.Fatalf("Codex-only scan: %d providers, %v", len(got), err)
	}
	if len(rt.sessionTrustedRoots()) != 1 {
		t.Fatal("disabled session roots exposed")
	}
	// Retained scanners remain available only with an explicit test opt-in.
	rt.features = personal.Features{Grok: true, Claude: true}
	got, err = rt.querySkills(context.Background())
	if err != nil || len(got) != 3 {
		t.Fatalf("opt-in scan: %d providers, %v", len(got), err)
	}
}

func TestPersonalManagementRejectsDisabledActions(t *testing.T) {
	rt, _ := sessionTestRuntime(t) // Even non-nil retained modules must be gated.
	rt.features = personal.Features{}
	var actions SkillActions
	installSkillActions(rt, &actions)
	var sessionActions SessionActions
	installSessionActions(rt, &sessionActions)
	ctx := context.Background()
	for _, p := range []store.Provider{store.ProviderClaude, store.ProviderGrok} {
		sk := skills.Skill{Provider: skills.Provider(p)}
		if _, err := rt.queryMCP(ctx, p); !errors.Is(err, errMCPUnavailable) {
			t.Fatal("disabled MCP query was dispatched")
		}
		if _, err := rt.deleteMCP(ctx, p, nil); !errors.Is(err, errMCPUnavailable) {
			t.Fatal("disabled MCP delete was dispatched")
		}
		if _, err := rt.queryManagedSessions(ctx, p); !errors.Is(err, errSessionsUnavailable) {
			t.Fatal("disabled session query was dispatched")
		}
		s := sessions.ManagedSession{Provider: string(p)}
		if _, err := sessionActions.Detail(ctx, s); !errors.Is(err, errSessionsUnavailable) {
			t.Fatal("disabled session detail was dispatched")
		}
		if _, err := sessionActions.Delete(ctx, []sessions.ManagedSession{s}); !errors.Is(err, errSessionsUnavailable) {
			t.Fatal("disabled session delete was dispatched")
		}
		if _, _, err := actions.ReadBody(sk); !errors.Is(err, errSkillsUnavailable) {
			t.Fatal("disabled skill read was dispatched")
		}
		if err := actions.SetEnabled(sk, true); !errors.Is(err, errSkillsUnavailable) {
			t.Fatal("disabled skill toggle was dispatched")
		}
		if _, err := actions.Delete(sk, nil); !errors.Is(err, errSkillsUnavailable) {
			t.Fatal("disabled skill delete was dispatched")
		}
		if _, err := actions.Inspect(ctx, sk); !errors.Is(err, errSkillsUnavailable) {
			t.Fatal("disabled skill inspect was dispatched")
		}
		for _, req := range []skills.TransferRequest{{Skill: sk, Target: skills.ProviderCodex}, {Skill: skills.Skill{Provider: skills.ProviderCodex}, Target: skills.Provider(p)}} {
			if _, err := actions.Transfer(req); !errors.Is(err, errSkillsUnavailable) {
				t.Fatal("disabled skill transfer was dispatched")
			}
		}
	}
	if _, err := (&liveApp{}).querySkills(ctx); !errors.Is(err, errSkillsUnavailable) {
		t.Fatal("missing home fell back to user data")
	}
}

func TestPersonalManagementHidesProviderTabsAndKeepsTrashMapping(t *testing.T) {
	u, tt, _, _ := settingsSkillsView(t, false)
	u.s.sys.Features = personal.Features{}
	tt.Frame()
	if hasExact(tt, "Claude") || hasExact(tt, "Grok") {
		t.Fatal("disabled skills tab visible")
	}
	if err := tt.Click(tx(l10n.New(l10n.English), xSkillsTrash)); err != nil {
		t.Fatal(err)
	}
	if u.skills.tab != skillsTrashTab {
		t.Fatal("Codex-only trash tab maps to a disabled provider")
	}
	if err := tt.Click("Codex"); err != nil {
		t.Fatal(err)
	}
	if u.skills.provider() != store.ProviderCodex || u.skills.tab == skillsTrashTab {
		t.Fatal("Codex tab does not restore the list")
	}
	u.skills.selectProvider(store.ProviderGrok)
	tt.Frame()
	if u.skills.provider() != store.ProviderCodex {
		t.Fatal("stale provider was not normalized")
	}
	m, mt, _ := settingsMCPView(t, MCPActions{})
	m.s.sys.Features = personal.Features{}
	m.mcp.switchProvider(store.ProviderGrok)
	mt.Frame()
	if hasExact(mt, "Claude") || hasExact(mt, "Grok") || m.mcp.provider != store.ProviderCodex {
		t.Fatal("MCP provider gate failed")
	}
	s, st, _ := settingsSessionsView(t, SessionActions{})
	s.s.sys.Features = personal.Features{}
	s.sessions.switchProvider(store.ProviderClaude)
	st.Frame()
	if hasExact(st, "Claude") || hasExact(st, "Grok") || s.sessions.provider != store.ProviderCodex {
		t.Fatal("session provider gate failed")
	}
}
