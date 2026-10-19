package skills

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
)

var detailFileLimit = MaxDetailFiles
var detailWalkLimit = MaxDetailWalk
var detailDepthLimit = MaxDetailDepth

var detailSkippedDirs = map[string]bool{
	".git": true, ".hg": true, ".svn": true, "node_modules": true,
	"__pycache__": true, ".venv": true, "venv": true,
	".mypy_cache": true, ".pytest_cache": true, ".tox": true,
}

var errNotSkillDir = errors.New("skills: skill directory is not a directory")

// Inspect lists the files under a skill's folder without following links.
func Inspect(ctx context.Context, skill Skill) (Detail, error) {
	if err := ctx.Err(); err != nil {
		return Detail{}, err
	}
	real, err := filepath.EvalSymlinks(skill.Dir)
	if err != nil {
		return Detail{}, err
	}
	real, err = filepath.Abs(real)
	if err != nil {
		return Detail{}, err
	}
	info, err := os.Lstat(real)
	if err != nil {
		return Detail{}, err
	}
	if !info.IsDir() {
		return Detail{}, errNotSkillDir
	}
	w := detailWalker{ctx: ctx, root: filepath.Clean(real)}
	if err := w.walkDir(w.root, 0, true); err != nil {
		return Detail{}, err
	}
	if err := ctx.Err(); err != nil {
		return Detail{}, err
	}
	sort.Slice(w.detail.Files, func(i, j int) bool { return w.detail.Files[i].Path < w.detail.Files[j].Path })
	limit := max(detailFileLimit, 0)
	if len(w.detail.Files) > limit {
		w.detail.Files = w.detail.Files[:limit]
		w.detail.Truncated = true
	}
	return w.detail, nil
}

type detailWalker struct {
	ctx    context.Context
	root   string
	walked int
	detail Detail
}

func (w *detailWalker) walkDir(dir string, depth int, root bool) error {
	if err := w.ctx.Err(); err != nil {
		return err
	}
	handle, err := os.Open(dir)
	if err != nil {
		if ctxErr := w.ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if root {
			return err
		}
		return nil
	}
	defer handle.Close()
	for {
		if err := w.ctx.Err(); err != nil {
			return err
		}
		entries, err := handle.ReadDir(1)
		if err != nil {
			if ctxErr := w.ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			if errors.Is(err, io.EOF) {
				return nil
			}
			if root {
				return err
			}
			return nil
		}
		if len(entries) == 0 {
			return nil
		}
		path := filepath.Join(dir, entries[0].Name())
		info, err := os.Lstat(path)
		if err != nil || info.Mode().IsRegular() && entries[0].Name() == ".DS_Store" {
			continue
		}
		if w.walked >= max(detailWalkLimit, 0) {
			w.detail.Truncated = true
			return nil
		}
		w.walked++
		if err := w.visit(path, info, depth+1); err != nil {
			return err
		}
	}
}

func (w *detailWalker) visit(path string, info os.FileInfo, depth int) error {
	name := filepath.Base(path)
	rel, err := filepath.Rel(w.root, path)
	if err != nil {
		return nil
	}
	entry := FileEntry{Path: filepath.ToSlash(rel), ModTime: info.ModTime()}
	if info.Mode()&os.ModeSymlink != 0 {
		entry.Link = true
		w.detail.Files = append(w.detail.Files, entry)
		return nil
	}
	if !info.IsDir() {
		if info.Mode().IsRegular() {
			entry.Size = info.Size()
		}
		w.detail.Files = append(w.detail.Files, entry)
		if info.Mode().IsRegular() {
			w.detail.FileCount++
			w.detail.Size += info.Size()
		}
		return nil
	}
	entry.Dir = true
	entry.Skipped = detailSkippedDirs[name]
	w.detail.DirCount++
	w.detail.Files = append(w.detail.Files, entry)
	if entry.Skipped {
		return nil
	}
	if depth >= max(detailDepthLimit, 0) {
		w.detail.Truncated = true
		return nil
	}
	return w.walkDir(path, depth, false)
}
