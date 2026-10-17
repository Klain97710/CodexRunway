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

var (
	errGrokManifest = errors.New("skills: invalid Grok bundled manifest")
)

type grokManifest struct {
	Checksums map[string]string `json:"checksums"`
}

type grokRoot struct {
	info      RootInfo
	compatOff bool
}

// ScanGrok reports the user, compatibility, bundled, and installed-plugin
// skills visible to Grok. Project-local skills are intentionally out of scope.
func ScanGrok(ctx context.Context, env Env) (Summary, error) {
	if err := ctx.Err(); err != nil {
		return Summary{}, err
	}
	env, err := normalizeEnv(env)
	if err != nil {
		return Summary{}, err
	}
	config, err := readGrokConfig(filepath.Join(env.GrokHome, "config.toml"))
	if err != nil {
		return Summary{}, err
	}
	summary := Summary{Provider: ProviderGrok, ScannedAt: env.Now}
	if err := scanGrokRoots(ctx, env, grokUserRoots(env, config), &summary); err != nil {
		return Summary{}, err
	}
	if err := scanGrokBundled(ctx, env, &summary); err != nil {
		return Summary{}, err
	}
	if err := scanGrokPlugins(ctx, env, config, &summary); err != nil {
		return Summary{}, err
	}
	applyGrokConfig(env.Home, config, summary.Skills)
	shadowGrokSkills(summary.Skills)
	finishSummary(&summary)
	return summary, nil
}

func grokUserRoots(env Env, config grokConfig) []grokRoot {
	claudeOn := grokCompatEnabled(env.Getenv("GROK_CLAUDE_SKILLS_ENABLED"), config.Compat.Claude.Skills)
	cursorOn := grokCompatEnabled(env.Getenv("GROK_CURSOR_SKILLS_ENABLED"), config.Compat.Cursor.Skills)
	roots := []grokRoot{
		{info: newRoot(env, filepath.Join(env.GrokHome, "skills"), ScopeUser)},
		{info: newRoot(env, filepath.Join(env.Home, ".agents", "skills"), ScopeUser)},
		{info: newRoot(env, filepath.Join(env.Home, ".claude", "skills"), ScopeUser), compatOff: !claudeOn},
		{info: newRoot(env, filepath.Join(env.Home, ".cursor", "skills"), ScopeUser), compatOff: !cursorOn},
	}
	for _, path := range config.Skills.Paths {
		roots = append(roots, grokRoot{info: newRoot(env, expandGrokPath(env.Home, path), ScopeUser)})
	}
	return roots
}

func scanGrokRoots(ctx context.Context, env Env, roots []grokRoot, summary *Summary) error {
	options := walkOptions{MaxDepth: 6, MaxDirs: 2000, MaxEntries: 20000, SkipHidden: true, FollowLinks: true}
	for _, root := range roots {
		if err := ctx.Err(); err != nil {
			return err
		}
		summary.Roots = append(summary.Roots, root.info)
		found, issues, err := scanRoot(ctx, env, ProviderGrok, root.info, options)
		if err != nil {
			return err
		}
		for i := range found {
			found[i].Toggleable = true
			if root.compatOff && found[i].State != StateNotLoaded {
				found[i].State, found[i].Reason = StateOff, ReasonCompatOff
			}
		}
		sortGrokSkills(found)
		summary.Skills = append(summary.Skills, found...)
		summary.Issues = append(summary.Issues, issues...)
	}
	return nil
}

func scanGrokBundled(ctx context.Context, env Env, summary *Summary) error {
	root := newRoot(env, filepath.Join(env.GrokHome, "bundled", "skills"), ScopeBundled)
	summary.Roots = append(summary.Roots, root)
	manifest, present, err := readGrokManifest(filepath.Join(env.GrokHome, "bundled", "manifest.json"))
	if err != nil {
		return err
	}
	allowed := func(name string) bool { return !present || manifestHasSkill(manifest, name) }
	found, issues, err := scanGrokDirect(ctx, env, root, allowed)
	if err != nil {
		return err
	}
	for _, skill := range found {
		skill.Deletable = false
		skill.Toggleable = true
		skill.ToggleBlock = ReasonNone
		summary.Skills = append(summary.Skills, skill)
	}
	summary.Issues = append(summary.Issues, issues...)
	return nil
}

func readGrokManifest(path string) (grokManifest, bool, error) {
	var manifest grokManifest
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return manifest, false, nil
	}
	if err != nil {
		return manifest, false, err
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return manifest, true, errGrokManifest
	}
	return manifest, true, nil
}

func manifestHasSkill(manifest grokManifest, name string) bool {
	prefix := "skills/" + name + "/"
	for path := range manifest.Checksums {
		path = filepath.ToSlash(path)
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func readDirIfPresent(path string) ([]os.DirEntry, error) {
	entries, err := os.ReadDir(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return entries, err
}

func scanGrokDirect(ctx context.Context, env Env, root RootInfo, allowed func(string) bool) ([]Skill, []Issue, error) {
	entries, err := readDirIfPresent(root.Path)
	if err != nil {
		return nil, nil, err
	}
	var found []Skill
	var issues []Issue
	dirs := 0
	for index, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		if index >= 20000 {
			issues = append(issues, Issue{Kind: IssueTruncated, Path: root.Path})
			break
		}
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if allowed != nil && !allowed(entry.Name()) {
			continue
		}
		if dirs >= 2000 {
			issues = append(issues, Issue{Kind: IssueTruncated, Path: root.Path})
			break
		}
		dirs++
		file := filepath.Join(root.Path, entry.Name(), "SKILL.md")
		info, err := os.Stat(file)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, nil, err
		}
		if !info.Mode().IsRegular() {
			continue
		}
		skill, skillIssues, err := loadSkill(env, ProviderGrok, root, file)
		if err != nil {
			return nil, nil, err
		}
		found = append(found, skill)
		issues = append(issues, skillIssues...)
	}
	sortGrokSkills(found)
	return found, issues, nil
}

func shadowGrokSkills(skills []Skill) {
	winners := make(map[string]string)
	for i := range skills {
		if skills[i].Scope == ScopePlugin || skills[i].State != StateOn {
			continue
		}
		if winner, ok := winners[skills[i].Name]; ok {
			skills[i].State = StateOverridden
			skills[i].Reason = ReasonShadowed
			skills[i].OverriddenBy = winner
			continue
		}
		winners[skills[i].Name] = skills[i].File
	}
}

func sortGrokSkills(skills []Skill) {
	sort.SliceStable(skills, func(i, j int) bool {
		if skills[i].InvokeName != skills[j].InvokeName {
			return skills[i].InvokeName < skills[j].InvokeName
		}
		return skills[i].File < skills[j].File
	})
}
