package skills

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type claudeInstalledPlugins struct {
	Version int                                `json:"version"`
	Plugins map[string][]claudeInstalledPlugin `json:"plugins"`
}

type claudeInstalledPlugin struct {
	Scope       string `json:"scope"`
	InstallPath string `json:"installPath"`
	Version     string `json:"version"`
}

func readClaudePluginManifest(path string) (pluginManifest, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return pluginManifest{}, false
	}
	var manifest pluginManifest
	if json.Unmarshal(data, &manifest) != nil {
		return pluginManifest{}, false
	}
	return manifest, true
}

func loadClaudeCollection(ctx context.Context, env Env, root RootInfo, collectionRoot string, manifest pluginManifest) ([]Skill, []Issue, error) {
	if manifest.Skills == nil {
		return loadClaudeDirectChildren(ctx, env, root, filepath.Join(collectionRoot, "skills"))
	}
	var paths []string
	if json.Unmarshal(manifest.Skills, &paths) != nil {
		return nil, nil, nil
	}
	var skills []Skill
	var issues []Issue
	if len(paths) > 20000 {
		paths = paths[:20000]
		issues = append(issues, Issue{Kind: IssueTruncated, Path: collectionRoot})
	}
	dirs := 0
	for _, declared := range paths {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		path := filepath.Clean(filepath.Join(collectionRoot, filepath.FromSlash(declared)))
		rel, err := filepath.Rel(collectionRoot, path)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			continue
		}
		info, err := os.Stat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, nil, err
		}
		if info.IsDir() {
			if dirs >= 2000 {
				issues = append(issues, Issue{Kind: IssueTruncated, Path: collectionRoot})
				break
			}
			dirs++
			found, foundIssues, err := loadClaudeDeclaredDirectory(ctx, env, root, path)
			if err != nil {
				return nil, nil, err
			}
			skills = append(skills, found...)
			issues = append(issues, foundIssues...)
			continue
		}
		if filepath.Base(path) != "SKILL.md" {
			continue
		}
		skill, skillIssues, err := loadSkill(env, ProviderClaude, root, path)
		if err != nil {
			return nil, nil, err
		}
		skills = append(skills, skill)
		issues = append(issues, skillIssues...)
	}
	return skills, issues, nil
}

func loadClaudeDeclaredDirectory(ctx context.Context, env Env, root RootInfo, dir string) ([]Skill, []Issue, error) {
	file := filepath.Join(dir, "SKILL.md")
	if info, err := os.Stat(file); err == nil && info.Mode().IsRegular() {
		skill, issues, err := loadSkill(env, ProviderClaude, root, file)
		if err != nil {
			return nil, nil, err
		}
		return []Skill{skill}, issues, nil
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, nil, err
	}
	return loadClaudeDirectChildren(ctx, env, root, dir)
}

func loadClaudeDirectChildren(ctx context.Context, env Env, root RootInfo, dir string) ([]Skill, []Issue, error) {
	entries, truncated, err := claudeReadDir(dir, 20000)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	var skills []Skill
	var issues []Issue
	if truncated {
		issues = append(issues, Issue{Kind: IssueTruncated, Path: dir})
	}
	dirs := 0
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		info, err := os.Stat(filepath.Join(dir, entry.Name()))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, nil, err
		}
		if !info.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if dirs >= 2000 {
			issues = append(issues, Issue{Kind: IssueTruncated, Path: dir})
			break
		}
		dirs++
		skill, skillIssues, err := loadSkill(env, ProviderClaude, root, filepath.Join(dir, entry.Name(), "SKILL.md"))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, nil, err
		}
		skills = append(skills, skill)
		issues = append(issues, skillIssues...)
	}
	return skills, issues, nil
}

func claudeReadDir(path string, limit int) ([]os.DirEntry, bool, error) {
	dir, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer dir.Close()
	entries, err := dir.ReadDir(limit + 1)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, false, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	if len(entries) <= limit {
		return entries, false, nil
	}
	return entries[:limit], true, nil
}

func scanClaudeInstalled(ctx context.Context, env Env, settings claudeSettings) ([]Skill, []RootInfo, []Issue, error) {
	data, err := os.ReadFile(filepath.Join(env.ClaudeDir, "plugins", "installed_plugins.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil, nil, nil
	}
	if err != nil {
		return nil, nil, nil, err
	}
	var installed claudeInstalledPlugins
	if json.Unmarshal(data, &installed) != nil || installed.Version != 2 {
		return nil, nil, nil, nil
	}
	keys := make([]string, 0, len(installed.Plugins))
	for key := range installed.Plugins {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var skills []Skill
	var roots []RootInfo
	var issues []Issue
	for _, pluginID := range keys {
		pluginName := strings.SplitN(pluginID, "@", 2)[0]
		for _, install := range installed.Plugins[pluginID] {
			if install.Scope != "user" || strings.TrimSpace(install.InstallPath) == "" {
				continue
			}
			if err := ctx.Err(); err != nil {
				return nil, nil, nil, err
			}
			root := newRoot(env, filepath.Join(install.InstallPath, "skills"), ScopePlugin)
			roots = append(roots, root)
			found, foundIssues, err := scanRoot(ctx, env, ProviderClaude, root, walkOptions{MaxDirs: 2000, MaxEntries: 20000, SkipHidden: true, DirectChildren: true})
			if err != nil {
				return nil, nil, nil, err
			}
			for i := range found {
				found[i].Plugin = pluginName
				found[i].InvokeName = pluginName + ":" + found[i].Name
				found[i].Deletable = false
				found[i].Toggleable = false
				found[i].ToggleBlock = ReasonPluginCollection
				if enabled, ok := settings.EnabledPlugins[pluginID]; isClaudeLoadable(found[i]) && ok && !enabled {
					found[i].State, found[i].Reason = StateOff, ReasonPluginOff
				}
			}
			skills = append(skills, found...)
			issues = append(issues, foundIssues...)
		}
	}
	return skills, roots, issues, nil
}
