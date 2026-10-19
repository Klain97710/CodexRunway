package app

import (
	"context"
	"errors"
	"time"

	"github.com/egoist/mygo"
	"github.com/licoy/quosparr/internal/mcp"
	"github.com/licoy/quosparr/internal/store"
)

var errMCPUnavailable = errors.New("MCP service unavailable")

func installMCPActions(rt *liveApp, actions *MCPActions) {
	actions.Query = rt.queryMCP
	actions.Delete = rt.deleteMCP
	actions.Reveal = func(path string) error {
		mygo.Shell.ShowItemInFolder(path)
		return nil
	}
}

func (rt *liveApp) queryMCP(ctx context.Context, provider store.Provider) (mcp.Snapshot, error) {
	if !rt.features.Enabled(string(provider)) {
		return mcp.Snapshot{}, errMCPUnavailable
	}
	switch provider {
	case store.ProviderCodex:
		if rt.codex != nil {
			return rt.codex.LoadMCP(ctx)
		}
	case store.ProviderClaude:
		if rt.claudeMod != nil && rt.claudeMod.Store != nil {
			return rt.claudeMod.Store.LoadMCP(ctx)
		}
	case store.ProviderGrok:
		if rt.grokMod != nil && rt.grokMod.Store != nil {
			return rt.grokMod.Store.LoadMCP(ctx)
		}
	default:
		return mcp.Snapshot{}, errors.New("unsupported MCP provider")
	}
	return mcp.Snapshot{}, errMCPUnavailable
}

func (rt *liveApp) deleteMCP(ctx context.Context, provider store.Provider, selected []mcp.Server) ([]mcp.DeleteResult, error) {
	if !rt.features.Enabled(string(provider)) {
		return nil, errMCPUnavailable
	}
	now := time.Now()
	switch provider {
	case store.ProviderCodex:
		if rt.codex != nil {
			return rt.codex.DeleteMCP(ctx, selected, now)
		}
	case store.ProviderClaude:
		if rt.claudeMod != nil && rt.claudeMod.Store != nil {
			return rt.claudeMod.Store.DeleteMCP(ctx, selected, now)
		}
	case store.ProviderGrok:
		if rt.grokMod != nil && rt.grokMod.Store != nil {
			return rt.grokMod.Store.DeleteMCP(ctx, selected, now)
		}
	default:
		return nil, errors.New("unsupported MCP provider")
	}
	return nil, errMCPUnavailable
}
