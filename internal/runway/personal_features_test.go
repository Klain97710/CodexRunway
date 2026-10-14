package runway

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/export"
	"github.com/licoy/quosparr/internal/personal"
	"github.com/licoy/quosparr/internal/store"
)

type forbiddenGrok struct{ calls atomic.Int32 }

func (g *forbiddenGrok) State(time.Time) GrokState     { g.calls.Add(1); return GrokState{} }
func (g *forbiddenGrok) Refresh(context.Context) error { g.calls.Add(1); return nil }
func (g *forbiddenGrok) SetOnChange(func())            { g.calls.Add(1) }

func TestPersonalRefreshTicksAndWidgetNeverCallGrok(t *testing.T) {
	f := newFake(t)
	g := &forbiddenGrok{}
	r := &fakeResetToday{now: f.clk.Now}
	s, _ := newService(t, f, func(o *Options) {
		o.Features = personal.Features{}
		o.Grok, o.ResetToday = g, r
		o.Preferences.SelectedProvider = store.ProviderGrok
		o.Preferences.StatusBarProviderScope = store.ProviderScopeBoth
		o.Preferences.ShowsRateLimitResetToday = true
		o.WidgetRequirements = export.RequireAll
	})
	refreshAll(t, s)
	for i := 0; i < 3; i++ {
		s.SelectProvider(store.ProviderGrok)
		s.Tick(f.clk.Advance(6 * time.Minute))
		settle(t, s)
		refreshAll(t, s)
	}
	if g.calls.Load() != 0 {
		t.Fatalf("Grok calls: %d", g.calls.Load())
	}
	if r.count() == 0 || f.count("quota") == 0 {
		t.Fatal("positive Codex/status controls did not run")
	}
	if s.Preferences().SelectedProvider != store.ProviderCodex || s.Preferences().StatusBarProviderScope != store.ProviderScopeSelected {
		t.Fatal("legacy preferences not normalized")
	}
	snap := s.WidgetSnapshot(f.clk.Now())
	if len(snap.Providers) != 1 || snap.Providers[0].Provider != export.ProviderCodex {
		t.Fatal("Widget leaked Grok")
	}
}
