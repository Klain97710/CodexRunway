package skills

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/platform"
)

func fixTestNested(t *testing.T, env Env, wrapper string) (Summary, Skill) {
	t.Helper()
	summary, err := ScanClaude(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	for _, skill := range summary.Skills {
		if skill.Reason == ReasonNested && strings.Contains(skill.File, wrapper) {
			return summary, skill
		}
	}
	t.Fatalf("no nested skill under %s in %#v", wrapper, summary.Skills)
	return Summary{}, Skill{}
}

func TestLiftNestedReplacesSameNameWrapper(t *testing.T) {
	env := claudeTestEnv(t)
	root := filepath.Join(env.ClaudeDir, "skills")
	claudeWriteSkill(t, filepath.Join(root, "ponytail", "ponytail"), "ponytail", "Tie hair")
	claudeWriteFile(t, filepath.Join(root, "ponytail", "README.md"), "readme")
	summary, skill := fixTestNested(t, env, "ponytail")
	fix, ok := PlanNestedFix(summary, skill)
	if !ok || fix.Wrapper != filepath.Join(root, "ponytail") || len(fix.Lifts) != 1 || fix.Lifts[0].Target != fix.Wrapper || fix.Extra != 1 {
		t.Fatalf("plan = %+v, %v", fix, ok)
	}
	trash := NewTrash(filepath.Join(env.Home, ".quosparr"), env.Home)
	got, err := trash.LiftNested(summary, skill, env.Now, trashTestNoSystem(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Paths) != 1 || got.Paths[0] != filepath.Join(root, "ponytail") || got.WrapperInSystemTrash {
		t.Fatalf("result = %+v", got)
	}
	if data, err := os.ReadFile(filepath.Join(root, "ponytail", "SKILL.md")); err != nil || !strings.Contains(string(data), "Tie hair") {
		t.Fatalf("lifted skill = %q, %v", data, err)
	}
	trashTestMissing(t, filepath.Join(root, "ponytail", "README.md"))
	entries, err := trash.List(context.Background())
	if err != nil || len(entries) != 1 || entries[0].Original != filepath.Join(root, "ponytail") {
		t.Fatalf("trash = %+v, %v", entries, err)
	}
	if _, err := os.Stat(filepath.Join(entries[0].Item, "README.md")); err != nil {
		t.Fatalf("wrapper contents not kept in trash: %v", err)
	}
	after, err := ScanClaude(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	for _, issue := range after.Issues {
		if issue.Kind == IssueNested {
			t.Fatalf("still nested: %+v", issue)
		}
	}
	names, _ := os.ReadDir(root)
	for _, e := range names {
		if strings.HasPrefix(e.Name(), ".") {
			t.Fatalf("left temporary %s", e.Name())
		}
	}
}

func TestLiftNestedLiftsWholeCollection(t *testing.T) {
	env := claudeTestEnv(t)
	root := filepath.Join(env.ClaudeDir, "skills")
	claudeWriteSkill(t, filepath.Join(root, "pack", "pack"), "pack", "Main")
	claudeWriteSkill(t, filepath.Join(root, "pack", "pack-audit"), "pack-audit", "Audit")
	claudeWriteSkill(t, filepath.Join(root, "pack", "more", "deep"), "deep", "Deep")
	claudeWriteFile(t, filepath.Join(root, "pack", "LICENSE"), "MIT")
	summary, skill := fixTestNested(t, env, "pack-audit")
	fix, ok := PlanNestedFix(summary, skill)
	if !ok || len(fix.Lifts) != 3 || fix.Extra != 1 {
		t.Fatalf("plan = %+v, %v", fix, ok)
	}
	trash := NewTrash(filepath.Join(env.Home, ".quosparr"), env.Home)
	got, err := trash.LiftNested(summary, skill, env.Now, trashTestNoSystem(t))
	if err != nil || len(got.Paths) != 3 {
		t.Fatalf("lift = %+v, %v", got, err)
	}
	for _, name := range []string{"pack", "pack-audit", "deep"} {
		if _, err := os.Stat(filepath.Join(root, name, "SKILL.md")); err != nil {
			t.Fatalf("%s not lifted: %v", name, err)
		}
	}
	trashTestMissing(t, filepath.Join(root, "pack", "LICENSE"))
	entries, err := trash.List(context.Background())
	if err != nil || len(entries) != 1 || len(entries[0].Covers) != 3 || entries[0].InvokeName != "pack" {
		t.Fatalf("trash = %+v, %v", entries, err)
	}
	after, err := ScanClaude(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Skills) != 3 {
		t.Fatalf("after = %+v", after.Skills)
	}
	for _, s := range after.Skills {
		if s.Reason == ReasonNested {
			t.Fatalf("still nested: %+v", s)
		}
	}
}

func TestPlanNestedFixRefusesTakenTargetAndLoadedSkill(t *testing.T) {
	env := claudeTestEnv(t)
	root := filepath.Join(env.ClaudeDir, "skills")
	claudeWriteSkill(t, filepath.Join(root, "wrap", "taken"), "taken", "Nested")
	claudeWriteSkill(t, filepath.Join(root, "taken"), "taken-top", "Top")
	claudeWriteSkill(t, filepath.Join(root, "host"), "host", "Loads")
	claudeWriteSkill(t, filepath.Join(root, "host", "inner"), "inner", "Nested in a skill")
	claudeWriteSkill(t, filepath.Join(root, "twins", "a", "same"), "same-a", "A")
	claudeWriteSkill(t, filepath.Join(root, "twins", "b", "same"), "same-b", "B")
	summary, err := ScanClaude(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	nested := 0
	for _, skill := range summary.Skills {
		if skill.Reason != ReasonNested {
			continue
		}
		nested++
		if fix, ok := PlanNestedFix(summary, skill); ok {
			t.Fatalf("offered %+v for %s", fix, skill.File)
		}
		trash := NewTrash(filepath.Join(env.Home, ".quosparr"), env.Home)
		if _, err := trash.LiftNested(summary, skill, env.Now, trashTestNoSystem(t)); !errors.Is(err, ErrNotFixable) {
			t.Fatalf("lift %s = %v", skill.File, err)
		}
	}
	if nested != 4 {
		t.Fatalf("nested skills = %d", nested)
	}
}

func TestLiftNestedRefusesPlaceTakenSinceScan(t *testing.T) {
	env := claudeTestEnv(t)
	root := filepath.Join(env.ClaudeDir, "skills")
	claudeWriteSkill(t, filepath.Join(root, "kit", "kit"), "kit", "Kit")
	claudeWriteSkill(t, filepath.Join(root, "kit", "zz-late"), "zz-late", "Late")
	summary, skill := fixTestNested(t, env, "kit")
	fix, ok := PlanNestedFix(summary, skill)
	if !ok || len(fix.Lifts) != 2 {
		t.Fatalf("plan = %+v, %v", fix, ok)
	}
	// Something takes a place at the top after the scan.
	late := fix.Lifts[1].Target
	if filepath.Base(late) == "kit" {
		late = fix.Lifts[0].Target
	}
	claudeWriteFile(t, filepath.Join(late, "note.txt"), "mine")
	trash := NewTrash(filepath.Join(env.Home, ".quosparr"), env.Home)
	if _, err := trash.LiftNested(summary, skill, env.Now, trashTestNoSystem(t)); err == nil {
		t.Fatal("lifted over a taken place")
	}
	for _, name := range []string{"kit", "zz-late"} {
		if _, err := os.Stat(filepath.Join(root, "kit", name, "SKILL.md")); err != nil {
			t.Fatalf("collection not kept: %v", err)
		}
	}
	if data, err := os.ReadFile(filepath.Join(late, "note.txt")); err != nil || string(data) != "mine" {
		t.Fatalf("taken place changed: %q, %v", data, err)
	}
	names, _ := os.ReadDir(root)
	for _, e := range names {
		if strings.HasPrefix(e.Name(), ".") {
			t.Fatalf("left temporary %s", e.Name())
		}
	}
}

func TestLiftNestedRefusesChangedSkill(t *testing.T) {
	env := claudeTestEnv(t)
	root := filepath.Join(env.ClaudeDir, "skills")
	file := claudeWriteSkill(t, filepath.Join(root, "repo", "tool"), "tool", "Tool")
	summary, skill := fixTestNested(t, env, "repo")
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(file, later, later); err != nil {
		t.Fatal(err)
	}
	trash := NewTrash(filepath.Join(env.Home, ".quosparr"), env.Home)
	if _, err := trash.LiftNested(summary, skill, env.Now, trashTestNoSystem(t)); err == nil {
		t.Fatal("lifted a skill that changed since the scan")
	}
	if _, err := os.Stat(filepath.Join(root, "repo", "tool", "SKILL.md")); err != nil {
		t.Fatalf("source moved: %v", err)
	}
	trashTestMissing(t, filepath.Join(root, "tool"))
}

func TestInsertDescription(t *testing.T) {
	cases := []struct{ in, want string }{
		{"---\nname: a\n---\nbody\n", "---\nname: a\ndescription: \"Does <x> & \\\"y\\\"\"\n---\nbody\n"},
		{"---\r\nname: a\r\ndescription:\r\n---\r\n", "---\r\nname: a\r\ndescription: \"Does <x> & \\\"y\\\"\"\r\n---\r\n"},
		{"# Title\n", "---\nname: \"dir\"\ndescription: \"Does <x> & \\\"y\\\"\"\n---\n\n# Title\n"},
	}
	for _, c := range cases {
		got, err := insertDescription([]byte(c.in), "dir", `Does <x> & "y"`)
		if err != nil || string(got) != c.want {
			t.Fatalf("insert %q = %q, %v; want %q", c.in, got, err, c.want)
		}
		front, _, err := parseFrontmatter(got)
		if err != nil || front.field("description") != `Does <x> & "y"` {
			t.Fatalf("round trip %q = %q, %v", got, front.field("description"), err)
		}
	}
	for _, in := range []string{"---\ndescription: set\n---\n", "---\ndescription:\n  folded\n---\n", "---\nname: a\n"} {
		if _, err := insertDescription([]byte(in), "dir", "x"); err == nil {
			t.Fatalf("edited %q", in)
		}
	}
}

func TestAddDescriptionBacksUpAndLoads(t *testing.T) {
	env := claudeTestEnv(t)
	env.CodexHome = filepath.Join(env.Home, "codex-home")
	file := filepath.Join(env.CodexHome, "skills", "plain", "SKILL.md")
	claudeWriteFile(t, file, "---\nname: plain\n---\n\n# Plain\n\nFormats plain text.\n")
	summary, err := ScanCodex(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	var skill Skill
	for _, s := range summary.Skills {
		if s.File == file {
			skill = s
		}
	}
	if skill.Reason != ReasonMissingDesc {
		t.Fatalf("fixture not missing a description: %+v", skill)
	}
	if got := SuggestDescription(skill); got != "Formats plain text." {
		t.Fatalf("suggestion = %q", got)
	}
	backups := filepath.Join(env.Home, ".quosparr", "backups")
	policy := platform.BackupPolicy{Dir: backups, Prefix: "skill-md-", Ext: ".md", Keep: 10}
	if err := AddDescription(skill, "Formats plain text.", policy, env.Now); err != nil {
		t.Fatal(err)
	}
	after, err := ScanCodex(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range after.Skills {
		if s.File == file && (s.Description != "Formats plain text." || s.State != StateOn) {
			t.Fatalf("after fix = %+v", s)
		}
	}
	entries, err := os.ReadDir(backups)
	if err != nil || len(entries) != 1 {
		t.Fatalf("backups = %v, %v", entries, err)
	}
	if err := AddDescription(skill, "again", policy, env.Now); !errors.Is(err, ErrChanged) {
		t.Fatalf("second write with a stale scan = %v", err)
	}
}
