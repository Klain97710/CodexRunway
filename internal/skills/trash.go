package skills

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/licoy/quosparr/internal/platform"
)

var (
	ErrTrashConflict   = errors.New("skills: something already exists at the original path")
	ErrTrashParentGone = errors.New("skills: the original folder no longer exists")
	ErrTrashEntry      = errors.New("skills: trash entry is missing or damaged")
	renameFile         = os.Rename
	isCrossDevice      = crossDevice
	trashMu            sync.Mutex
	// maxTrashWalk caps the entries Size visits; a var so tests stay fast.
	maxTrashWalk = 20000
)

const maxTrashMeta = 64 << 10

// Trash holds restorable skills under the app-owned directory.
type Trash struct{ Dir, Home string }

type trashMetadata struct {
	Version int `json:"version"`
	TrashEntry
}

// NewTrash locates skill trash under the app-owned directory.
func NewTrash(runwayDir, home string) Trash {
	return Trash{Dir: filepath.Join(runwayDir, "skills-trash"), Home: home}
}

// MoveSkill revalidates the scanned removal before moving it into private trash.
func (t Trash) MoveSkill(skill Skill, roots []RootInfo, now time.Time, system func(string) error) (TrashEntry, bool, error) {
	trashMu.Lock()
	defer trashMu.Unlock()
	if system == nil {
		return TrashEntry{}, false, ErrNotDeletable
	}
	skill, err := validateSkillRemoval(skill, roots)
	if err != nil {
		return TrashEntry{}, false, err
	}
	entry := TrashEntry{
		Kind: TrashSkill, Provider: skill.Provider, SharedWith: skill.SharedWith,
		Name: skill.Name, InvokeName: skill.InvokeName, DisplayName: skill.DisplayName,
		Description: skill.Description, Covers: skill.Removal.Covers,
		Original: skill.Removal.Path, Root: skill.Root, IsLink: skill.Removal.IsLink,
		LinkTarget: skill.LinkTarget, Origin: skill.Origin, DeletedAt: now.UTC(),
	}
	return t.move(entry, system)
}

// MoveBrokenLink moves only the dangling link reported by the scan.
func (t Trash) MoveBrokenLink(summary Summary, path string, now time.Time, system func(string) error) (TrashEntry, bool, error) {
	trashMu.Lock()
	defer trashMu.Unlock()
	if system == nil {
		return TrashEntry{}, false, ErrNotDeletable
	}
	path, err := validateBrokenLink(summary, path)
	if err != nil {
		return TrashEntry{}, false, err
	}
	target, err := os.Readlink(path)
	if err != nil {
		return TrashEntry{}, false, changed(err)
	}
	root := filepath.Dir(path)
	for _, candidate := range summary.Roots {
		if candidate.Scope == ScopeUser && within(candidate.Path, path) && filepath.Clean(candidate.Path) != path {
			root = candidate.Path
			break
		}
	}
	entry := TrashEntry{Kind: TrashBrokenLink, Provider: summary.Provider,
		Name: filepath.Base(path), Original: path, Root: root,
		IsLink: true, LinkTarget: target, DeletedAt: now.UTC()}
	return t.move(entry, system)
}

func (t Trash) move(entry TrashEntry, system func(string) error) (TrashEntry, bool, error) {
	entry.OriginalLabel = entry.Original
	if t.Home != "" {
		entry.OriginalLabel = rootLabel(t.Home, entry.Original)
	}
	entry, err := t.prepare(entry)
	if err != nil {
		return TrashEntry{}, false, fmt.Errorf("skills: move to trash: %w", err)
	}
	if err := renameFile(entry.Original, entry.Item); err != nil {
		if cleanupErr := cleanupTrashEntry(filepath.Join(t.Dir, entry.ID)); cleanupErr != nil {
			return TrashEntry{}, false, fmt.Errorf("skills: move to trash: %w", errors.Join(err, cleanupErr))
		}
		if !isCrossDevice(err) {
			return TrashEntry{}, false, fmt.Errorf("skills: move to trash: %w", err)
		}
		entry.ID, entry.Item = "", ""
		if err := system(entry.Original); err != nil {
			return entry, false, fmt.Errorf("skills: move to trash: %w", err)
		}
		return entry, true, nil
	}
	return entry, false, nil
}

func (t Trash) prepare(entry TrashEntry) (_ TrashEntry, err error) {
	if err := platform.EnsurePrivateDirectory(t.Dir); err != nil {
		return TrashEntry{}, err
	}
	var random [4]byte
	if _, err := rand.Read(random[:]); err != nil {
		return TrashEntry{}, err
	}
	entry.ID = entry.DeletedAt.Format("20060102T150405Z") + "-" + hex.EncodeToString(random[:])
	dir := filepath.Join(t.Dir, entry.ID)
	if _, err := os.Lstat(dir); err == nil {
		return TrashEntry{}, fs.ErrExist
	} else if !errors.Is(err, fs.ErrNotExist) {
		return TrashEntry{}, err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, cleanupTrashEntry(dir))
		}
	}()
	for _, path := range []string{dir, filepath.Join(dir, "item")} {
		if err := platform.EnsurePrivateDirectory(path); err != nil {
			return TrashEntry{}, err
		}
	}
	entry.Size = trashSize(entry.Original)
	data, err := json.Marshal(trashMetadata{Version: 1, TrashEntry: entry})
	if err != nil {
		return TrashEntry{}, err
	}
	if len(data) > maxTrashMeta {
		return TrashEntry{}, ErrTrashEntry
	}
	if err := platform.WriteFileAtomic(filepath.Join(dir, "meta.json"), data, 0o600); err != nil {
		return TrashEntry{}, err
	}
	entry.Item = filepath.Join(dir, "item", filepath.Base(entry.Original))
	return entry, nil
}

func trashSize(path string) int64 {
	var size int64
	count := 0
	err := filepath.WalkDir(path, func(_ string, entry fs.DirEntry, err error) error {
		count++
		if err != nil {
			return err
		}
		if count > maxTrashWalk {
			return fs.ErrInvalid
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			size += info.Size()
		}
		return nil
	})
	if err != nil {
		return -1
	}
	return size
}

// List skips damaged entries and derives storage paths from their directories.
func (t Trash) List(ctx context.Context) ([]TrashEntry, error) {
	trashMu.Lock()
	defer trashMu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	dirs, err := os.ReadDir(t.Dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var entries []TrashEntry
	for _, dir := range dirs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !dir.IsDir() || !validTrashID(dir.Name()) {
			continue
		}
		if entry, err := t.readEntry(dir.Name()); err == nil {
			entries = append(entries, entry)
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].DeletedAt.Equal(entries[j].DeletedAt) {
			return entries[i].ID > entries[j].ID
		}
		return entries[i].DeletedAt.After(entries[j].DeletedAt)
	})
	return entries, nil
}

func validTrashID(id string) bool {
	return len(id) > 0 && len(id) <= 128 && !strings.ContainsFunc(id, func(r rune) bool {
		return !(r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r == '-')
	})
}

func (t Trash) entryDir(id string) (string, error) {
	if !validTrashID(id) {
		return "", ErrTrashEntry
	}
	dir := filepath.Join(t.Dir, id)
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() {
		return "", ErrTrashEntry
	}
	return dir, nil
}

func (t Trash) readEntry(id string) (TrashEntry, error) {
	dir, err := t.entryDir(id)
	if err != nil {
		return TrashEntry{}, err
	}
	meta := filepath.Join(dir, "meta.json")
	info, err := os.Lstat(meta)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxTrashMeta {
		return TrashEntry{}, ErrTrashEntry
	}
	file, err := os.Open(meta)
	if err != nil {
		return TrashEntry{}, ErrTrashEntry
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxTrashMeta+1))
	var stored trashMetadata
	if err != nil || len(data) > maxTrashMeta || json.Unmarshal(data, &stored) != nil || stored.Version != 1 {
		return TrashEntry{}, ErrTrashEntry
	}
	entry := stored.TrashEntry
	entry.Original = filepath.Clean(entry.Original)
	rel, err := filepath.Rel(entry.Root, entry.Original)
	if err != nil || !filepath.IsAbs(entry.Original) || !filepath.IsAbs(entry.Root) || !within(entry.Root, entry.Original) || rel == "." {
		return TrashEntry{}, ErrTrashEntry
	}
	itemDir := filepath.Join(dir, "item")
	info, err = os.Lstat(itemDir)
	if err != nil || !info.IsDir() {
		return TrashEntry{}, ErrTrashEntry
	}
	entry.ID = id
	entry.Item = filepath.Join(itemDir, filepath.Base(entry.Original))
	if _, err := os.Lstat(entry.Item); err != nil {
		return TrashEntry{}, ErrTrashEntry
	}
	return entry, nil
}

// Restore returns an item to its original location without changing CLI config.
func (t Trash) Restore(id string) (TrashEntry, error) {
	trashMu.Lock()
	defer trashMu.Unlock()
	entry, err := t.readEntry(id)
	if err != nil {
		return TrashEntry{}, err
	}
	_, originalErr := os.Lstat(entry.Original)
	if originalErr == nil {
		return TrashEntry{}, ErrTrashConflict
	}
	parent, err := os.Stat(filepath.Dir(entry.Original))
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) || err == nil && !parent.IsDir() {
		return TrashEntry{}, ErrTrashParentGone
	}
	if err != nil {
		return TrashEntry{}, err
	}
	if !errors.Is(originalErr, fs.ErrNotExist) {
		return TrashEntry{}, originalErr
	}
	if err := validateRemovalRoot(entry.Original, []RootInfo{{Path: entry.Root, Scope: ScopeUser, Exists: true}}); err != nil {
		return TrashEntry{}, ErrTrashEntry
	}
	if err := renameFile(entry.Item, entry.Original); err != nil {
		return TrashEntry{}, fmt.Errorf("skills: restore from trash: %w", err)
	}
	// Cleanup is best effort after the user's item has been restored.
	_ = cleanupTrashEntry(filepath.Join(t.Dir, id))
	return entry, nil
}

func cleanupTrashEntry(dir string) error {
	var errs []error
	for _, path := range []string{filepath.Join(dir, "meta.json"), filepath.Join(dir, "item"), dir} {
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Purge hands the entire entry directory to the system trash.
func (t Trash) Purge(id string, system func(string) error) error {
	trashMu.Lock()
	defer trashMu.Unlock()
	dir, err := t.entryDir(id)
	if err != nil {
		return err
	}
	if system == nil {
		return ErrNotDeletable
	}
	return system(dir)
}

// Empty continues past individual failures and counts successful handoffs.
func (t Trash) Empty(system func(string) error) (int, error) {
	trashMu.Lock()
	defer trashMu.Unlock()
	dirs, err := os.ReadDir(t.Dir)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if system == nil {
		return 0, ErrNotDeletable
	}
	count := 0
	var errs []error
	for _, dir := range dirs {
		if !dir.IsDir() || !validTrashID(dir.Name()) {
			continue
		}
		if err := system(filepath.Join(t.Dir, dir.Name())); err != nil {
			errs = append(errs, err)
		} else {
			count++
		}
	}
	return count, errors.Join(errs...)
}
