package app

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	"github.com/egoist/mygo"
	"github.com/licoy/quosparr/internal/claude"
	"github.com/licoy/quosparr/internal/export/swiftjson"
	"github.com/licoy/quosparr/internal/grok"
	"github.com/licoy/quosparr/internal/sessions"
	"github.com/licoy/quosparr/internal/store"
	"github.com/licoy/quosparr/internal/usage"
)

var errSessionsUnavailable = errors.New("session management service unavailable")

func installSessionActions(rt *liveApp, actions *SessionActions) {
	actions.Query = rt.queryManagedSessions
	actions.Detail = sessions.ReadManagedDetail
	actions.Delete = rt.deleteManagedSessions
	actions.Reveal = func(path string) error {
		mygo.Shell.ShowItemInFolder(path)
		return nil
	}
}

func (rt *liveApp) queryManagedSessions(ctx context.Context, provider store.Provider) ([]sessions.ManagedSession, error) {
	switch provider {
	case store.ProviderCodex:
		home := rt.codexFolder()
		if home == "" {
			return nil, errSessionsUnavailable
		}
		return (sessions.Scanner{CodexHome: home, Pricer: managedSessionPricer}).ListManaged(ctx)
	case store.ProviderClaude:
		if rt.claudeMod == nil || rt.claudeMod.Store == nil {
			return nil, errSessionsUnavailable
		}
		var cache *claude.SessionCache
		if rt.claudeB != nil {
			cache = &rt.claudeB.sessions
		}
		return (claude.SessionScanner{
			ConfigDir: rt.claudeMod.Store.ConfigDir(), Home: rt.claudeMod.Store.HomeDir(),
			Location: time.Local, Cache: cache,
		}).ListManaged(ctx)
	case store.ProviderGrok:
		if rt.grokMod == nil || rt.grokMod.Store == nil {
			return nil, errSessionsUnavailable
		}
		return (grok.SessionScanner{GrokHome: rt.grokMod.Store.OfficialHome(), Location: time.Local}).ListManaged(ctx)
	default:
		return nil, errSessionsUnavailable
	}
}

func managedSessionPricer(model string, totals sessions.Totals) (swiftjson.Decimal, bool) {
	cost, ok := usage.CostForTotals(model, usage.Totals{
		TotalTokens: int64(totals.TotalTokens), UncachedInputTokens: int64(totals.UncachedInputTokens),
		CachedInputTokens: int64(totals.CachedInputTokens), OutputTokens: int64(totals.OutputTokens),
		Turns: int64(totals.Turns), Threads: int64(totals.Threads),
	})
	if !ok {
		return swiftjson.Decimal{}, false
	}
	value, err := swiftjson.ParseDecimal(cost.String())
	return value, err == nil
}

func (rt *liveApp) deleteManagedSessions(ctx context.Context, selected []sessions.ManagedSession) ([]sessions.DeleteResult, error) {
	results, err := sessions.DeleteManaged(ctx, selected, rt.sessionTrustedRoots(), mygo.Shell.TrashItem)
	removed := map[store.Provider]bool{}
	for _, result := range results {
		if result.Removed > 0 {
			removed[store.Provider(result.Session.Provider)] = true
		}
	}
	for provider := range removed {
		rt.refreshDeletedSessions(provider)
	}
	return results, err
}

func (rt *liveApp) sessionTrustedRoots() map[string][]string {
	roots := map[string][]string{}
	if home := rt.codexFolder(); home != "" {
		roots[sessions.ProviderCodex] = []string{filepath.Join(home, "sessions"), filepath.Join(home, "archived_sessions")}
	}
	if rt.claudeMod != nil && rt.claudeMod.Store != nil {
		roots[sessions.ProviderClaude] = claude.ProjectsDirs(rt.claudeMod.Store.ConfigDir(), rt.claudeMod.Store.HomeDir())
	}
	if rt.grokMod != nil && rt.grokMod.Store != nil {
		roots[sessions.ProviderGrok] = []string{filepath.Join(rt.grokMod.Store.OfficialHome(), "sessions")}
	}
	return roots
}

func (rt *liveApp) refreshDeletedSessions(provider store.Provider) {
	if rt.svc == nil {
		return
	}
	if provider == store.ProviderClaude || provider == store.ProviderGrok {
		rt.svc.RefreshProviderLocal(provider)
		return
	}
	if provider != store.ProviderCodex {
		return
	}
	rt.svc.RefreshRecentSessions(true)
	rt.svc.RefreshSpeed(true)
	rt.svc.RefreshCost(usage.RefreshForce)
	rt.svc.RefreshTokenHeatmap(usage.RefreshForce)
}
