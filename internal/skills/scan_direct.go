package skills

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func (w *rootWalker) walkDirectRoot(path, firstLink, linkTarget string) error {
	if w.isSkipped(path) {
		return nil
	}
	entered, err := w.enterDirectDir(path)
	if err != nil || !entered {
		return err
	}
	entries, err := w.readDir(path)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := w.ctx.Err(); err != nil {
			return err
		}
		if w.opts.SkipHidden && strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		child := filepath.Join(path, entry.Name())
		if w.isSkipped(child) {
			continue
		}
		info, err := os.Lstat(child)
		if err != nil {
			return err
		}
		if info.IsDir() {
			if err := w.walkDirectChild(child, firstLink, linkTarget); err != nil {
				return err
			}
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			if err := w.walkDirectLink(child, firstLink, linkTarget); err != nil {
				return err
			}
		}
	}
	return nil
}

func (w *rootWalker) walkDirectLink(path, firstLink, linkTarget string) error {
	if w.isSkipped(path) {
		return nil
	}
	target, err := absoluteLinkTarget(path)
	if err != nil {
		return err
	}
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
	if !info.IsDir() || !w.opts.FollowLinks {
		return nil
	}
	if firstLink == "" {
		firstLink, linkTarget = path, target
	}
	return w.walkDirectChild(path, firstLink, linkTarget)
}

func (w *rootWalker) walkDirectChild(path, firstLink, linkTarget string) error {
	if w.isSkipped(path) {
		return nil
	}
	entered, err := w.enterDirectDir(path)
	if err != nil || !entered {
		return err
	}
	entries, err := w.readDir(path)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := w.ctx.Err(); err != nil {
			return err
		}
		if entry.Name() != "SKILL.md" {
			continue
		}
		return w.walkDirectSkill(filepath.Join(path, entry.Name()), firstLink, linkTarget)
	}
	return nil
}

func (w *rootWalker) walkDirectSkill(path, firstLink, linkTarget string) error {
	if w.isSkipped(path) {
		return nil
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode().IsRegular() {
		return w.addSkill(path, firstLink, linkTarget)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return nil
	}
	target, err := absoluteLinkTarget(path)
	if err != nil {
		return err
	}
	targetInfo, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) && w.root.Scope == ScopeUser {
			w.issues = append(w.issues, Issue{Kind: IssueBrokenLink, Path: path, Name: "SKILL.md", Detail: target, Removable: true})
			return nil
		}
		return err
	}
	if !targetInfo.Mode().IsRegular() {
		return nil
	}
	if firstLink == "" {
		firstLink, linkTarget = path, target
	}
	return w.addSkill(path, firstLink, linkTarget)
}

func (w *rootWalker) enterDirectDir(path string) (bool, error) {
	if err := w.ctx.Err(); err != nil {
		return false, err
	}
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false, err
	}
	if w.seen[filepath.Clean(real)] {
		return false, nil
	}
	if w.dirs >= w.opts.MaxDirs {
		w.addTruncated(path)
		return false, nil
	}
	w.seen[filepath.Clean(real)] = true
	w.dirs++
	return true, nil
}

func absoluteLinkTarget(path string) (string, error) {
	target, err := os.Readlink(path)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(path), target)
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return "", err
	}
	return filepath.Clean(target), nil
}
