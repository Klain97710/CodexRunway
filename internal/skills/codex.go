package skills

import (
	"context"
	"path/filepath"
	"sort"
	"unicode/utf8"
)

var codexAdminRoot = "/etc/codex/skills"

// ScanCodex reports the global skills visible to Codex.
func ScanCodex(ctx context.Context, input Env) (Summary, error) {
	if err := ctx.Err(); err != nil {
		return Summary{}, err
	}
	env, err := normalizeEnv(input)
	if err != nil {
		return Summary{}, err
	}
	summary := Summary{Provider: ProviderCodex, ScannedAt: env.Now}
	opts := walkOptions{MaxDepth: 6, MaxDirs: 2000, MaxEntries: 20000, SkipHidden: true, FollowLinks: true}

	roots := []RootInfo{
		newRoot(env, filepath.Join(env.CodexHome, "skills"), ScopeUser),
		newRoot(env, filepath.Join(env.Home, ".agents", "skills"), ScopeUser),
		newRoot(env, filepath.Join(env.CodexHome, "skills", ".system"), ScopeSystem),
	}
	if env.GOOS != "windows" {
		roots = append(roots, newRoot(env, codexAdminRoot, ScopeAdmin))
	}

	for _, root := range roots {
		if err := ctx.Err(); err != nil {
			return Summary{}, err
		}
		rootOpts := opts
		if root.Scope == ScopeSystem {
			rootOpts.FollowLinks = false
		}
		skills, issues, err := scanRoot(ctx, env, ProviderCodex, root, rootOpts)
		if err != nil {
			return Summary{}, err
		}
		for n := range skills {
			applyCodexCollection(&skills[n], root)
			setCodexDefaults(&skills[n])
		}
		summary.Roots = append(summary.Roots, root)
		summary.Skills = append(summary.Skills, skills...)
		summary.Issues = append(summary.Issues, issues...)
	}

	config := readCodexConfig(filepath.Join(env.CodexHome, "config.toml"))
	pluginSkills, pluginRoots, pluginIssues, err := scanCodexPlugins(ctx, env, config)
	if err != nil {
		return Summary{}, err
	}
	summary.Roots = append(summary.Roots, pluginRoots...)
	summary.Skills = append(summary.Skills, pluginSkills...)
	summary.Issues = append(summary.Issues, pluginIssues...)
	summary.Skills = dedupeSkillsByRealFile(summary.Skills)
	applyCodexConfig(summary.Skills, config)
	addCodexNameIssues(&summary)
	sortCodexSkills(summary.Skills)
	newOriginScanner(env).fill(summary.Skills)
	finishSummary(&summary)
	return summary, nil
}

func setCodexDefaults(skill *Skill) {
	skill.Toggleable = skill.Scope != ScopePlugin
	if skill.Toggleable {
		skill.ToggleBlock = ReasonNone
	} else {
		skill.ToggleBlock = ReasonPluginCollection
	}
}

func addCodexNameIssues(summary *Summary) {
	for _, skill := range summary.Skills {
		if utf8.RuneCountInString(skill.Name) > 64 || utf8.RuneCountInString(skill.InvokeName) > 129 {
			summary.Issues = append(summary.Issues, Issue{Kind: IssueNameTooLong, Path: skill.File, Name: skill.InvokeName})
		}
	}
}

func sortCodexSkills(skills []Skill) {
	priority := map[Scope]int{ScopeUser: 0, ScopeSystem: 1, ScopeAdmin: 2, ScopePlugin: 3}
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
