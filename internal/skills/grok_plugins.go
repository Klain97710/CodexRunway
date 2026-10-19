package skills

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
)

var errGrokRegistry = errors.New("skills: invalid Grok plugin registry")

type grokPluginRegistry struct {
	Version int                       `json:"version"`
	Repos   map[string]grokPluginRepo `json:"repos"`
}

type grokPluginRepo struct {
	Kind        grokPluginKind                 `json:"kind"`
	InstalledAt string                         `json:"installed_at"`
	UpdatedAt   string                         `json:"updated_at"`
	Path        string                         `json:"path"`
	Marketplace grokPluginMarketplace          `json:"marketplace"`
	Plugins     map[string]grokInstalledPlugin `json:"plugins"`
}

type grokPluginKind struct {
	Type   string `json:"type"`
	URL    string `json:"url"`
	GitRef string `json:"git_ref"`
	Commit string `json:"commit"`
}

type grokPluginMarketplace struct {
	SourceDisplayName string `json:"source_display_name"`
}

type grokInstalledPlugin struct {
	Subdir  string `json:"subdir"`
	Version string `json:"version"`
}

func scanGrokPlugins(ctx context.Context, env Env, config grokConfig, summary *Summary) error {
	base := filepath.Join(env.GrokHome, "installed-plugins")
	data, err := os.ReadFile(filepath.Join(base, "registry.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var registry grokPluginRegistry
	if err := json.Unmarshal(data, &registry); err != nil {
		return errGrokRegistry
	}
	for _, repoID := range grokSortedRepoKeys(registry.Repos) {
		repo := registry.Repos[repoID]
		for _, pluginName := range grokSortedPluginKeys(repo.Plugins) {
			if err := scanGrokPlugin(ctx, env, config, summary, base, repoID, repo, pluginName); err != nil {
				return err
			}
		}
	}
	return nil
}

func scanGrokPlugin(ctx context.Context, env Env, config grokConfig, summary *Summary, base, repoID string, repo grokPluginRepo, pluginName string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	repoPath := repo.Path
	if !filepath.IsAbs(repoPath) {
		repoPath = filepath.Join(base, repoPath)
	}
	root := newRoot(env, filepath.Join(repoPath, repo.Plugins[pluginName].Subdir, "skills"), ScopePlugin)
	summary.Roots = append(summary.Roots, root)
	found, issues, err := scanGrokDirect(ctx, env, root, nil)
	if err != nil {
		return err
	}
	on := grokPluginEnabled(config, repoID, pluginName)
	for _, skill := range found {
		skill.Plugin = pluginName
		skill.InvokeName = pluginName + ":" + skill.Name
		skill.Deletable = false
		skill.Toggleable = false
		skill.ToggleBlock = ReasonPluginCollection
		if !on && skill.State != StateNotLoaded {
			skill.State, skill.Reason = StateOff, ReasonPluginOff
		}
		summary.Skills = append(summary.Skills, skill)
	}
	summary.Issues = append(summary.Issues, issues...)
	return nil
}

func grokPluginEnabled(config grokConfig, repoID, name string) bool {
	if stringInSlice(config.Plugins.Disabled, repoID) || stringInSlice(config.Plugins.Disabled, name) {
		return false
	}
	return stringInSlice(config.Plugins.Enabled, repoID) || stringInSlice(config.Plugins.Enabled, name)
}

func grokSortedRepoKeys(values map[string]grokPluginRepo) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func grokSortedPluginKeys(values map[string]grokInstalledPlugin) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
