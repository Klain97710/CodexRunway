package skills

import (
	"testing"
	"time"
)

func TestMergeOriginCandidates(t *testing.T) {
	installed := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	updated := installed.Add(time.Hour)
	lock := originCandidate{from: OriginLock, repo: "https://github.com/acme/main", ref: "v2", skillPath: "skills/demo", installer: "skills"}
	git := originCandidate{from: OriginGit, repo: "https://github.com/acme/git", branch: "main", installedAt: installed}
	plugin := originCandidate{from: OriginPlugin, repo: "https://github.com/acme/plugin", homepage: "https://docs.example.com/demo/", version: "2.0", updatedAt: updated}
	front := originCandidate{from: OriginFrontmatter, homepage: "https://front.example.com", version: "3.0"}
	got := mergeOriginCandidates(lock, git, plugin, front)
	if got.From != OriginLock || got.Repository != lock.repo || got.GitHub != "acme/main" || got.SkillPath != "skills/demo" || got.Folder != lock.repo+"/tree/v2/skills/demo" || got.Branch != "" || got.Installer != "skills" {
		t.Fatalf("repository fields = %#v", got)
	}
	if got.Homepage != "https://docs.example.com/demo" || got.Version != "2.0" || !got.InstalledAt.Equal(installed) || !got.UpdatedAt.Equal(updated) {
		t.Fatalf("merged metadata = %#v", got)
	}
	if lock.repo != "https://github.com/acme/main" {
		t.Fatal("merge mutated input")
	}
}

func TestMergeOriginCandidatesWithoutRepository(t *testing.T) {
	candidates := []originCandidate{{from: OriginFrontmatter, homepage: "https://github.com/acme/demo/tree/main", version: "1"}}
	got := mergeOriginCandidates(candidates...)
	if got.Repository != "https://github.com/acme/demo" || got.GitHub != "acme/demo" || got.Homepage != "" || got.From != OriginFrontmatter || got.Version != "1" {
		t.Fatalf("promoted homepage = %#v", got)
	}
	if candidates[0].homepage != "https://github.com/acme/demo/tree/main" || candidates[0].repo != "" {
		t.Fatalf("promotion mutated input = %#v", candidates[0])
	}
	got = mergeOriginCandidates(originCandidate{from: OriginFrontmatter, version: "1"})
	if got.From != OriginNone || got.Version != "1" || got.Repository != "" {
		t.Fatalf("version-only origin = %#v", got)
	}
	if got := mergeOriginCandidates(originCandidate{from: OriginLock, installedAt: time.Now()}); got != (Origin{}) {
		t.Fatalf("time-only origin = %#v", got)
	}
	got = mergeOriginCandidates(
		originCandidate{from: OriginLock, installedAt: time.Now()},
		originCandidate{from: OriginFrontmatter, homepage: "https://docs.example.com", version: "1"},
	)
	if got.Repository != "" || got.Homepage != "https://docs.example.com" || got.Version != "1" || !got.InstalledAt.IsZero() || !got.UpdatedAt.IsZero() {
		t.Fatalf("homepage-only origin = %#v", got)
	}
}

func TestMergeOriginRepositoryPriorityFallback(t *testing.T) {
	lock := originCandidate{from: OriginLock, repo: "https://github.com/acme/lock"}
	git := originCandidate{from: OriginGit, repo: "https://github.com/acme/git"}
	plugin := originCandidate{from: OriginPlugin, repo: "https://github.com/acme/plugin"}
	front := originCandidate{from: OriginFrontmatter, repo: "https://github.com/acme/front"}
	for _, test := range []struct {
		candidates []originCandidate
		want       OriginFrom
	}{
		{[]originCandidate{lock, git, plugin, front}, OriginLock},
		{[]originCandidate{{}, git, plugin, front}, OriginGit},
		{[]originCandidate{{}, {}, plugin, front}, OriginPlugin},
		{[]originCandidate{{}, {}, {}, front}, OriginFrontmatter},
	} {
		if got := mergeOriginCandidates(test.candidates...); got.From != test.want {
			t.Errorf("priority = %#v, want %s", got, test.want)
		}
	}
	got := mergeOriginCandidates(
		originCandidate{from: OriginLock, version: "lock-only"},
		originCandidate{from: OriginGit, repo: git.repo, homepage: git.repo},
		originCandidate{from: OriginPlugin, homepage: "https://docs.example.com"},
	)
	if got.From != OriginGit || got.Homepage != "https://docs.example.com" || got.Version != "lock-only" {
		t.Fatalf("labels = %#v", got)
	}
}

func TestMergeOriginKeepsFrontHomepageWhenGitRepositoryWins(t *testing.T) {
	front := frontmatterOrigin(frontmatter{values: map[string]string{"homepage": "https://github.com/docs/site/tree/main"}, metadata: map[string]string{}, nested: map[string]map[string]string{}})
	got := mergeOriginCandidates(
		originCandidate{},
		originCandidate{from: OriginGit, repo: "https://github.com/acme/repo"},
		originCandidate{},
		candidateFromOrigin(front),
	)
	if got.Repository != "https://github.com/acme/repo" || got.Homepage != "https://github.com/docs/site/tree/main" || got.From != OriginGit {
		t.Fatalf("git with front homepage = %#v", got)
	}
}

func TestMergeOriginKeepsExplicitRepositoryWikiHomepage(t *testing.T) {
	got := mergeOriginCandidates(originCandidate{
		from: OriginFrontmatter,
		repo: "https://github.com/acme/repo", homepage: "https://github.com/acme/repo/wiki",
	})
	if got.Repository != "https://github.com/acme/repo" || got.Homepage != "https://github.com/acme/repo/wiki" {
		t.Fatalf("repository wiki homepage = %#v", got)
	}
	got = mergeOriginCandidates(originCandidate{from: OriginFrontmatter, homepage: "https://github.com/acme/repo/tree/main"})
	if got.Repository != "https://github.com/acme/repo" || got.Homepage != "" {
		t.Fatalf("promoted tree homepage = %#v", got)
	}
}

func TestFrontmatterOriginPriority(t *testing.T) {
	data := []byte("---\nrepository: https://github.com/top/repo/tree/dev/skills/demo\nhomepage: https://docs.example.com/\nversion: v3\nmetadata:\n  repository: https://github.com/meta/repo\n  homepage: https://meta.example.com\n  version: v2\n---\n")
	front, _, err := parseFrontmatter(data)
	if err != nil {
		t.Fatal(err)
	}
	got := frontmatterOrigin(front)
	if got.Repository != "https://github.com/top/repo" || got.SkillPath != "" || got.Branch != "" || got.Homepage != "https://docs.example.com" || got.Version != "v3" {
		t.Fatalf("frontmatter origin = %#v", got)
	}
	nested, _, err := parseFrontmatter([]byte("---\nrepository:\n  url: https://gitlab.com/g/r\nurl: https://site.example.com\nmetadata:\n  repo: https://github.com/meta/repo\n---\n"))
	if err != nil || frontmatterOrigin(nested).Repository != "https://gitlab.com/g/r" {
		t.Fatalf("nested repository = %#v, %v", frontmatterOrigin(nested), err)
	}
}

func TestFrontmatterOriginAliasesAndInvalidRepository(t *testing.T) {
	for _, test := range []struct {
		body, repo, homepage string
	}{
		{"repo: https://github.com/a/repo", "https://github.com/a/repo", ""},
		{"source: https://gitlab.com/g/repo", "https://gitlab.com/g/repo", ""},
		{"website: http://docs.example.com/", "", "https://docs.example.com"},
		{"metadata:\n  repository: https://github.com/m/repo\n  url: https://meta.example.com", "https://github.com/m/repo", "https://meta.example.com"},
		{"repository: owner/repo\nhomepage: invalid", "", ""},
	} {
		front, _, err := parseFrontmatter([]byte("---\n" + test.body + "\n---\n"))
		if err != nil {
			t.Fatal(err)
		}
		got := frontmatterOrigin(front)
		if got.Repository != test.repo || got.Homepage != test.homepage {
			t.Errorf("frontmatter %q = %#v", test.body, got)
		}
	}
}
