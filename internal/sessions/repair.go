package sessions

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/licoy/quosparr/internal/platform"
)

// Repairer diagnoses and rebuilds the session index of a Codex home.
type Repairer struct {
	CodexHome string
	// Now supplies the backup time; nil means time.Now.
	Now func() time.Time
}

// Report is what DryRun and Repair found. Every ID list is sorted.
type Report struct {
	// MissingIndexIDs have a session file but no index line.
	MissingIndexIDs []string
	// OrphanIndexIDs have an index line but no session file.
	OrphanIndexIDs []string
	// DuplicateIndexIDs appear on more than one index line.
	DuplicateIndexIDs []string
	// StaleTitleIDs are indexed under a title that differs from the session's.
	StaleTitleIDs []string
	// BackupPath is the copy made by Repair; empty for DryRun or when there
	// was no index to copy.
	BackupPath string
	// PlannedEntries is the number of lines the rebuilt index has.
	PlannedEntries int
}

// DryRun reports the index problems without writing anything.
func (r Repairer) DryRun() (Report, error) {
	sessions, err := r.scanSessions()
	if err != nil {
		return Report{}, err
	}
	index, err := readIndex(r.CodexHome)
	if err != nil {
		return Report{}, err
	}
	return buildReport(sessions, index, ""), nil
}

// Repair copies session_index.jsonl to session_index.backup-<unix>.jsonl, then
// replaces it with one line per session file found (newest first), written
// atomically. Session files are only read.
func (r Repairer) Repair() (Report, error) {
	sessions, err := r.scanSessions()
	if err != nil {
		return Report{}, err
	}
	index, err := readIndex(r.CodexHome)
	if err != nil {
		return Report{}, err
	}
	backup, err := r.backupIndex()
	if err != nil {
		return Report{}, err
	}
	if err := r.writeIndex(sessions); err != nil {
		return Report{}, err
	}
	return buildReport(sessions, index, backup), nil
}

func (r Repairer) indexPath() string { return filepath.Join(r.CodexHome, IndexFileName) }

func buildReport(sessions, index []indexEntry, backup string) Report {
	sessionIDs := map[string]bool{}
	for _, s := range sessions {
		sessionIDs[s.ID] = true
	}
	counts := map[string]int{}
	first := map[string]indexEntry{}
	for _, e := range index {
		counts[e.ID]++
		if _, ok := first[e.ID]; !ok {
			first[e.ID] = e
		}
	}
	report := Report{BackupPath: backup, PlannedEntries: len(sessions)}
	for id, n := range counts {
		if !sessionIDs[id] {
			report.OrphanIndexIDs = append(report.OrphanIndexIDs, id)
		}
		if n > 1 {
			report.DuplicateIndexIDs = append(report.DuplicateIndexIDs, id)
		}
	}
	for _, s := range sessions {
		entry, indexed := first[s.ID]
		switch {
		case !indexed:
			report.MissingIndexIDs = append(report.MissingIndexIDs, s.ID)
		case entry.ThreadName != s.ThreadName:
			report.StaleTitleIDs = append(report.StaleTitleIDs, s.ID)
		}
	}
	for _, ids := range [][]string{report.MissingIndexIDs, report.OrphanIndexIDs, report.DuplicateIndexIDs, report.StaleTitleIDs} {
		sort.Strings(ids)
	}
	return report
}

// scanSessions reads every session file, keeps the most recently updated one
// per id, and returns them newest first (ties by id, for a stable result).
func (r Repairer) scanSessions() ([]indexEntry, error) {
	best := map[string]indexEntry{}
	var order []string
	for _, folder := range []string{"sessions", "archived_sessions"} {
		root := filepath.Join(r.CodexHome, folder)
		var paths []string
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if path == root && errors.Is(err, fs.ErrNotExist) {
				return nil // The CLI need not have created either session tree yet.
			}
			if err != nil {
				return err
			}
			if path == root && !d.IsDir() {
				return fmt.Errorf("session root is not a directory: %s", folder)
			}
			if !d.IsDir() && filepath.Ext(path) == ".jsonl" {
				paths = append(paths, path)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("scan %s: %w", folder, err)
		}
		for _, path := range paths {
			entry, ok, err := parseIndexEntry(path)
			if err != nil {
				return nil, err
			}
			if !ok {
				continue
			}
			existing, seen := best[entry.ID]
			if !seen {
				order = append(order, entry.ID)
			}
			if !seen || entry.UpdatedAt.After(existing.UpdatedAt) {
				best[entry.ID] = entry
			}
		}
	}
	entries := make([]indexEntry, 0, len(order))
	for _, id := range order {
		entries = append(entries, best[id])
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].UpdatedAt.Equal(entries[j].UpdatedAt) {
			return entries[i].ID < entries[j].ID
		}
		return entries[i].UpdatedAt.After(entries[j].UpdatedAt)
	})
	return entries, nil
}

const (
	repairHeadBytes = 64 * 1024
	repairHeadLines = 40
	repairTailLines = 80
)

// parseIndexEntry derives an index line from a session file: its id from the
// session_meta record, its time from the newest timestamp and its title from
// the first user message, looking only at the first 40 lines of the first
// 64 KiB and the last 80 lines of the last 64 KiB.
func parseIndexEntry(path string) (indexEntry, bool, error) {
	lines, err := repairProbeLines(path)
	if err != nil {
		return indexEntry{}, false, err
	}
	var id, title string
	var updatedAt time.Time
	hasTime := false
	for _, line := range lines {
		obj, ok := decodeLine(line)
		if !ok {
			continue
		}
		if stamp, ok := stringOf(obj, "timestamp"); ok {
			if date, ok := parseTimestamp(stamp); ok {
				if !hasTime || date.After(updatedAt) {
					updatedAt = date
				}
				hasTime = true
			}
		}
		payload := objectOf(obj["payload"])
		if t, _ := stringOf(obj, "type"); t == "session_meta" && id == "" {
			if v, ok := stringOf(payload, "id"); ok {
				id = v
			} else if v, ok := stringOf(payload, "session_id"); ok {
				id = v
			}
		}
		if title == "" {
			title = userTitle(payload)
		}
	}
	if id == "" || !hasTime {
		return indexEntry{}, false, nil
	}
	if title == "" {
		title = untitled
	}
	return indexEntry{ID: id, ThreadName: title, UpdatedAt: updatedAt}, true, nil
}

func repairProbeLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	end, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, err
	}
	head, err := readAt(f, 0, min(end, repairHeadBytes))
	if err != nil {
		return nil, err
	}
	tailStart := max(end-repairHeadBytes, 0)
	tail, err := readAt(f, tailStart, end-tailStart)
	if err != nil {
		return nil, err
	}
	headLines := splitLines(head)
	if len(headLines) > repairHeadLines {
		headLines = headLines[:repairHeadLines]
	}
	tailLines := splitLines(tail)
	if len(tailLines) > repairTailLines {
		tailLines = tailLines[len(tailLines)-repairTailLines:]
	}
	return append(headLines, tailLines...), nil
}

// backupIndex copies the current index next to itself. It returns "" when
// there is no index. An existing backup of the same second is never
// overwritten: that is an error.
func (r Repairer) backupIndex() (string, error) {
	src, err := os.Open(r.indexPath())
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	defer src.Close()
	info, err := src.Stat()
	if err != nil {
		return "", err
	}
	now := time.Now
	if r.Now != nil {
		now = r.Now
	}
	backup := filepath.Join(r.CodexHome, fmt.Sprintf("session_index.backup-%d.jsonl", now().Unix()))
	dst, err := os.OpenFile(backup, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		os.Remove(backup)
		return "", err
	}
	if err := dst.Sync(); err != nil {
		dst.Close()
		os.Remove(backup)
		return "", err
	}
	if err := dst.Close(); err != nil {
		os.Remove(backup)
		return "", err
	}
	return backup, nil
}

// writeIndex replaces the index atomically, keeping the mode of the file it
// replaces (0644 for a new one).
func (r Repairer) writeIndex(entries []indexEntry) error {
	lines := make([][]byte, 0, len(entries))
	for _, e := range entries {
		line, err := encodeIndexLine(e)
		if err != nil {
			return err
		}
		lines = append(lines, line)
	}
	perm := fs.FileMode(0o644)
	if info, err := os.Stat(r.indexPath()); err == nil {
		perm = info.Mode().Perm()
	}
	return platform.WriteFileAtomic(r.indexPath(), joinLines(lines), perm)
}
