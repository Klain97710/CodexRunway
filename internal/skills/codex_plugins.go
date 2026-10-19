package skills

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type pluginManifest struct {
	Name   string          `json:"name"`
	Skills json.RawMessage `json:"skills"`
}

func applyCodexCollection(skill *Skill, root RootInfo) {
	manifestRoot, name := nearestPluginManifest(skill.RealFile, root.Path)
	if name == "" {
		manifestRoot, name = nearestPluginManifest(skill.File, root.Path)
	}
	if name == "" {
		return
	}
	skill.Plugin = name
	skill.InvokeName = name + ":" + skill.Name
	if skill.Scope == ScopeUser {
		skill.Removal.Path = manifestRoot
		skill.Removal.IsLink = false
	}
}

func nearestPluginManifest(file, stop string) (string, string) {
	if file == "" {
		return "", ""
	}
	stop = filepath.Clean(stop)
	for dir := filepath.Dir(file); ; dir = filepath.Dir(dir) {
		for _, rel := range []string{".codex-plugin/plugin.json", ".claude-plugin/plugin.json", ".cursor-plugin/plugin.json"} {
			if name := readPluginName(filepath.Join(dir, filepath.FromSlash(rel))); name != "" {
				return dir, name
			}
		}
		if dir == stop || filepath.Dir(dir) == dir {
			return "", ""
		}
	}
}

func readPluginName(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var manifest pluginManifest
	if json.Unmarshal(data, &manifest) != nil {
		return ""
	}
	if name := strings.TrimSpace(manifest.Name); name != "" {
		return name
	}
	return filepath.Base(filepath.Dir(filepath.Dir(path)))
}

func scanCodexPlugins(ctx context.Context, env Env, config codexConfig) ([]Skill, []RootInfo, []Issue, error) {
	cache := filepath.Join(env.CodexHome, "plugins", "cache")
	var skills []Skill
	var roots []RootInfo
	var issues []Issue
	for _, pluginID := range codexPluginIDs(config) {
		parts := strings.SplitN(pluginID, "@", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			continue
		}
		if err := ctx.Err(); err != nil {
			return nil, nil, nil, err
		}
		base := filepath.Join(cache, parts[1], parts[0])
		version, ok, err := newestVersion(base)
		if errors.Is(err, os.ErrNotExist) {
			roots = append(roots, newRoot(env, base, ScopePlugin))
			continue
		}
		if err != nil {
			return nil, nil, nil, err
		}
		if !ok {
			roots = append(roots, newRoot(env, base, ScopePlugin))
			continue
		}
		root := newRoot(env, filepath.Join(version, "skills"), ScopePlugin)
		roots = append(roots, root)
		found, foundIssues, err := scanRoot(ctx, env, ProviderCodex, root, walkOptions{MaxDirs: 2000, MaxEntries: 20000, SkipHidden: true, DirectChildren: true})
		if err != nil {
			return nil, nil, nil, err
		}
		for i := range found {
			found[i].Plugin = parts[0]
			found[i].InvokeName = parts[0] + ":" + found[i].Name
			found[i].Deletable = false
			found[i].Toggleable = false
			found[i].ToggleBlock = ReasonPluginCollection
			if isCodexLoadable(found[i]) && !*config.Plugins[pluginID].Enabled {
				found[i].State, found[i].Reason = StateOff, ReasonPluginOff
			}
		}
		skills = append(skills, found...)
		issues = append(issues, foundIssues...)
	}
	return skills, roots, issues, nil
}

func codexPluginIDs(config codexConfig) []string {
	ids := make([]string, 0, len(config.Plugins))
	for id, setting := range config.Plugins {
		if setting.Enabled != nil {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

func newestVersion(root string) (string, bool, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", false, err
	}
	var newest string
	var newestMod int64
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return "", false, err
		}
		path := filepath.Join(root, entry.Name())
		if newest == "" || info.ModTime().UnixNano() > newestMod || info.ModTime().UnixNano() == newestMod && path > newest {
			newest, newestMod = path, info.ModTime().UnixNano()
		}
	}
	return newest, newest != "", nil
}
