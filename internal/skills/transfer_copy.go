package skills

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

func resolveFuturePath(path string) (string, error) {
	path = filepath.Clean(path)
	var suffix []string
	for {
		if _, err := os.Lstat(path); err == nil {
			resolved, err := filepath.EvalSymlinks(path)
			if err != nil {
				return "", err
			}
			for i := len(suffix) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, suffix[i])
			}
			return filepath.Clean(resolved), nil
		} else if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(path)
		if parent == path {
			return "", fs.ErrNotExist
		}
		suffix = append(suffix, filepath.Base(path))
		path = parent
	}
}

func pathsOverlap(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	return within(a, b) || within(b, a)
}

func ensureTransferRoot(path string) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("skills: create transfer root: %w", err)
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("skills: invalid transfer root: %w", ErrTransferOverlap)
	}
	return nil
}

type transferTarget struct {
	root    *os.Root
	name    string
	created []transferCreated
}

type transferCreated struct {
	path string
	info fs.FileInfo
}

func (t *transferTarget) cleanup() error {
	var errs []error
	for i := len(t.created) - 1; i >= 0; i-- {
		created := t.created[i]
		current, err := t.root.Lstat(created.path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil || !os.SameFile(created.info, current) {
			errs = append(errs, fmt.Errorf("skills: target changed during cleanup: %s", created.path))
			continue
		}
		if err := t.root.Remove(created.path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func copySkill(source transferSource, targetRoot, name string) (*transferTarget, error) {
	target, err := os.OpenRoot(targetRoot)
	if err != nil {
		return nil, err
	}
	if err := target.Mkdir(name, 0o755); err != nil {
		_ = target.Close()
		if errors.Is(err, fs.ErrExist) {
			return nil, ErrTransferConflict
		}
		return nil, fmt.Errorf("skills: create target skill: %w", err)
	}
	rootInfo, err := target.Lstat(name)
	if err != nil {
		_ = target.Close()
		return nil, err
	}
	created := &transferTarget{root: target, name: name, created: []transferCreated{{path: name, info: rootInfo}}}
	destination, err := target.OpenRoot(name)
	if err == nil {
		err = copySkillTree(source.root, destination, created)
		_ = destination.Close()
	}
	if err != nil {
		err = errors.Join(err, created.cleanup())
		_ = target.Close()
		return nil, err
	}
	return created, nil
}

func copySkillTree(source, target *os.Root, created *transferTarget) error {
	err := fs.WalkDir(source.FS(), ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || path == "." {
			return walkErr
		}
		if path == "SKILL.md" {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		return copySkillEntry(source, target, created, path, info)
	})
	if err != nil {
		return err
	}
	info, err := source.Lstat("SKILL.md")
	if err != nil {
		return fmt.Errorf("%w: SKILL.md", ErrTransferSource)
	}
	return copySkillEntry(source, target, created, "SKILL.md", info)
}

func copySkillEntry(source, target *os.Root, created *transferTarget, path string, info fs.FileInfo) error {
	var err error
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		err = copySkillLink(source, target, path)
	case info.IsDir():
		err = target.Mkdir(path, info.Mode().Perm()|0o700)
	case info.Mode().IsRegular():
		createdInfo, fileErr := copySkillFile(source, target, path, info.Mode().Perm())
		if createdInfo != nil {
			created.created = append(created.created, transferCreated{path: filepath.Join(created.name, path), info: createdInfo})
		}
		return fileErr
	default:
		return fmt.Errorf("%w: %s", ErrTransferSource, path)
	}
	if err == nil {
		err = recordTransferCreated(target, created, path)
	}
	return err
}

func recordTransferCreated(target *os.Root, created *transferTarget, path string) error {
	info, err := target.Lstat(path)
	if err != nil {
		return err
	}
	created.created = append(created.created, transferCreated{path: filepath.Join(created.name, path), info: info})
	return nil
}

func copySkillLink(source, target *os.Root, path string) error {
	link, err := source.Readlink(path)
	if err != nil || filepath.IsAbs(link) {
		return fmt.Errorf("%w: %s", ErrTransferUnsafeLink, path)
	}
	if _, err := source.Stat(path); err != nil {
		return fmt.Errorf("%w: %s", ErrTransferUnsafeLink, path)
	}
	if err := target.Symlink(link, path); err != nil {
		return fmt.Errorf("skills: copy link %s: %w", path, err)
	}
	return nil
}

func copySkillFile(source, target *os.Root, path string, mode fs.FileMode) (fs.FileInfo, error) {
	in, err := source.Open(path)
	if err != nil {
		return nil, err
	}
	out, err := target.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		_ = in.Close()
		return nil, err
	}
	createdInfo, statErr := out.Stat()
	if statErr != nil {
		return nil, errors.Join(statErr, out.Close(), in.Close())
	}
	_, copyErr := io.Copy(out, in)
	return createdInfo, errors.Join(copyErr, out.Close(), in.Close())
}
