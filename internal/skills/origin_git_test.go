package skills

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestOriginGitCandidate(t *testing.T) {
	home := t.TempDir()
	repo := filepath.Join(home, "checkout")
	skillDir := filepath.Join(repo, "skills", "demo")
	writeOriginCoreTestFile(t, filepath.Join(skillDir, "SKILL.md"), "---\nname: demo\ndescription: test\n---\n")
	writeOriginCoreTestFile(t, filepath.Join(repo, ".git", "config"), `[remote "upstream"]
 url = "https://example.com/first.git" # note
[REMOTE "ORIGIN"]
 URL = "https://user:secret@github.com/acme/demo.git?token=hidden"
`)
	writeOriginCoreTestFile(t, filepath.Join(repo, ".git", "HEAD"), "ref: refs/heads/feature/origin\n")
	cache := newOriginGitCache(Env{Home: home})
	candidate := cache.candidate(Skill{RealFile: filepath.Join(skillDir, "SKILL.md"), Root: repo})
	if candidate.repo != "https://github.com/acme/demo" || candidate.branch != "feature/origin" || candidate.skillPath != "skills/demo" || candidate.installer != "git" {
		t.Fatalf("git candidate = %#v", candidate)
	}
}

func TestOriginGitWorktreeAndSymlinkMarker(t *testing.T) {
	home := t.TempDir()
	common := filepath.Join(home, "common")
	worktree := filepath.Join(home, "worktree")
	writeOriginCoreTestFile(t, filepath.Join(common, "config"), "[remote 'origin']\nurl = ssh://git@gitlab.com/group/repo.git\n")
	gitDir := filepath.Join(home, "gitdirs", "one")
	writeOriginCoreTestFile(t, filepath.Join(gitDir, "commondir"), filepath.ToSlash(filepath.Join("..", "..", "common"))+"\n")
	writeOriginCoreTestFile(t, filepath.Join(gitDir, "HEAD"), "5ad00d\n")
	writeOriginCoreTestFile(t, filepath.Join(worktree, ".git"), "gitdir: ../gitdirs/one\n")
	skill := filepath.Join(worktree, "demo", "SKILL.md")
	writeOriginCoreTestFile(t, skill, "---\nname: demo\ndescription: test\n---\n")
	candidate := newOriginGitCache(Env{Home: home}).candidate(Skill{RealFile: skill, Root: worktree})
	if candidate.repo != "https://gitlab.com/group/repo" || candidate.branch != "" || candidate.skillPath != "demo" {
		t.Fatalf("worktree candidate = %#v", candidate)
	}
	if runtime.GOOS == "windows" {
		return
	}
	outer := filepath.Join(home, "outer")
	writeOriginCoreTestFile(t, filepath.Join(outer, ".git", "config"), "[remote \"origin\"]\nurl=https://github.com/outer/repo\n")
	writeOriginCoreTestFile(t, filepath.Join(outer, ".git", "HEAD"), "ref: refs/heads/main\n")
	nested := filepath.Join(outer, "nested")
	if err := os.MkdirAll(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outer, ".git"), filepath.Join(nested, ".git")); err != nil {
		t.Fatal(err)
	}
	nestedSkill := filepath.Join(nested, "skill", "SKILL.md")
	writeOriginCoreTestFile(t, nestedSkill, "x")
	if got := newOriginGitCache(Env{Home: home}).candidate(Skill{RealFile: nestedSkill, Root: nested}); got != (originCandidate{}) {
		t.Fatalf("symlink marker used outer repo = %#v", got)
	}
}

func TestOriginRemoteURLWithoutOrigin(t *testing.T) {
	config := "[remote \"first\"]\nurl = \"https://example.com/o/r.git\" # note\n[remote \"second\"]\nurl=https://example.com/other\n"
	if got := originRemoteURL(config); got != "https://example.com/o/r.git" {
		t.Fatalf("first remote = %q", got)
	}
}

func TestOriginGitExternalEightLevelBoundary(t *testing.T) {
	home := t.TempDir()
	external := t.TempDir()
	writeOriginCoreTestFile(t, filepath.Join(external, ".git", "config"), "[remote \"origin\"]\nurl=https://github.com/acme/far\n")
	writeOriginCoreTestFile(t, filepath.Join(external, ".git", "HEAD"), "ref: refs/heads/main\n")
	dir := external
	for i := 0; i < 8; i++ {
		dir = filepath.Join(dir, "d")
	}
	file := filepath.Join(dir, "SKILL.md")
	writeOriginCoreTestFile(t, file, "x")
	if got := newOriginGitCache(Env{Home: home}).candidate(Skill{RealFile: file, Root: filepath.Join(home, "root")}); got != (originCandidate{}) {
		t.Fatalf("found repo beyond eight levels = %#v", got)
	}
}

func TestOriginGitCacheRespectsBoundaryAndCachesRepository(t *testing.T) {
	home := t.TempDir()
	external := t.TempDir()
	config := filepath.Join(external, ".git", "config")
	writeOriginCoreTestFile(t, config, "[remote \"origin\"]\nurl=https://github.com/acme/first\n")
	writeOriginCoreTestFile(t, filepath.Join(external, ".git", "HEAD"), "ref: refs/heads/main\n")
	first := filepath.Join(external, "skills", "one", "SKILL.md")
	second := filepath.Join(external, "skills", "two", "SKILL.md")
	writeOriginCoreTestFile(t, first, "x")
	writeOriginCoreTestFile(t, second, "x")
	cache := newOriginGitCache(Env{Home: home})
	if got := cache.candidate(Skill{RealFile: first, Root: filepath.Dir(first)}); got != (originCandidate{}) {
		t.Fatalf("narrow boundary escaped root = %#v", got)
	}
	if got := cache.candidate(Skill{RealFile: first, Root: filepath.Join(home, "unrelated")}); got.repo != "https://github.com/acme/first" {
		t.Fatalf("broader boundary reused negative = %#v", got)
	}
	writeOriginCoreTestFile(t, config, "[remote \"origin\"]\nurl=https://github.com/acme/second\n")
	if got := cache.candidate(Skill{RealFile: second, Root: filepath.Join(home, "unrelated")}); got.repo != "https://github.com/acme/first" {
		t.Fatalf("repository config was reread = %#v", got)
	}
}

func TestOriginGitDoesNotInspectHomeBoundary(t *testing.T) {
	home := t.TempDir()
	writeOriginCoreTestFile(t, filepath.Join(home, ".git", "config"), "[remote \"origin\"]\nurl=https://github.com/acme/home\n")
	writeOriginCoreTestFile(t, filepath.Join(home, ".git", "HEAD"), "ref: refs/heads/main\n")
	file := filepath.Join(home, "outside-root", "skill", "SKILL.md")
	writeOriginCoreTestFile(t, file, "x")
	if got := newOriginGitCache(Env{Home: home}).candidate(Skill{RealFile: file, Root: filepath.Join(home, "unrelated")}); got != (originCandidate{}) {
		t.Fatalf("home boundary repo = %#v", got)
	}
}
