package skills

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type walkOptions struct {
	MaxDepth       int
	MaxDirs        int
	MaxEntries     int
	SkipHidden     bool
	FollowLinks    bool
	DirectChildren bool
	SkipPaths      []string
}

func (o walkOptions) normalized() walkOptions {
	if o.MaxDepth <= 0 {
		o.MaxDepth = 6
	}
	if o.MaxDirs <= 0 {
		o.MaxDirs = 2000
	}
	if o.MaxEntries <= 0 {
		o.MaxEntries = 20000
	}
	o.SkipPaths = append([]string(nil), o.SkipPaths...)
	for i := range o.SkipPaths {
		o.SkipPaths[i] = filepath.Clean(o.SkipPaths[i])
	}
	return o
}

type rootWalker struct {
	ctx       context.Context
	env       Env
	provider  Provider
	root      RootInfo
	opts      walkOptions
	dirs      int
	entries   int
	truncated bool
	seen      map[string]bool
	skills    []Skill
	issues    []Issue
}

type walkItem struct {
	path       string
	depth      int
	firstLink  string
	linkTarget string
}

func scanRoot(ctx context.Context, env Env, provider Provider, root RootInfo, opts walkOptions) ([]Skill, []Issue, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	w := rootWalker{ctx: ctx, env: env, provider: provider, root: root, opts: opts.normalized(), seen: make(map[string]bool)}
	if w.isSkipped(root.Path) {
		return nil, nil, nil
	}
	linkInfo, err := os.Lstat(root.Path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	if linkInfo.Mode()&os.ModeSymlink != 0 {
		if err := w.walkLink(root.Path, -1, "", ""); err != nil {
			return nil, nil, err
		}
		return w.skills, w.issues, nil
	}
	info := linkInfo
	if info.Mode().IsRegular() {
		if w.opts.DirectChildren {
			return nil, nil, nil
		}
		if filepath.Base(root.Path) != "SKILL.md" {
			return nil, nil, nil
		}
		skill, issues, err := loadSkill(env, provider, root, root.Path)
		if err != nil {
			return nil, nil, err
		}
		return []Skill{skill}, issues, nil
	}
	if !info.IsDir() {
		return nil, nil, nil
	}
	if w.opts.DirectChildren {
		if err := w.walkDirectRoot(root.Path, "", ""); err != nil {
			return nil, nil, err
		}
		return w.skills, w.issues, nil
	}
	if err := w.walk(root.Path, 0, "", ""); err != nil {
		return nil, nil, err
	}
	return w.skills, w.issues, nil
}

func (w *rootWalker) walk(path string, depth int, firstLink, linkTarget string) error {
	if err := w.ctx.Err(); err != nil {
		return err
	}
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	real, err = filepath.Abs(real)
	if err != nil {
		return err
	}
	if w.seen[real] {
		return nil
	}
	w.seen[real] = true
	if w.dirs >= w.opts.MaxDirs {
		w.addTruncated(path)
		return nil
	}
	w.dirs++
	entries, err := w.readDir(path)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := w.ctx.Err(); err != nil {
			return err
		}
		item := walkItem{path: filepath.Join(path, entry.Name()), depth: depth, firstLink: firstLink, linkTarget: linkTarget}
		if err := w.walkEntry(entry, item); err != nil {
			return err
		}
	}
	return nil
}

func (w *rootWalker) readDir(path string) ([]os.DirEntry, error) {
	remaining := w.opts.MaxEntries - w.entries
	if remaining <= 0 {
		w.addTruncated(path)
		return nil, nil
	}
	dir, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	entries, readErr := dir.ReadDir(remaining + 1)
	closeErr := dir.Close()
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if len(entries) > remaining {
		entries = entries[:remaining]
		w.addTruncated(path)
	}
	w.entries += len(entries)
	return entries, nil
}

func (w *rootWalker) walkEntry(entry os.DirEntry, item walkItem) error {
	if w.isSkipped(item.path) {
		return nil
	}
	if w.opts.SkipHidden && strings.HasPrefix(entry.Name(), ".") {
		return nil
	}
	info, err := os.Lstat(item.path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return w.walkLink(item.path, item.depth, item.firstLink, item.linkTarget)
	}
	if info.IsDir() {
		if item.depth >= w.opts.MaxDepth {
			w.addTruncated(item.path)
			return nil
		}
		return w.walk(item.path, item.depth+1, item.firstLink, item.linkTarget)
	}
	if entry.Name() == "SKILL.md" && info.Mode().IsRegular() {
		return w.addSkill(item.path, item.firstLink, item.linkTarget)
	}
	return nil
}

func (w *rootWalker) isSkipped(path string) bool {
	path = filepath.Clean(path)
	for _, skipped := range w.opts.SkipPaths {
		if within(skipped, path) {
			return true
		}
	}
	return false
}

func (w *rootWalker) walkLink(path string, depth int, firstLink, linkTarget string) error {
	target, err := os.Readlink(path)
	if err != nil {
		return err
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(path), target)
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return err
	}
	target = filepath.Clean(target)
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			if w.root.Scope == ScopeUser {
				w.issues = append(w.issues, Issue{Kind: IssueBrokenLink, Path: path, Name: filepath.Base(path), Detail: target, Removable: true})
			}
			return nil
		}
		return err
	}
	if firstLink == "" {
		firstLink, linkTarget = path, target
	}
	if info.IsDir() {
		if !w.opts.FollowLinks {
			return nil
		}
		if w.opts.DirectChildren {
			return w.walkDirectRoot(path, firstLink, linkTarget)
		}
		if depth >= w.opts.MaxDepth {
			w.addTruncated(path)
			return nil
		}
		return w.walk(path, depth+1, firstLink, linkTarget)
	}
	if filepath.Base(path) == "SKILL.md" && info.Mode().IsRegular() {
		return w.addSkill(path, firstLink, linkTarget)
	}
	return nil
}

func (w *rootWalker) addSkill(file, linkPath, linkTarget string) error {
	skill, issues, err := loadSkill(w.env, w.provider, w.root, file)
	if err != nil {
		return err
	}
	if linkPath != "" {
		skill.LinkPath = linkPath
		skill.LinkTarget = linkTarget
	}
	w.skills = append(w.skills, skill)
	w.issues = append(w.issues, issues...)
	return nil
}

func (w *rootWalker) addTruncated(path string) {
	if !w.truncated {
		w.issues = append(w.issues, Issue{Kind: IssueTruncated, Path: path})
		w.truncated = true
	}
}
