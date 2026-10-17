package skills

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestNormalizeEnvAndRootLabel(t *testing.T) {
	if _, err := normalizeEnv(Env{}); err == nil {
		t.Fatal("missing Home was accepted")
	}
	home := t.TempDir()
	env, err := normalizeEnv(Env{Home: home})
	if err != nil {
		t.Fatal(err)
	}
	if env.CodexHome != filepath.Join(home, ".codex") || env.ClaudeDir != filepath.Join(home, ".claude") || env.GrokHome != filepath.Join(home, ".grok") || env.GOOS == "" || env.Now.IsZero() {
		t.Fatalf("defaults = %#v", env)
	}
	root := newRoot(env, filepath.Join(home, ".agents", "skills"), ScopeUser)
	if root.Label != filepath.Join("~", ".agents", "skills") || root.Exists {
		t.Fatalf("root = %#v", root)
	}
}

func TestScanRootLimitsHiddenAndContext(t *testing.T) {
	home := t.TempDir()
	env, _ := normalizeEnv(Env{Home: home})
	rootPath := filepath.Join(home, "skills")
	commonSkill(t, filepath.Join(rootPath, "visible"), "visible")
	commonSkill(t, filepath.Join(rootPath, ".hidden"), "hidden")
	commonSkill(t, filepath.Join(rootPath, "one", "two", "three"), "deep")
	root := newRoot(env, rootPath, ScopeUser)
	skills, issues, err := scanRoot(context.Background(), env, ProviderCodex, root, walkOptions{MaxDepth: 1, SkipHidden: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 || skills[0].Name != "visible" || !hasIssue(issues, IssueTruncated) {
		t.Fatalf("limited scan = %#v, %#v", skills, issues)
	}

	_, issues, err = scanRoot(context.Background(), env, ProviderCodex, root, walkOptions{MaxEntries: 1})
	if err != nil || !hasIssue(issues, IssueTruncated) {
		t.Fatalf("entry limit = %#v, %v", issues, err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := scanRoot(canceled, env, ProviderCodex, root, walkOptions{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled scan error = %v", err)
	}
}

func TestScanRootSymlinksLoopBrokenAndSystemPolicy(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires privileges on Windows")
	}
	home := t.TempDir()
	env, _ := normalizeEnv(Env{Home: home})
	rootPath := filepath.Join(home, "skills")
	target := filepath.Join(home, "target")
	commonSkill(t, filepath.Join(target, "linked"), "linked")
	if err := os.MkdirAll(rootPath, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(rootPath, "collection")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(rootPath, filepath.Join(target, "loop")); err != nil {
		t.Fatal(err)
	}
	broken := filepath.Join(rootPath, "broken")
	if err := os.Symlink(filepath.Join(home, "missing"), broken); err != nil {
		t.Fatal(err)
	}
	fileTarget := filepath.Join(home, "single", "SKILL.md")
	commonSkill(t, filepath.Dir(fileTarget), "file-link")
	fileLinkDir := filepath.Join(rootPath, "file-link")
	if err := os.MkdirAll(fileLinkDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(fileTarget, filepath.Join(fileLinkDir, "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	root := newRoot(env, rootPath, ScopeUser)
	skills, issues, err := scanRoot(context.Background(), env, ProviderCodex, root, walkOptions{FollowLinks: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 2 || skills[0].LinkPath != link || skills[0].LinkTarget != target || !hasIssue(issues, IssueBrokenLink) {
		t.Fatalf("linked scan = %#v, %#v", skills, issues)
	}
	skills, _, err = scanRoot(context.Background(), env, ProviderCodex, RootInfo{Path: rootPath, Scope: ScopeSystem, Exists: true}, walkOptions{FollowLinks: false})
	if err != nil || len(skills) != 1 || skills[0].Name != "file-link" {
		t.Fatalf("system followed links: %#v, %v", skills, err)
	}
}

func TestScanSymlinkRootRemovesOnlyLink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires privileges on Windows")
	}
	home := t.TempDir()
	env, _ := normalizeEnv(Env{Home: home})
	target := filepath.Join(home, "target")
	commonSkill(t, filepath.Join(target, "one"), "one")
	rootPath := filepath.Join(home, "linked-skills")
	if err := os.Symlink(target, rootPath); err != nil {
		t.Fatal(err)
	}
	root := newRoot(env, rootPath, ScopeUser)
	skills, issues, err := scanRoot(context.Background(), env, ProviderCodex, root, walkOptions{FollowLinks: true})
	if err != nil || len(issues) != 0 || len(skills) != 1 {
		t.Fatalf("root link scan = %#v, %#v, %v", skills, issues, err)
	}
	summary := Summary{Skills: skills, Roots: []RootInfo{root}}
	finishSummary(&summary)
	if !summary.Skills[0].Deletable || !summary.Skills[0].Removal.IsLink || summary.Skills[0].Removal.Path != rootPath {
		t.Fatalf("root link removal = %#v", summary.Skills[0].Removal)
	}
}

func TestSingleFileRootIsNotDeletable(t *testing.T) {
	home := t.TempDir()
	env, _ := normalizeEnv(Env{Home: home})
	file := filepath.Join(home, "standalone", "SKILL.md")
	commonSkill(t, filepath.Dir(file), "standalone")
	root := newRoot(env, file, ScopeUser)
	skills, issues, err := scanRoot(context.Background(), env, ProviderCodex, root, walkOptions{})
	if err != nil || len(issues) != 0 || len(skills) != 1 {
		t.Fatalf("single file scan = %#v, %#v, %v", skills, issues, err)
	}
	summary := Summary{Skills: skills, Roots: []RootInfo{root}}
	finishSummary(&summary)
	if summary.Skills[0].Deletable {
		t.Fatalf("single-file root removal = %#v", summary.Skills[0].Removal)
	}
}

func TestScanRootDirectoryAndEntryLimits(t *testing.T) {
	home := t.TempDir()
	env, _ := normalizeEnv(Env{Home: home})
	rootPath := filepath.Join(home, "skills")
	commonSkill(t, filepath.Join(rootPath, "one"), "one")
	commonSkill(t, filepath.Join(rootPath, "two"), "two")
	root := newRoot(env, rootPath, ScopeUser)
	_, issues, err := scanRoot(context.Background(), env, ProviderCodex, root, walkOptions{MaxDirs: 1})
	if err != nil || !hasIssue(issues, IssueTruncated) {
		t.Fatalf("directory limit = %#v, %v", issues, err)
	}
}

func TestScanRootDirectChildrenIgnoresOtherLevels(t *testing.T) {
	home := t.TempDir()
	env, _ := normalizeEnv(Env{Home: home})
	rootPath := filepath.Join(home, "plugins")
	commonWriteFile(t, filepath.Join(rootPath, "SKILL.md"), "---\ninvalid root frontmatter\n---\n")
	commonSkill(t, filepath.Join(rootPath, "plugin"), "plugin")
	commonWriteFile(t, filepath.Join(rootPath, "plugin", "scripts", "SKILL.md"), "---\ninvalid scripts frontmatter\n---\n")
	commonWriteFile(t, filepath.Join(rootPath, "plugin", "references", "SKILL.md"), "---\ninvalid references frontmatter\n---\n")
	commonWriteFile(t, filepath.Join(rootPath, "without-skill", "deep", "SKILL.md"), "---\ninvalid deep frontmatter\n---\n")
	root := newRoot(env, rootPath, ScopePlugin)
	skills, issues, err := scanRoot(context.Background(), env, ProviderCodex, root, walkOptions{DirectChildren: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 || skills[0].Name != "plugin" || len(issues) != 0 {
		t.Fatalf("direct children scan = %#v, %#v", skills, issues)
	}
}

func TestScanRootDirectChildrenKeepsDirectoryLimit(t *testing.T) {
	home := t.TempDir()
	env, _ := normalizeEnv(Env{Home: home})
	rootPath := filepath.Join(home, "plugins")
	if err := os.MkdirAll(rootPath, 0o700); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2001; i++ {
		if err := os.Mkdir(filepath.Join(rootPath, fmt.Sprintf("plugin-%04d", i)), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	root := newRoot(env, rootPath, ScopePlugin)
	_, issues, err := scanRoot(context.Background(), env, ProviderCodex, root, walkOptions{DirectChildren: true})
	if err != nil {
		t.Fatal(err)
	}
	if !hasIssue(issues, IssueTruncated) {
		t.Fatalf("directory limit issue missing: %#v", issues)
	}
}

func TestScanRootDirectChildrenDirectoryLinkPolicy(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires privileges on Windows")
	}
	home := t.TempDir()
	env, _ := normalizeEnv(Env{Home: home})
	rootPath := filepath.Join(home, "plugins")
	target := filepath.Join(home, "target-plugin")
	commonSkill(t, target, "linked-plugin")
	if err := os.MkdirAll(rootPath, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(rootPath, "linked")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	root := newRoot(env, rootPath, ScopePlugin)
	skills, _, err := scanRoot(context.Background(), env, ProviderCodex, root, walkOptions{DirectChildren: true})
	if err != nil || len(skills) != 0 {
		t.Fatalf("direct scan followed disabled link: %#v, %v", skills, err)
	}
	skills, _, err = scanRoot(context.Background(), env, ProviderCodex, root, walkOptions{DirectChildren: true, FollowLinks: true})
	if err != nil || len(skills) != 1 || skills[0].LinkPath != link {
		t.Fatalf("direct scan did not follow enabled link: %#v, %v", skills, err)
	}
}

func TestScanRootSkipPathsAvoidsReadingSubtree(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires privileges on Windows")
	}
	home := t.TempDir()
	env, _ := normalizeEnv(Env{Home: home})
	rootPath := filepath.Join(home, "skills")
	skipped := filepath.Join(rootPath, "skipped")
	credential := filepath.Join(home, "auth.json")
	commonWriteFile(t, credential, "private-content")
	if err := os.MkdirAll(skipped, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(credential, filepath.Join(skipped, "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	commonSkill(t, filepath.Join(rootPath, "normal"), "normal")
	root := newRoot(env, rootPath, ScopeUser)
	for _, opts := range []walkOptions{
		{FollowLinks: true, SkipPaths: []string{filepath.Join(skipped, ".")}},
		{DirectChildren: true, FollowLinks: true, SkipPaths: []string{filepath.Join(skipped, ".")}},
	} {
		skills, issues, err := scanRoot(context.Background(), env, ProviderCodex, root, opts)
		if err != nil || len(issues) != 0 || len(skills) != 1 || skills[0].Name != "normal" {
			t.Fatalf("skip paths scan = %#v, %#v, %v", skills, issues, err)
		}
	}

	outside := filepath.Join(rootPath, "outside")
	if err := os.MkdirAll(outside, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(credential, filepath.Join(outside, "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := scanRoot(context.Background(), env, ProviderCodex, root, walkOptions{FollowLinks: true, SkipPaths: []string{skipped}}); !errors.Is(err, errNotSkillFile) {
		t.Fatalf("outside error was swallowed: %v", err)
	}
}

func TestFinishSummaryCountsAndDuplicateOrder(t *testing.T) {
	home := t.TempDir()
	rootA := filepath.Join(home, "a")
	rootB := filepath.Join(home, "b")
	summary := Summary{Provider: ProviderCodex, Roots: []RootInfo{{Path: rootA}, {Path: rootB}}, Skills: []Skill{
		{Provider: ProviderCodex, InvokeName: "same", File: filepath.Join(rootA, "one", "SKILL.md"), RealFile: filepath.Join(rootA, "one", "SKILL.md"), Root: rootA},
		{Provider: ProviderCodex, InvokeName: "same", File: filepath.Join(rootB, "two", "SKILL.md"), RealFile: filepath.Join(rootB, "two", "SKILL.md"), Root: rootB},
	}}
	finishSummary(&summary)
	if summary.Roots[0].Count != 1 || summary.Roots[1].Count != 1 || len(summary.Issues) != 1 || summary.Issues[0].Detail != summary.Skills[0].File || summary.Skills[0].File == summary.Skills[1].File {
		t.Fatalf("summary = %#v", summary)
	}
}

func commonSkill(t *testing.T, dir, name string) {
	t.Helper()
	commonWriteFile(t, filepath.Join(dir, "SKILL.md"), "---\nname: "+name+"\ndescription: test\n---\nbody\n")
}

func hasIssue(issues []Issue, kind IssueKind) bool {
	for _, issue := range issues {
		if issue.Kind == kind {
			return true
		}
	}
	return false
}
