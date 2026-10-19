package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/egoist/mygo"
	"github.com/licoy/quosparr/internal/platform"
	"github.com/licoy/quosparr/internal/skills"
	"github.com/licoy/quosparr/internal/store"
)

var errSkillsUnavailable = errors.New("skills service unavailable")

// installSkillActions wires the skills pages to the scanner, the provider
// config writers, the Quosparr skills trash and the system shell. Deleting
// moves to that trash (the system Trash from another volume); purging
// moves on to the system Trash, so nothing is erased. The diagnostics
// fixes go through the same trash and back SKILL.md up before editing it.
func installSkillActions(rt *liveApp, a *SkillActions) {
	a.Query = rt.querySkills
	home := rt.skillsEnv().Home
	if home == "" {
		return
	}
	trash := skills.NewTrash(platform.RunwayDir(home), home)
	system := mygo.Shell.TrashItem
	a.ReadBody = func(s skills.Skill) (string, bool, error) {
		if !rt.features.Enabled(string(s.Provider)) {
			return "", false, errSkillsUnavailable
		}
		return skills.ReadBody(s)
	}
	a.SetEnabled = rt.setSkillEnabled
	a.Delete = func(s skills.Skill, roots []skills.RootInfo) (bool, error) {
		if !rt.features.Enabled(string(s.Provider)) {
			return false, errSkillsUnavailable
		}
		_, inSystem, err := trash.MoveSkill(s, roots, time.Now(), system)
		return inSystem, err
	}
	a.DeleteLink = func(sum skills.Summary, path string) (bool, error) {
		if !rt.features.Enabled(string(sum.Provider)) {
			return false, errSkillsUnavailable
		}
		_, inSystem, err := trash.MoveBrokenLink(sum, path, time.Now(), system)
		return inSystem, err
	}
	a.Transfer = func(req skills.TransferRequest) (skills.TransferResult, error) {
		if !rt.features.Enabled(string(req.Skill.Provider)) || !rt.features.Enabled(string(req.Target)) {
			return skills.TransferResult{}, errSkillsUnavailable
		}
		return skills.TransferSkill(rt.skillsEnv(), req, system)
	}
	a.LiftNested = func(sum skills.Summary, s skills.Skill) (skills.LiftResult, error) {
		if !rt.features.Enabled(string(sum.Provider)) || !rt.features.Enabled(string(s.Provider)) {
			return skills.LiftResult{}, errSkillsUnavailable
		}
		return trash.LiftNested(sum, s, time.Now(), system)
	}
	a.SuggestDescription = func(s skills.Skill) string {
		if !rt.features.Enabled(string(s.Provider)) {
			return ""
		}
		return skills.SuggestDescription(s)
	}
	backup := platform.BackupPolicy{Dir: filepath.Join(platform.RunwayDir(home), "backups"), Prefix: "skill-md-", Ext: ".md", Keep: 10}
	a.AddDescription = func(s skills.Skill, description string) error {
		if !rt.features.Enabled(string(s.Provider)) {
			return errSkillsUnavailable
		}
		return skills.AddDescription(s, description, backup, time.Now())
	}
	a.OpenFolder = mygo.Shell.OpenPath
	a.Reveal = mygo.Shell.ShowItemInFolder
	a.Copy = mygo.Clipboard.WriteText
	a.Inspect = func(ctx context.Context, s skills.Skill) (skills.Detail, error) {
		if !rt.features.Enabled(string(s.Provider)) {
			return skills.Detail{}, errSkillsUnavailable
		}
		return skills.Inspect(ctx, s)
	}
	a.TrashDir = trash.Dir
	a.ListTrash = func(ctx context.Context) ([]skills.TrashEntry, error) {
		entries, err := trash.List(ctx)
		filtered := make([]skills.TrashEntry, 0, len(entries))
		for _, entry := range entries {
			if rt.features.Enabled(string(entry.Provider)) {
				filtered = append(filtered, entry)
			}
		}
		return filtered, err
	}
	allowedTrash := func(id string) bool {
		entries, err := a.ListTrash(context.Background())
		if err != nil {
			return false
		}
		for _, entry := range entries {
			if entry.ID == id {
				return true
			}
		}
		return false
	}
	a.Restore = func(id string) (skills.TrashEntry, error) {
		if !allowedTrash(id) {
			return skills.TrashEntry{}, errSkillsUnavailable
		}
		return trash.Restore(id)
	}
	a.Purge = func(id string) error {
		if !allowedTrash(id) {
			return errSkillsUnavailable
		}
		return trash.Purge(id, system)
	}
	a.EmptyTrash = func() (int, error) {
		entries, err := a.ListTrash(context.Background())
		if err != nil {
			return 0, err
		}
		for i, entry := range entries {
			if err := trash.Purge(entry.ID, system); err != nil {
				return i, err
			}
		}
		return len(entries), nil
	}
}

func (rt *liveApp) skillsEnv() skills.Env {
	env := skills.Env{
		Home:      rt.home,
		CodexHome: rt.codexFolder(),
		ClaudeDir: rt.claudeFolder(),
		GrokHome:  rt.grokFolder(),
		GOOS:      runtime.GOOS,
		Getenv:    os.Getenv,
		Now:       time.Now(),
	}
	return env
}

// querySkills scans enabled providers only, using the explicitly supplied home.
func (rt *liveApp) querySkills(ctx context.Context) (map[store.Provider]skills.Summary, error) {
	if rt.home == "" || rt.codex == nil {
		return nil, errSkillsUnavailable
	}
	env := rt.skillsEnv()
	cx, err := skills.ScanCodex(ctx, env)
	if err != nil {
		return nil, err
	}
	summaries := []*skills.Summary{&cx}
	if rt.features.Claude {
		cl, err := skills.ScanClaude(ctx, env)
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, &cl)
	}
	if rt.features.Grok {
		gk, err := skills.ScanGrok(ctx, env)
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, &gk)
	}
	skills.MarkShared(summaries...)
	result := make(map[store.Provider]skills.Summary, len(summaries))
	for _, sum := range summaries {
		result[store.Provider(sum.Provider)] = *sum
	}
	return result, nil
}

// setSkillEnabled writes the provider's own switch: Codex [[skills.config]],
// Claude skillOverrides, Grok [skills].disabled; each backs up first.
func (rt *liveApp) setSkillEnabled(s skills.Skill, on bool) error {
	if !rt.features.Enabled(string(s.Provider)) {
		return errSkillsUnavailable
	}
	now := time.Now()
	switch s.Provider {
	case skills.ProviderCodex:
		if rt.codex == nil {
			return errSkillsUnavailable
		}
		return rt.codex.SetSkillEnabledFile(s.File, s.InvokeName, on, now)
	case skills.ProviderClaude:
		if rt.claudeMod == nil || rt.claudeMod.Store == nil {
			return errSkillsUnavailable
		}
		return rt.claudeMod.Store.SetSkillEnabledFile(s.Name, on, now)
	case skills.ProviderGrok:
		if rt.grokMod == nil || rt.grokMod.Store == nil {
			return errSkillsUnavailable
		}
		return rt.grokMod.Store.SetSkillEnabledFile(s.Name, on, now)
	}
	return errSkillsUnavailable
}
