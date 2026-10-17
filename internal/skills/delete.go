package skills

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

var (
	ErrNotDeletable = errors.New("skills: item is not deletable")
	ErrChanged      = errors.New("skills: item changed since scan")
	ErrOutsideRoot  = errors.New("skills: item is outside a user root")
)

func setRemovals(summary *Summary) {
	for i := range summary.Skills {
		skill := &summary.Skills[i]
		if skill.Scope != ScopeUser {
			continue
		}
		path, isLink := removalPath(*skill)
		if path == "" || !isLink && filepath.Clean(path) == filepath.Clean(skill.Root) {
			continue
		}
		skill.Deletable = true
		skill.Removal = Removal{Path: path, IsLink: isLink}
	}
	for i := range summary.Skills {
		skill := &summary.Skills[i]
		if !skill.Deletable {
			continue
		}
		seen := make(map[string]bool)
		for _, candidate := range summary.Skills {
			if removalCovers(skill.Removal, candidate) && !seen[candidate.InvokeName] {
				skill.Removal.Covers = append(skill.Removal.Covers, candidate.InvokeName)
				seen[candidate.InvokeName] = true
			}
		}
	}
}

func removalPath(skill Skill) (string, bool) {
	if skill.LinkPath != "" {
		return filepath.Clean(skill.LinkPath), true
	}
	root := filepath.Clean(skill.Root)
	if root == filepath.Clean(skill.File) || filepath.Base(root) == "SKILL.md" {
		return "", false
	}
	candidate := ""
	for dir := filepath.Clean(skill.Dir); dir != root && within(root, dir); dir = filepath.Dir(dir) {
		if isCollection(dir) {
			candidate = dir
		}
		if filepath.Dir(dir) == dir {
			break
		}
	}
	if candidate != "" {
		return candidate, false
	}
	return filepath.Clean(skill.Dir), false
}

func isCollection(dir string) bool {
	for _, marker := range []string{filepath.Join(".claude-plugin", "plugin.json"), filepath.Join(".codex-plugin", "plugin.json"), ".git"} {
		if _, err := os.Lstat(filepath.Join(dir, marker)); err == nil {
			return true
		}
	}
	return false
}

func removalCovers(removal Removal, skill Skill) bool {
	if removal.IsLink {
		return skill.LinkPath == removal.Path || within(removal.Path, skill.File)
	}
	return within(removal.Path, skill.File)
}

// Delete moves a previously scanned user-owned skill to trash.
func Delete(skill Skill, roots []RootInfo, trash func(path string) error) error {
	if !skill.Deletable || skill.Scope != ScopeUser || trash == nil {
		return ErrNotDeletable
	}
	skill.Removal.Path = filepath.Clean(skill.Removal.Path)
	skill.File = filepath.Clean(skill.File)
	if !removalCovers(skill.Removal, skill) {
		return ErrOutsideRoot
	}
	info, err := os.Lstat(skill.Removal.Path)
	if err != nil {
		return changed(err)
	}
	if (info.Mode()&os.ModeSymlink != 0) != skill.Removal.IsLink {
		return ErrChanged
	}
	if !skill.Removal.IsLink {
		if err := validateRemovalRoot(skill.Removal.Path, roots); err != nil {
			return err
		}
		for _, root := range roots {
			if root.Scope == ScopeUser && root.Exists && sameResolved(root.Path, skill.Removal.Path) {
				return ErrOutsideRoot
			}
		}
	}
	fileInfo, err := os.Stat(skill.File)
	if err != nil {
		if !(skill.Removal.IsLink && errors.Is(err, fs.ErrNotExist)) {
			return changed(err)
		}
	} else if !fileInfo.ModTime().Equal(skill.ModTime) {
		return ErrChanged
	}
	if err := trash(skill.Removal.Path); err != nil {
		return fmt.Errorf("skills: move to trash: %w", err)
	}
	return nil
}

// DeleteBrokenLink moves a dangling user-root link reported by a scan to trash.
func DeleteBrokenLink(summary Summary, path string, trash func(path string) error) error {
	path = filepath.Clean(path)
	if trash == nil || !slices.ContainsFunc(summary.Issues, func(issue Issue) bool {
		return issue.Kind == IssueBrokenLink && issue.Removable && filepath.Clean(issue.Path) == filepath.Clean(path)
	}) {
		return ErrNotDeletable
	}
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		return changed(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		return ErrChanged
	}
	if err := trash(path); err != nil {
		return fmt.Errorf("skills: move broken link to trash: %w", err)
	}
	return nil
}

func validateRemovalRoot(path string, roots []RootInfo) error {
	path = filepath.Clean(path)
	parent, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		return ErrOutsideRoot
	}
	for _, root := range roots {
		if root.Scope != ScopeUser || !root.Exists || !within(root.Path, path) || filepath.Clean(root.Path) == path {
			continue
		}
		realRoot, err := filepath.EvalSymlinks(root.Path)
		if err == nil && within(realRoot, parent) {
			return nil
		}
	}
	return ErrOutsideRoot
}

func sameResolved(a, b string) bool {
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	return errA == nil && errB == nil && filepath.Clean(ra) == filepath.Clean(rb)
}

func within(parent, child string) bool {
	rel, err := filepath.Rel(filepath.Clean(parent), filepath.Clean(child))
	return err == nil && !filepath.IsAbs(rel) && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func changed(err error) error {
	if err == nil {
		return ErrChanged
	}
	return fmt.Errorf("%w: %v", ErrChanged, err)
}
