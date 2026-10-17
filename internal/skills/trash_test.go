package skills

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"
)

const trashTestBody = "---\nname: one\ndescription: example\n---\nbody\n"

func trashTestSkill(t *testing.T, kind string) (Trash, Skill, []RootInfo) {
	t.Helper()
	home := t.TempDir()
	root := filepath.Join(home, "skills")
	dir := filepath.Join(root, "one")
	link, target := "", ""
	if kind == "collection" {
		dir = filepath.Join(root, "collection", "one")
		commonWriteFile(t, filepath.Join(root, "collection", ".git", "HEAD"), "ref")
	}
	if kind == "link" {
		trashTestRequireLinks(t)
		target = filepath.Join(home, "target")
		commonWriteFile(t, filepath.Join(target, "SKILL.md"), trashTestBody)
		if err := os.MkdirAll(root, 0o700); err != nil {
			t.Fatal(err)
		}
		link = dir
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
	} else {
		commonWriteFile(t, filepath.Join(dir, "SKILL.md"), trashTestBody)
	}
	file := filepath.Join(dir, "SKILL.md")
	info, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	skill := Skill{Provider: ProviderCodex, Scope: ScopeUser, Root: root,
		Name: "one", InvokeName: "one", DisplayName: "One", Description: "example",
		SharedWith: []Provider{ProviderClaude}, Dir: dir, File: file, ModTime: info.ModTime(),
		LinkPath: link, LinkTarget: target, Origin: Origin{From: OriginGit, Repository: "https://example.com/repo"}}
	summary := Summary{Skills: []Skill{skill}}
	setRemovals(&summary)
	return NewTrash(filepath.Join(home, ".quosparr"), home), summary.Skills[0], []RootInfo{{Path: root, Scope: ScopeUser, Exists: true}}
}

func trashTestRequireLinks(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires privileges on Windows")
	}
}

func trashTestNoSystem(t *testing.T) func(string) error {
	t.Helper()
	return func(path string) error { t.Fatalf("unexpected system trash call: %s", path); return nil }
}

func trashTestMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected missing %s, got %v", path, err)
	}
}

func trashTestMove(t *testing.T, trash Trash, skill Skill, roots []RootInfo, now time.Time) TrashEntry {
	t.Helper()
	entry, system, err := trash.MoveSkill(skill, roots, now, trashTestNoSystem(t))
	if err != nil || system {
		t.Fatalf("move = %#v, %v, %v", entry, system, err)
	}
	return entry
}

func TestTrashMoveSkill(t *testing.T) {
	for _, kind := range []string{"directory", "collection", "link"} {
		t.Run(kind, func(t *testing.T) {
			trash, skill, roots := trashTestSkill(t, kind)
			now := time.Date(2026, 10, 17, 14, 15, 16, 0, time.FixedZone("local", 8*3600))
			entry := trashTestMove(t, trash, skill, roots, now)
			trashTestMissing(t, skill.Removal.Path)
			if !regexp.MustCompile(`^20261017T061516Z-[0-9a-f]{8}$`).MatchString(entry.ID) {
				t.Fatalf("id = %q", entry.ID)
			}
			wantSize := int64(len(trashTestBody))
			if kind == "collection" {
				wantSize += 3
			}
			if kind == "link" {
				wantSize = 0
			}
			want := TrashEntry{ID: entry.ID, Item: filepath.Join(trash.Dir, entry.ID, "item", filepath.Base(skill.Removal.Path)),
				Kind: TrashSkill, Provider: skill.Provider, SharedWith: skill.SharedWith,
				Name: skill.Name, InvokeName: skill.InvokeName, DisplayName: skill.DisplayName,
				Description: skill.Description, Covers: skill.Removal.Covers, Original: skill.Removal.Path,
				OriginalLabel: filepath.Join("~", "skills", filepath.Base(skill.Removal.Path)), Root: skill.Root,
				IsLink: skill.Removal.IsLink, LinkTarget: skill.LinkTarget, Origin: skill.Origin,
				DeletedAt: now.UTC(), Size: wantSize}
			if !reflect.DeepEqual(entry, want) {
				t.Fatalf("entry = %#v, want %#v", entry, want)
			}
			trashTestMovedContent(t, entry, kind)
			trashTestMetadata(t, trash, entry)
		})
	}
}

func trashTestMovedContent(t *testing.T, entry TrashEntry, kind string) {
	t.Helper()
	file := filepath.Join(entry.Item, "SKILL.md")
	if kind == "collection" {
		file = filepath.Join(entry.Item, "one", "SKILL.md")
		if data, err := os.ReadFile(filepath.Join(entry.Item, ".git", "HEAD")); err != nil || string(data) != "ref" {
			t.Fatalf("collection git marker = %q, %v", data, err)
		}
	}
	if data, err := os.ReadFile(file); err != nil || string(data) != trashTestBody {
		t.Fatalf("moved content = %q, %v", data, err)
	}
	if kind == "link" {
		if target, err := os.Readlink(entry.Item); err != nil || target != entry.LinkTarget {
			t.Fatalf("moved link = %q, %v", target, err)
		}
		if data, err := os.ReadFile(filepath.Join(entry.LinkTarget, "SKILL.md")); err != nil || string(data) != trashTestBody {
			t.Fatalf("target changed: %q, %v", data, err)
		}
	}
}

func trashTestMetadata(t *testing.T, trash Trash, entry TrashEntry) {
	t.Helper()
	dir := filepath.Join(trash.Dir, entry.ID)
	data, err := os.ReadFile(filepath.Join(dir, "meta.json"))
	if err != nil {
		t.Fatal(err)
	}
	var stored trashMetadata
	if err := json.Unmarshal(data, &stored); err != nil {
		t.Fatal(err)
	}
	want := entry
	want.Item = ""
	if stored.Version != 1 || !reflect.DeepEqual(stored.TrashEntry, want) {
		t.Fatalf("metadata = %#v, want %#v", stored, want)
	}
	if runtime.GOOS == "windows" {
		return
	}
	for path, mode := range map[string]fs.FileMode{trash.Dir: 0o700, dir: 0o700, filepath.Join(dir, "item"): 0o700, filepath.Join(dir, "meta.json"): 0o600} {
		if info, err := os.Stat(path); err != nil || info.Mode().Perm() != mode {
			t.Fatalf("permissions of %s = %v, %v; want %o", path, info, err, mode)
		}
	}
}

func trashTestBroken(t *testing.T) (Trash, Summary, string) {
	t.Helper()
	trashTestRequireLinks(t)
	home := t.TempDir()
	root := filepath.Join(home, "skills")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "broken")
	if err := os.Symlink("../missing", path); err != nil {
		t.Fatal(err)
	}
	summary := Summary{Provider: ProviderClaude, Roots: []RootInfo{{Path: root, Scope: ScopeUser, Exists: true}},
		Issues: []Issue{{Kind: IssueBrokenLink, Path: path, Removable: true}}}
	return NewTrash(filepath.Join(home, ".quosparr"), home), summary, path
}

func TestTrashMoveBrokenLink(t *testing.T) {
	for _, roots := range []bool{true, false} {
		t.Run(fmt.Sprintf("roots-%v", roots), func(t *testing.T) {
			trash, summary, path := trashTestBroken(t)
			if !roots {
				summary.Roots = nil
			}
			now := time.Now()
			entry, system, err := trash.MoveBrokenLink(summary, path, now, trashTestNoSystem(t))
			if err != nil || system {
				t.Fatalf("move broken link = %#v, %v, %v", entry, system, err)
			}
			if entry.Kind != TrashBrokenLink || entry.Provider != summary.Provider || entry.Name != "broken" || entry.Original != path || entry.Root != filepath.Dir(path) || !entry.IsLink || entry.LinkTarget != "../missing" || entry.Size != 0 || !entry.DeletedAt.Equal(now) {
				t.Fatalf("broken link entry = %#v", entry)
			}
			trashTestMissing(t, path)
			if _, err := trash.Restore(entry.ID); err != nil {
				t.Fatal(err)
			}
			if target, err := os.Readlink(path); err != nil || target != "../missing" {
				t.Fatalf("restored broken link = %q, %v", target, err)
			}
		})
	}
}

func trashTestInjectRename(t *testing.T, failure error, cross bool) {
	t.Helper()
	oldRename, oldCross := renameFile, isCrossDevice
	t.Cleanup(func() { renameFile, isCrossDevice = oldRename, oldCross })
	renameFile = func(string, string) error { return failure }
	isCrossDevice = func(error) bool { return cross }
}

func TestTrashRenameFailures(t *testing.T) {
	for _, broken := range []bool{false, true} {
		for _, kind := range []string{"cross-device", "ordinary", "system-error"} {
			t.Run(fmt.Sprintf("%s-broken-%v", kind, broken), func(t *testing.T) {
				trash, skill, roots := trashTestSkill(t, "directory")
				path := skill.Removal.Path
				var summary Summary
				if broken {
					trash, summary, path = trashTestBroken(t)
				}
				failure := &os.LinkError{Op: "rename", Old: path, Err: syscall.EXDEV}
				trashTestInjectRename(t, failure, kind != "ordinary")
				systemErr := errors.New("system unavailable")
				var calls []string
				system := func(path string) error {
					calls = append(calls, path)
					trashTestEmptyDir(t, trash.Dir)
					if kind == "system-error" {
						return systemErr
					}
					return nil
				}
				var entry TrashEntry
				var inSystem bool
				var err error
				if broken {
					entry, inSystem, err = trash.MoveBrokenLink(summary, path, time.Now(), system)
				} else {
					entry, inSystem, err = trash.MoveSkill(skill, roots, time.Now(), system)
				}
				trashTestRenameResult(t, kind, err, failure, systemErr, inSystem)
				if kind != "ordinary" && (!slices.Equal(calls, []string{path}) || entry.ID != "" || entry.Item != "" || entry.Original != path) {
					t.Fatalf("system fallback = %#v, calls %v", entry, calls)
				}
				if kind == "ordinary" && len(calls) != 0 {
					t.Fatalf("ordinary failure called system: %v", calls)
				}
				trashTestEmptyDir(t, trash.Dir)
				if _, err := os.Lstat(path); err != nil {
					t.Fatalf("source changed: %v", err)
				}
			})
		}
	}
}

func trashTestRenameResult(t *testing.T, kind string, err, failure, systemErr error, inSystem bool) {
	t.Helper()
	want := error(nil)
	if kind == "ordinary" {
		want = failure
	} else if kind == "system-error" {
		want = systemErr
	}
	if !errors.Is(err, want) || inSystem != (kind == "cross-device") {
		t.Fatalf("result = %v, %v; want %v", inSystem, err, want)
	}
	if err != nil && !strings.HasPrefix(err.Error(), "skills: move to trash: ") {
		t.Fatalf("unwrapped move error: %v", err)
	}
}

func trashTestEmptyDir(t *testing.T, path string) {
	t.Helper()
	entries, err := os.ReadDir(path)
	if err != nil || len(entries) != 0 {
		t.Fatalf("expected empty directory %s: %v, %v", path, entries, err)
	}
}

func TestTrashMoveValidation(t *testing.T) {
	for _, kind := range []string{"nil-system", "not-deletable", "non-user", "outside", "changed"} {
		t.Run(kind, func(t *testing.T) {
			trash, skill, roots := trashTestSkill(t, "directory")
			system := trashTestNoSystem(t)
			want := ErrNotDeletable
			switch kind {
			case "nil-system":
				system = nil
			case "not-deletable":
				skill.Deletable = false
			case "non-user":
				skill.Scope = ScopeSystem
			case "outside":
				roots = nil
				want = ErrOutsideRoot
			case "changed":
				skill.ModTime = skill.ModTime.Add(time.Second)
				want = ErrChanged
			}
			if _, _, err := trash.MoveSkill(skill, roots, time.Now(), system); !errors.Is(err, want) {
				t.Fatalf("validation = %v, want %v", err, want)
			}
			trashTestMissing(t, trash.Dir)
			if _, err := os.Stat(skill.File); err != nil {
				t.Fatalf("validation changed source: %v", err)
			}
		})
	}
}

func TestTrashBrokenLinkValidation(t *testing.T) {
	for _, kind := range []string{"nil-system", "unlisted", "repaired"} {
		t.Run(kind, func(t *testing.T) {
			trash, summary, path := trashTestBroken(t)
			system, want := trashTestNoSystem(t), ErrNotDeletable
			switch kind {
			case "nil-system":
				system = nil
			case "unlisted":
				summary.Issues = nil
			case "repaired":
				commonWriteFile(t, filepath.Join(trash.Home, "missing"), "target")
				want = ErrChanged
			}
			if _, _, err := trash.MoveBrokenLink(summary, path, time.Now(), system); !errors.Is(err, want) {
				t.Fatalf("validation = %v, want %v", err, want)
			}
			trashTestMissing(t, trash.Dir)
		})
	}
}

func TestTrashListOrderAndPaths(t *testing.T) {
	trash, skill, roots := trashTestSkill(t, "directory")
	first := trashTestMove(t, trash, skill, roots, time.Unix(100, 0))
	var want []TrashEntry
	for _, id := range []string{"entry-a", "entry-z", "entry-later"} {
		entry := first
		entry.ID = id
		entry.Item = filepath.Join(trash.Dir, id, "item", filepath.Base(entry.Original))
		if id == "entry-later" {
			entry.DeletedAt = entry.DeletedAt.Add(time.Second)
		}
		commonWriteFile(t, filepath.Join(entry.Item, "SKILL.md"), trashTestBody)
		stored := entry
		stored.ID, stored.Item = "forged-id", filepath.Join(trash.Home, "outside")
		trashTestWriteMeta(t, trash, id, trashMetadata{Version: 1, TrashEntry: stored})
		want = append(want, entry)
	}
	want = []TrashEntry{want[2], want[1], want[0], first}
	got, err := trash.List(context.Background())
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("list = %#v, %v; want %#v", got, err, want)
	}
}

func trashTestWriteMeta(t *testing.T, trash Trash, id string, stored trashMetadata) {
	t.Helper()
	data, err := json.Marshal(stored)
	if err != nil {
		t.Fatal(err)
	}
	commonWriteFile(t, filepath.Join(trash.Dir, id, "meta.json"), string(data))
}

func TestTrashListDamagedEntries(t *testing.T) {
	for _, damage := range []string{"missing-meta", "bad-json", "missing-item", "version", "oversized", "relative", "invalid-dir"} {
		t.Run(damage, func(t *testing.T) {
			trash, skill, roots := trashTestSkill(t, "directory")
			entry := trashTestMove(t, trash, skill, roots, time.Now())
			dir := filepath.Join(trash.Dir, entry.ID)
			meta := filepath.Join(dir, "meta.json")
			switch damage {
			case "missing-meta":
				if err := os.Remove(meta); err != nil {
					t.Fatal(err)
				}
			case "bad-json":
				commonWriteFile(t, meta, "{")
			case "missing-item":
				if err := os.Rename(entry.Item, filepath.Join(trash.Home, "saved-item")); err != nil {
					t.Fatal(err)
				}
			case "version":
				trashTestWriteMeta(t, trash, entry.ID, trashMetadata{Version: 2, TrashEntry: entry})
			case "oversized":
				commonWriteFile(t, meta, strings.Repeat(" ", maxTrashMeta+1))
			case "relative":
				entry.Original = "relative"
				trashTestWriteMeta(t, trash, entry.ID, trashMetadata{Version: 1, TrashEntry: entry})
			case "invalid-dir":
				if err := os.Rename(dir, filepath.Join(trash.Dir, "bad.id")); err != nil {
					t.Fatal(err)
				}
			}
			if entries, err := trash.List(context.Background()); err != nil || len(entries) != 0 {
				t.Fatalf("damaged list = %#v, %v", entries, err)
			}
			if _, err := trash.Restore(entry.ID); !errors.Is(err, ErrTrashEntry) {
				t.Fatalf("damaged restore = %v", err)
			}
		})
	}
}

func TestTrashMissingAndCanceled(t *testing.T) {
	trash := NewTrash(t.TempDir(), "")
	if entries, err := trash.List(context.Background()); err != nil || len(entries) != 0 {
		t.Fatalf("missing list = %v, %v", entries, err)
	}
	if count, err := trash.Empty(nil); err != nil || count != 0 {
		t.Fatalf("missing empty = %d, %v", count, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := trash.List(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled list = %v", err)
	}
}

func TestTrashRestore(t *testing.T) {
	for _, kind := range []string{"directory", "collection", "link"} {
		t.Run(kind, func(t *testing.T) {
			trash, skill, roots := trashTestSkill(t, kind)
			entry := trashTestMove(t, trash, skill, roots, time.Now())
			got, err := trash.Restore(entry.ID)
			if err != nil || !reflect.DeepEqual(got, entry) {
				t.Fatalf("restore = %#v, %v; want %#v", got, err, entry)
			}
			if data, err := os.ReadFile(skill.File); err != nil || string(data) != trashTestBody {
				t.Fatalf("restored content = %q, %v", data, err)
			}
			if kind == "link" {
				if target, err := os.Readlink(skill.Removal.Path); err != nil || target != skill.LinkTarget {
					t.Fatalf("restored link = %q, %v", target, err)
				}
			}
			trashTestMissing(t, filepath.Join(trash.Dir, entry.ID))
		})
	}
}

func TestTrashRestoreFailures(t *testing.T) {
	for _, kind := range []string{"conflict", "parent-gone", "parent-file", "outside-root", "root-itself", "root-case", "relative", "rename"} {
		t.Run(kind, func(t *testing.T) {
			trash, skill, roots := trashTestSkill(t, "directory")
			entry := trashTestMove(t, trash, skill, roots, time.Now())
			want := ErrTrashEntry
			switch kind {
			case "conflict":
				commonWriteFile(t, entry.Original, "occupied")
				want = ErrTrashConflict
			case "parent-gone", "parent-file":
				if err := os.Remove(skill.Root); err != nil {
					t.Fatal(err)
				}
				if kind == "parent-file" {
					commonWriteFile(t, skill.Root, "parent is a file")
				}
				want = ErrTrashParentGone
			case "outside-root":
				entry.Original = filepath.Join(trash.Home, "outside", filepath.Base(entry.Original))
			case "root-itself":
				entry.Original = entry.Root
			case "root-case":
				entry.Original = strings.ToUpper(entry.Root)
			case "relative":
				entry.Original = "one"
			case "rename":
				want = errors.New("restore failed")
				trashTestInjectRename(t, want, false)
			}
			trashTestWriteMeta(t, trash, entry.ID, trashMetadata{Version: 1, TrashEntry: entry})
			if _, err := trash.Restore(entry.ID); !errors.Is(err, want) {
				t.Fatalf("restore error = %v, want %v", err, want)
			}
			if _, err := os.Lstat(entry.Item); err != nil {
				t.Fatalf("failed restore changed item: %v", err)
			}
		})
	}
}

func TestTrashRejectsInvalidIDs(t *testing.T) {
	trash := NewTrash(t.TempDir(), "")
	for _, id := range []string{"../x", "", "a/b", ".", "..", `a\b`, "a.b", strings.Repeat("a", 129)} {
		t.Run(id, func(t *testing.T) {
			if _, err := trash.Restore(id); !errors.Is(err, ErrTrashEntry) {
				t.Fatalf("restore id %q = %v", id, err)
			}
			if err := trash.Purge(id, trashTestNoSystem(t)); !errors.Is(err, ErrTrashEntry) {
				t.Fatalf("purge id %q = %v", id, err)
			}
		})
	}
}

func TestTrashPurgeAndEmpty(t *testing.T) {
	trash, skill, roots := trashTestSkill(t, "directory")
	entry := trashTestMove(t, trash, skill, roots, time.Now())
	dir := filepath.Join(trash.Dir, entry.ID)
	if err := trash.Purge(entry.ID, nil); !errors.Is(err, ErrNotDeletable) {
		t.Fatalf("nil purge = %v", err)
	}
	failure := errors.New("system failure")
	if err := trash.Purge(entry.ID, func(path string) error {
		if path != dir {
			t.Fatalf("purge path = %s, want %s", path, dir)
		}
		return failure
	}); !errors.Is(err, failure) {
		t.Fatalf("purge error = %v", err)
	}
	saved := filepath.Join(trash.Home, "system-trash-entry")
	if err := trash.Purge(entry.ID, func(path string) error { return os.Rename(path, saved) }); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(saved, "item", filepath.Base(entry.Original), "SKILL.md")); err != nil || string(data) != trashTestBody {
		t.Fatalf("purge content = %q, %v", data, err)
	}
	trashTestMissing(t, dir)
	trashTestEmpty(t, trash)
}

func trashTestEmpty(t *testing.T, trash Trash) {
	t.Helper()
	for _, id := range []string{"a", "b", "c", "invalid.id"} {
		commonWriteFile(t, filepath.Join(trash.Dir, id, "payload"), "keep")
	}
	commonWriteFile(t, filepath.Join(trash.Dir, "plain-file"), "keep")
	if _, err := trash.Empty(nil); !errors.Is(err, ErrNotDeletable) {
		t.Fatalf("nil empty = %v", err)
	}
	first, second := errors.New("first"), errors.New("second")
	var calls []string
	count, err := trash.Empty(func(path string) error {
		calls = append(calls, path)
		switch filepath.Base(path) {
		case "a":
			return first
		case "b":
			return second
		}
		return nil
	})
	if count != 1 || !errors.Is(err, first) || !errors.Is(err, second) {
		t.Fatalf("empty = %d, %v", count, err)
	}
	want := []string{filepath.Join(trash.Dir, "a"), filepath.Join(trash.Dir, "b"), filepath.Join(trash.Dir, "c")}
	if !slices.Equal(calls, want) {
		t.Fatalf("empty paths = %v, want %v", calls, want)
	}
	for _, path := range calls {
		if data, err := os.ReadFile(filepath.Join(path, "payload")); err != nil || string(data) != "keep" {
			t.Fatalf("empty deleted content: %q, %v", data, err)
		}
	}
}

func TestTrashSize(t *testing.T) {
	trashTestRequireLinks(t)
	home := t.TempDir()
	dir := filepath.Join(home, "content")
	commonWriteFile(t, filepath.Join(dir, "one"), "123")
	commonWriteFile(t, filepath.Join(home, "target", "two"), "12345")
	link := filepath.Join(dir, "link")
	if err := os.Symlink(filepath.Join(home, "target"), link); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		path string
		size int64
	}{{dir, 3}, {link, 0}, {filepath.Join(home, "missing"), -1}} {
		if size := trashSize(test.path); size != test.size {
			t.Fatalf("size of %s = %d, want %d", test.path, size, test.size)
		}
	}
}

func TestTrashOriginalLabel(t *testing.T) {
	for _, home := range []string{"empty", "outside"} {
		t.Run(home, func(t *testing.T) {
			trash, skill, roots := trashTestSkill(t, "directory")
			trash.Home = ""
			if home == "outside" {
				trash.Home = skill.Removal.Path + "-other"
			}
			entry := trashTestMove(t, trash, skill, roots, time.Now())
			if entry.OriginalLabel != entry.Original {
				t.Fatalf("label = %s, want %s", entry.OriginalLabel, entry.Original)
			}
		})
	}
}

func TestTrashCrossDevice(t *testing.T) {
	device := syscall.EXDEV
	if runtime.GOOS == "windows" {
		device = syscall.Errno(17)
	}
	if !crossDevice(&os.LinkError{Op: "rename", Err: device}) || crossDevice(fs.ErrPermission) {
		t.Fatal("cross-device classification failed")
	}
}

func TestTrashMoveFile(t *testing.T) {
	trash, skill, roots := trashTestSkill(t, "directory")
	skill.Removal.Path = skill.File
	entry := trashTestMove(t, trash, skill, roots, time.Now())
	if data, err := os.ReadFile(entry.Item); err != nil || string(data) != trashTestBody || entry.Size != int64(len(data)) {
		t.Fatalf("moved file = %q, %d, %v", data, entry.Size, err)
	}
	trashTestMissing(t, skill.File)
	if _, err := trash.Restore(entry.ID); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(skill.File); err != nil || string(data) != trashTestBody {
		t.Fatalf("restored file = %q, %v", data, err)
	}
}

func TestTrashOversizedMetadata(t *testing.T) {
	trash, skill, roots := trashTestSkill(t, "directory")
	skill.Description = strings.Repeat("x", maxTrashMeta)
	if _, _, err := trash.MoveSkill(skill, roots, time.Now(), trashTestNoSystem(t)); !errors.Is(err, ErrTrashEntry) {
		t.Fatalf("oversized metadata = %v", err)
	}
	trashTestEmptyDir(t, trash.Dir)
	if data, err := os.ReadFile(skill.File); err != nil || string(data) != trashTestBody {
		t.Fatalf("source changed: %q, %v", data, err)
	}
}

func TestTrashSizeLimit(t *testing.T) {
	saved := maxTrashWalk
	maxTrashWalk = 20
	t.Cleanup(func() { maxTrashWalk = saved })
	dir := t.TempDir()
	for i := range maxTrashWalk - 1 {
		if err := os.Mkdir(filepath.Join(dir, fmt.Sprint(i)), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if size := trashSize(dir); size != 0 {
		t.Fatalf("size at limit = %d, want 0", size)
	}
	if err := os.Mkdir(filepath.Join(dir, "extra"), 0o700); err != nil {
		t.Fatal(err)
	}
	if size := trashSize(dir); size != -1 {
		t.Fatalf("size above limit = %d, want -1", size)
	}
}

func TestTrashDoesNotFollowStorageLinks(t *testing.T) {
	trashTestRequireLinks(t)
	for _, kind := range []string{"entry", "metadata", "item-directory"} {
		t.Run(kind, func(t *testing.T) {
			trash, skill, roots := trashTestSkill(t, "directory")
			entry := trashTestMove(t, trash, skill, roots, time.Now())
			path := filepath.Join(trash.Dir, entry.ID)
			if kind == "metadata" {
				path = filepath.Join(path, "meta.json")
			} else if kind == "item-directory" {
				path = filepath.Join(path, "item")
			}
			saved := filepath.Join(trash.Home, "saved")
			if err := os.Rename(path, saved); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(saved, path); err != nil {
				t.Fatal(err)
			}
			if entries, err := trash.List(context.Background()); err != nil || len(entries) != 0 {
				t.Fatalf("storage link list = %v, %v", entries, err)
			}
			if _, err := trash.Restore(entry.ID); !errors.Is(err, ErrTrashEntry) {
				t.Fatalf("storage link restore = %v", err)
			}
			if kind == "entry" {
				if err := trash.Purge(entry.ID, trashTestNoSystem(t)); !errors.Is(err, ErrTrashEntry) {
					t.Fatalf("storage link purge = %v", err)
				}
				if count, err := trash.Empty(trashTestNoSystem(t)); err != nil || count != 0 {
					t.Fatalf("storage link empty = %d, %v", count, err)
				}
			}
		})
	}
}

func TestTrashRestoreRejectsChangedParentLink(t *testing.T) {
	trashTestRequireLinks(t)
	trash, skill, roots := trashTestSkill(t, "directory")
	group := filepath.Join(skill.Root, "group")
	if err := os.Mkdir(group, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(skill.Dir, filepath.Join(group, "one")); err != nil {
		t.Fatal(err)
	}
	skill.Dir, skill.Removal.Path = filepath.Join(group, "one"), filepath.Join(group, "one")
	skill.File = filepath.Join(skill.Dir, "SKILL.md")
	entry := trashTestMove(t, trash, skill, roots, time.Now())
	if err := os.Remove(group); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), group); err != nil {
		t.Fatal(err)
	}
	if _, err := trash.Restore(entry.ID); !errors.Is(err, ErrTrashEntry) {
		t.Fatalf("changed parent link = %v", err)
	}
	if _, err := os.Lstat(entry.Item); err != nil {
		t.Fatalf("failed restore changed item: %v", err)
	}
}
