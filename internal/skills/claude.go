package skills

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var claudeManagedRootFn = claudeManagedRoot

func claudeManagedRoot(goos string) string {
	switch goos {
	case "darwin":
		return "/Library/Application Support/ClaudeCode/.claude/skills"
	case "windows":
		return `C:\Program Files\ClaudeCode\.claude\skills`
	default:
		return "/etc/claude-code/.claude/skills"
	}
}

// ScanClaude reports the global skills visible to Claude Code.
func ScanClaude(ctx context.Context, input Env) (Summary, error) {
	if err := ctx.Err(); err != nil {
		return Summary{}, err
	}
	env, err := normalizeEnv(input)
	if err != nil {
		return Summary{}, err
	}
	settings := readClaudeSettings(filepath.Join(env.ClaudeDir, "settings.json"))
	summary := Summary{Provider: ProviderClaude, ScannedAt: env.Now}
	userRoot := newRoot(env, filepath.Join(env.ClaudeDir, "skills"), ScopeUser)
	syncedRoot := newRoot(env, filepath.Join(env.ClaudeDir, "skills", "synced"), ScopeSynced)
	adminRoot := claudeManagedRootInfo(env)
	summary.Roots = append(summary.Roots, userRoot, syncedRoot, adminRoot)

	userSkills, userIssues, err := scanClaudeDirectory(ctx, env, userRoot, true, settings)
	if err != nil {
		return Summary{}, err
	}
	syncedSkills, syncedIssues, err := scanClaudeDirectory(ctx, env, syncedRoot, false, settings)
	if err != nil {
		return Summary{}, err
	}
	adminSkills, adminIssues, err := scanClaudeDirectory(ctx, env, adminRoot, false, settings)
	if err != nil {
		return Summary{}, err
	}
	summary.Skills = append(summary.Skills, userSkills...)
	summary.Skills = append(summary.Skills, syncedSkills...)
	summary.Skills = append(summary.Skills, adminSkills...)
	summary.Issues = append(summary.Issues, userIssues...)
	summary.Issues = append(summary.Issues, syncedIssues...)
	summary.Issues = append(summary.Issues, adminIssues...)

	installedSkills, installedRoots, installedIssues, err := scanClaudeInstalled(ctx, env, settings)
	if err != nil {
		return Summary{}, err
	}
	summary.Skills = append(summary.Skills, installedSkills...)
	summary.Roots = append(summary.Roots, installedRoots...)
	summary.Issues = append(summary.Issues, installedIssues...)
	sortClaudeSkills(summary.Skills)
	summary.Skills = dedupeSkillsByRealFile(summary.Skills)
	applyClaudeOverrides(summary.Skills, settings)
	applyClaudeAdminShadow(summary.Skills)
	finishSummary(&summary)
	return summary, nil
}

func scanClaudeDirectory(ctx context.Context, env Env, root RootInfo, pluginCollections bool, settings claudeSettings) ([]Skill, []Issue, error) {
	collections, collectionRoots, collectionIssues, err := findClaudeCollections(ctx, root, pluginCollections)
	if err != nil {
		return nil, nil, err
	}
	skipPaths := append([]string(nil), collectionRoots...)
	if root.Scope == ScopeUser {
		skipPaths = append(skipPaths, filepath.Join(root.Path, "synced"))
	}
	found, issues, err := scanRoot(ctx, env, ProviderClaude, root, walkOptions{
		MaxDepth: 6, MaxDirs: 2000, MaxEntries: 20000,
		SkipHidden: true, FollowLinks: true, SkipPaths: skipPaths,
	})
	if err != nil {
		return nil, nil, err
	}
	issues = append(issues, collectionIssues...)
	for i := range found {
		setClaudeDefaults(&found[i], root.Scope)
		if !isDirectSkill(root.Path, found[i].File) {
			found[i].State = StateNotLoaded
			found[i].Reason = ReasonNested
			issues = append(issues, Issue{Kind: IssueNested, Path: found[i].File, Name: found[i].InvokeName})
		}
	}
	pluginSkills, pluginIssues, err := loadClaudeCollections(ctx, env, root, collections, collectionRoots, settings)
	if err != nil {
		return nil, nil, err
	}
	found = append(found, pluginSkills...)
	issues = append(issues, pluginIssues...)
	return found, issues, nil
}

func findClaudeCollections(ctx context.Context, root RootInfo, enabled bool) (map[string]pluginManifest, []string, []Issue, error) {
	collections := make(map[string]pluginManifest)
	if !enabled || !root.Exists {
		return collections, nil, nil, nil
	}
	entries, truncated, err := claudeReadDir(root.Path, 20000)
	if err != nil {
		return nil, nil, nil, err
	}
	var issues []Issue
	if truncated {
		issues = append(issues, Issue{Kind: IssueTruncated, Path: root.Path})
	}
	dirs := 0
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, nil, nil, err
		}
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		info, err := os.Stat(filepath.Join(root.Path, entry.Name()))
		if os.IsNotExist(err) || err == nil && !info.IsDir() {
			continue
		}
		if err != nil {
			return nil, nil, nil, err
		}
		if dirs >= 2000 {
			issues = append(issues, Issue{Kind: IssueTruncated, Path: root.Path})
			break
		}
		dirs++
		collectionRoot := filepath.Join(root.Path, entry.Name())
		if manifest, ok := readClaudePluginManifest(filepath.Join(collectionRoot, ".claude-plugin", "plugin.json")); ok {
			if strings.TrimSpace(manifest.Name) == "" {
				manifest.Name = entry.Name()
			}
			collections[collectionRoot] = manifest
		}
	}
	roots := make([]string, 0, len(collections))
	for root := range collections {
		roots = append(roots, root)
	}
	sort.Strings(roots)
	return collections, roots, issues, nil
}

func loadClaudeCollections(ctx context.Context, env Env, root RootInfo, collections map[string]pluginManifest, roots []string, settings claudeSettings) ([]Skill, []Issue, error) {
	var skills []Skill
	var issues []Issue
	for _, collectionRoot := range roots {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		manifest := collections[collectionRoot]
		found, foundIssues, err := loadClaudeCollection(ctx, env, root, collectionRoot, manifest)
		if err != nil {
			return nil, nil, err
		}
		for i := range found {
			found[i].Plugin = manifest.Name
			found[i].InvokeName = manifest.Name + ":" + found[i].Name
			found[i].Scope = ScopeUser
			found[i].Toggleable = false
			found[i].ToggleBlock = ReasonPluginCollection
			if enabled, ok := settings.EnabledPlugins[manifest.Name+"@skills-dir"]; isClaudeLoadable(found[i]) && ok && !enabled {
				found[i].State, found[i].Reason = StateOff, ReasonPluginOff
			}
		}
		skills = append(skills, found...)
		issues = append(issues, foundIssues...)
	}
	return skills, issues, nil
}

func claudeManagedRootInfo(env Env) RootInfo {
	path := claudeManagedRootFn(env.GOOS)
	if env.GOOS == "windows" && !filepath.IsAbs(path) {
		return RootInfo{Path: path, Label: path, Scope: ScopeAdmin}
	}
	return newRoot(env, path, ScopeAdmin)
}

func setClaudeDefaults(skill *Skill, scope Scope) {
	skill.Scope = scope
	switch scope {
	case ScopeAdmin:
		skill.Deletable = false
		skill.Toggleable = false
		skill.ToggleBlock = ReasonReadOnlyScope
	case ScopeSynced:
		skill.Deletable = false
		skill.Toggleable = true
	default:
		skill.Toggleable = true
	}
}

func isDirectSkill(root, file string) bool {
	rel, err := filepath.Rel(root, file)
	if err != nil || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return false
	}
	parts := strings.Split(filepath.Clean(rel), string(os.PathSeparator))
	return len(parts) == 2 && parts[1] == "SKILL.md"
}

func dedupeSkillsByRealFile(skills []Skill) []Skill {
	seen := make(map[string]bool)
	out := skills[:0]
	for _, skill := range skills {
		key := skill.RealFile
		if key == "" {
			key = skill.File
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, skill)
	}
	return out
}

func sortClaudeSkills(skills []Skill) {
	priority := map[Scope]int{ScopeAdmin: 0, ScopeUser: 1, ScopeSynced: 2, ScopePlugin: 3}
	sort.SliceStable(skills, func(i, j int) bool {
		if priority[skills[i].Scope] != priority[skills[j].Scope] {
			return priority[skills[i].Scope] < priority[skills[j].Scope]
		}
		if skills[i].InvokeName != skills[j].InvokeName {
			return skills[i].InvokeName < skills[j].InvokeName
		}
		return skills[i].File < skills[j].File
	})
}
