package skills

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/licoy/quosparr/internal/platform"
)

// ErrNotFixable means a diagnostics entry has no one-click fix, or no longer
// has one because the files changed since the scan.
var ErrNotFixable = errors.New("skills: this problem has no automatic fix")

// Fix is a one-click fix for a diagnostics entry.
type Fix string

// Fix values.
const (
	FixRemoveLink     Fix = "removeLink"     // move the dangling link to the skills trash
	FixLiftNested     Fix = "liftNested"     // see Issue.Lift and LiftNested
	FixDisableCopy    Fix = "disableCopy"    // turn the second copy off by its path (Codex)
	FixTrashCopy      Fix = "trashCopy"      // move the second copy to the skills trash
	FixAddDescription Fix = "addDescription" // see AddDescription
)

// markFixes lists the fixes each issue can take, from the scan alone. A
// name switch is offered only where it is a path (Codex), so turning one
// copy off cannot turn off the other.
func markFixes(summary *Summary) {
	byFile := make(map[string]Skill, len(summary.Skills))
	for _, skill := range summary.Skills {
		byFile[filepath.Clean(skill.File)] = skill
	}
	for i := range summary.Issues {
		issue := &summary.Issues[i]
		issue.Fixes, issue.Lift = nil, NestedFix{}
		skill, ok := byFile[filepath.Clean(issue.Path)]
		switch issue.Kind {
		case IssueBrokenLink:
			if issue.Removable {
				issue.Fixes = append(issue.Fixes, FixRemoveLink)
			}
		case IssueNested:
			if lift, can := PlanNestedFix(*summary, skill); ok && can {
				issue.Fixes, issue.Lift = append(issue.Fixes, FixLiftNested), lift
			}
		case IssueMissingDesc:
			if ok && skill.Scope == ScopeUser {
				issue.Fixes = append(issue.Fixes, FixAddDescription)
			}
		case IssueDuplicateName:
			if ok && skill.Provider == ProviderCodex && skill.Toggleable && skill.State == StateOn {
				issue.Fixes = append(issue.Fixes, FixDisableCopy)
			}
			if ok && skill.Deletable {
				issue.Fixes = append(issue.Fixes, FixTrashCopy)
			}
		}
	}
}

// maxSuggestedDescription bounds the description AddDescription proposes.
const maxSuggestedDescription = 200

// NestedFix is how nested Claude skills are lifted: Wrapper, the folder at
// the top of the user root that holds them, goes to the Quosparr skills
// trash with everything else in it, and every skill found in it is copied
// to the top of the root first.
type NestedFix struct {
	Wrapper string
	IsLink  bool // the wrapper is a link; only the link is moved
	Lifts   []Lift
	// Extra counts the wrapper's top-level entries that hold none of the
	// skills (README, .git, …), which go to the trash with it.
	Extra int
}

// Lift is one skill of a NestedFix and the folder it is copied to.
type Lift struct {
	File   string // the skill's SKILL.md, as scanned
	Name   string
	Target string
}

// LiftResult is the outcome of LiftNested.
type LiftResult struct {
	Paths []string // the skills' new folders
	// WrapperInSystemTrash is set when the wrapper sat on another volume
	// and went to the system Trash instead of the Quosparr one.
	WrapperInSystemTrash bool
}

// PlanNestedFix reports how LiftNested would fix a nested Claude skill,
// together with every other skill in the same wrapper. It offers no fix
// when the wrapper holds a skill that loads, a skill inside another, two
// skills of one folder name, or when a place at the top is taken by
// something other than the wrapper.
func PlanNestedFix(summary Summary, skill Skill) (NestedFix, bool) {
	if !liftable(skill) {
		return NestedFix{}, false
	}
	root := filepath.Clean(skill.Root)
	if !userRootExists(summary.Roots, root) {
		return NestedFix{}, false
	}
	parts := relParts(root, skill.Dir)
	if len(parts) < 2 || !validTransferName(parts[0], ProviderClaude) {
		return NestedFix{}, false
	}
	wrapper := filepath.Join(root, parts[0])
	for _, issue := range summary.Issues {
		if issue.Kind == IssueTruncated && pathsOverlap(issue.Path, wrapper) {
			return NestedFix{}, false // the wrapper may hold skills the scan missed
		}
	}
	info, err := os.Lstat(wrapper)
	if err != nil {
		return NestedFix{}, false
	}
	fix := NestedFix{Wrapper: wrapper, IsLink: info.Mode()&os.ModeSymlink != 0}
	held := map[string]bool{}
	for _, other := range summary.Skills {
		if !within(wrapper, other.File) {
			continue
		}
		lift, ok := planLift(root, wrapper, other)
		if !ok {
			return NestedFix{}, false
		}
		held[relParts(root, other.Dir)[1]] = true
		fix.Lifts = append(fix.Lifts, lift)
	}
	for i, a := range fix.Lifts {
		for _, b := range fix.Lifts[i+1:] {
			if a.Target == b.Target || pathsOverlap(filepath.Dir(a.File), filepath.Dir(b.File)) {
				return NestedFix{}, false
			}
		}
	}
	if entries, err := os.ReadDir(wrapper); err == nil {
		for _, entry := range entries {
			if !held[entry.Name()] {
				fix.Extra++
			}
		}
	}
	return fix, len(fix.Lifts) > 0
}

func liftable(skill Skill) bool {
	return skill.Provider == ProviderClaude && skill.Reason == ReasonNested && skill.Scope == ScopeUser
}

func relParts(root, dir string) []string {
	rel, err := filepath.Rel(root, filepath.Clean(dir))
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil
	}
	return strings.Split(rel, string(filepath.Separator))
}

func planLift(root, wrapper string, skill Skill) (Lift, bool) {
	parts := relParts(root, skill.Dir)
	if !liftable(skill) || filepath.Clean(skill.Root) != root || len(parts) < 2 {
		return Lift{}, false
	}
	if skill.LinkPath != "" && filepath.Clean(skill.LinkPath) != wrapper {
		return Lift{}, false // a link deeper down: lifting would copy through it
	}
	name := parts[len(parts)-1]
	if !validTransferName(name, ProviderClaude) {
		return Lift{}, false
	}
	target := filepath.Join(root, name)
	if target != wrapper {
		if _, err := os.Lstat(target); !errors.Is(err, fs.ErrNotExist) {
			return Lift{}, false
		}
	}
	return Lift{File: skill.File, Name: skill.InvokeName, Target: target}, true
}

func userRootExists(roots []RootInfo, path string) bool {
	for _, root := range roots {
		if root.Scope == ScopeUser && root.Exists && filepath.Clean(root.Path) == path {
			return true
		}
	}
	return false
}

// LiftNested fixes nested Claude skills as PlanNestedFix describes. Every
// skill is copied first under a hidden name the CLI skips; only then does
// the wrapper go to the trash and the copies take their places. A failure
// takes the placed copies back out and puts the wrapper back.
func (t Trash) LiftNested(summary Summary, skill Skill, now time.Time, system func(string) error) (LiftResult, error) {
	fix, ok := PlanNestedFix(summary, skill)
	if !ok || system == nil {
		return LiftResult{}, ErrNotFixable
	}
	byFile := make(map[string]Skill, len(summary.Skills))
	for _, s := range summary.Skills {
		byFile[s.File] = s
	}
	root := filepath.Dir(fix.Wrapper)
	var copies []liftCopy
	undo := func(err error) (LiftResult, error) {
		for _, c := range copies {
			err = errors.Join(err, c.created.cleanup())
			_ = c.created.root.Close()
		}
		return LiftResult{}, err
	}
	for _, lift := range fix.Lifts {
		c, err := copyLift(byFile[lift.File], root, lift)
		if err != nil {
			return undo(err)
		}
		copies = append(copies, c)
	}

	entry, inSystem, err := t.moveWrapper(summary, skill, fix, now, system)
	if err != nil {
		return undo(err)
	}
	result := LiftResult{WrapperInSystemTrash: inSystem}
	for i, c := range copies {
		if err = placeLift(root, c); err != nil {
			// Placed copies go back under their hidden names so cleanup
			// finds them where it made them.
			for _, placed := range copies[:i] {
				err = errors.Join(err, renameFile(placed.lift.Target, filepath.Join(root, placed.temp)))
			}
			err = fmt.Errorf("skills: lift nested skill: %w", err)
			if !inSystem {
				if _, restoreErr := t.Restore(entry.ID); restoreErr != nil {
					err = errors.Join(err, restoreErr)
				}
			}
			return undo(err)
		}
		result.Paths = append(result.Paths, c.lift.Target)
	}
	for _, c := range copies {
		_ = c.created.root.Close()
	}
	return result, nil
}

type liftCopy struct {
	lift    Lift
	temp    string
	created *transferTarget
}

func copyLift(skill Skill, root string, lift Lift) (liftCopy, error) {
	if skill.File == "" {
		return liftCopy{}, ErrChanged
	}
	source, err := validateTransferSource(skill)
	if err != nil {
		return liftCopy{}, err
	}
	defer source.root.Close()
	var random [4]byte
	if _, err := rand.Read(random[:]); err != nil {
		return liftCopy{}, err
	}
	temp := "." + filepath.Base(lift.Target) + ".quosparr-" + hex.EncodeToString(random[:])
	created, err := copySkill(source, root, temp)
	if err != nil {
		return liftCopy{}, err
	}
	if err := revalidateTransferSource(skill, source); err != nil {
		err = errors.Join(err, created.cleanup())
		_ = created.root.Close()
		return liftCopy{}, err
	}
	return liftCopy{lift: lift, temp: temp, created: created}, nil
}

func placeLift(root string, c liftCopy) error {
	_, err := os.Lstat(c.lift.Target)
	if err == nil {
		return ErrTransferConflict
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return renameFile(filepath.Join(root, c.temp), c.lift.Target)
}

func (t Trash) moveWrapper(summary Summary, skill Skill, fix NestedFix, now time.Time, system func(string) error) (TrashEntry, bool, error) {
	trashMu.Lock()
	defer trashMu.Unlock()
	info, err := os.Lstat(fix.Wrapper)
	if err != nil {
		return TrashEntry{}, false, changed(err)
	}
	if (info.Mode()&os.ModeSymlink != 0) != fix.IsLink {
		return TrashEntry{}, false, ErrChanged
	}
	if !fix.IsLink {
		if err := validateRemovalRoot(fix.Wrapper, summary.Roots); err != nil {
			return TrashEntry{}, false, err
		}
	}
	covers := make([]string, len(fix.Lifts))
	for i, lift := range fix.Lifts {
		covers[i] = lift.Name
	}
	entry := TrashEntry{
		Kind: TrashSkill, Provider: skill.Provider, SharedWith: skill.SharedWith,
		Name: filepath.Base(fix.Wrapper), InvokeName: skill.InvokeName, DisplayName: skill.DisplayName,
		Description: skill.Description, Covers: covers,
		Original: fix.Wrapper, Root: filepath.Dir(fix.Wrapper), IsLink: fix.IsLink,
		Origin: skill.Origin, DeletedAt: now.UTC(),
	}
	if len(fix.Lifts) > 1 {
		entry.InvokeName, entry.DisplayName, entry.Description = filepath.Base(fix.Wrapper), "", ""
	}
	if fix.IsLink {
		entry.LinkTarget, _ = os.Readlink(fix.Wrapper)
	}
	return t.move(entry, system)
}

// SuggestDescription proposes the description AddDescription writes: the
// first paragraph of the body, or else the skill's name.
func SuggestDescription(skill Skill) string {
	text := ""
	if body, _, err := ReadBody(skill); err == nil {
		text = firstParagraph(body)
	}
	if text == "" {
		text = skill.Name
	}
	if utf8.RuneCountInString(text) > maxSuggestedDescription {
		text = strings.TrimSpace(string([]rune(text)[:maxSuggestedDescription-1])) + "…"
	}
	return text
}

// AddDescription writes description into the frontmatter of a user skill
// that has none, backing SKILL.md up first. It changes only that field: an
// empty description line is filled, otherwise one line is added, and a file
// without frontmatter gets one with its name and the description.
func AddDescription(skill Skill, description string, backup platform.BackupPolicy, now time.Time) error {
	description = singleLine(description)
	if skill.Scope != ScopeUser || description == "" || filepath.Base(skill.File) != "SKILL.md" {
		return ErrNotFixable
	}
	if err := validateSkillPath(skill.File); err != nil {
		return err
	}
	check := func() error {
		info, err := os.Stat(skill.File)
		if err != nil {
			return changed(err)
		}
		if !info.Mode().IsRegular() || !info.ModTime().Equal(skill.ModTime) {
			return ErrChanged
		}
		if real, err := filepath.EvalSymlinks(skill.File); err != nil || skill.RealFile != "" && filepath.Clean(real) != filepath.Clean(skill.RealFile) {
			return ErrChanged
		}
		return nil
	}
	if err := check(); err != nil {
		return err
	}
	return platform.EditFileWithBackupChecked(skill.File, backup, now, func(old []byte) ([]byte, error) {
		return insertDescription(old, filepath.Base(skill.Dir), description)
	}, check)
}

func insertDescription(data []byte, name, description string) ([]byte, error) {
	newline := "\n"
	if bytes.Contains(data, []byte("\r\n")) {
		newline = "\r\n"
	}
	line := "description: " + yamlQuote(description)
	if _, hasFront, err := frontmatterEnd(data); err != nil {
		return nil, ErrNotFixable
	} else if !hasFront {
		head := "---" + newline + "name: " + yamlQuote(name) + newline + line + newline + "---" + newline + newline
		return append([]byte(head), data...), nil
	}
	lines, offsets := splitLines(data)
	end := -1
	for i := 1; i < len(lines); i++ {
		if lines[i] == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		return nil, ErrNotFixable
	}
	for i := 1; i < end; i++ {
		key, raw, ok := splitKeyValue(lines[i])
		if !ok || leadingSpaces(lines[i]) != 0 || key != "description" {
			continue
		}
		// Only an empty value on its own line is filled; anything else is
		// a description written since the scan, or a form left alone.
		if v := strings.TrimSpace(raw); v != "" && v != `""` && v != "''" || i+1 < end && leadingSpaces(lines[i+1]) > 0 {
			return nil, ErrChanged
		}
		out := append([]byte(nil), data[:offsets[i]]...)
		out = append(out, line...)
		return append(out, data[offsets[i]+len(lines[i]):]...), nil
	}
	out := append([]byte(nil), data[:offsets[end]]...)
	out = append(out, line+newline...)
	return append(out, data[offsets[end]:]...), nil
}

// yamlQuote writes s as a double-quoted scalar; JSON's escapes are YAML's.
func yamlQuote(s string) string {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	return strings.TrimSuffix(b.String(), "\n")
}
