package skills

import (
	"strings"
	"testing"
)

func TestNormalizeRepoURL(t *testing.T) {
	tests := []struct {
		raw, want, github string
		ok                bool
	}{
		{" git@github.com:Owner/Repo.git ", "https://github.com/Owner/Repo", "Owner/Repo", true},
		{"ssh://token@GitHub.com:2222/owner/repo.git?secret=yes#x", "https://github.com/owner/repo", "owner/repo", true},
		{"git+https://user:pass@gitlab.com/group/sub/repo/-/tree/main/skill", "https://gitlab.com/group/sub/repo", "", true},
		{"git://example.com/team/repo.git/", "https://example.com/team/repo", "", true},
		{"http://www.github.com/owner/repo/tree/main/skills/demo", "https://github.com/owner/repo", "owner/repo", true},
		{"https://user:token@example.com:8443/team/repo?token=secret#fragment", "https://example.com:8443/team/repo", "", true},
		{"file:///tmp/repo", "", "", false},
		{"/tmp/repo", "", "", false},
		{"./repo", "", "", false},
		{"../repo", "", "", false},
		{"~/repo", "", "", false},
		{`C:\repo`, "", "", false},
		{"https://localhost/repo", "", "", false},
		{"https://github.com/owner//repo", "", "", false},
		{"https://github.com/owner", "", "", false},
		{"https://example.com", "", "", false},
		{"ssh://git@example.com:22", "", "", false},
		{"git@example.com:22/repo", "", "", false},
	}
	for _, test := range tests {
		got, github, ok := NormalizeRepoURL(test.raw)
		if got != test.want || github != test.github || ok != test.ok {
			t.Errorf("NormalizeRepoURL(%q) = %q, %q, %v", test.raw, got, github, ok)
		}
		if strings.Contains(got, "user") || strings.Contains(got, "token") || strings.Contains(got, "secret") {
			t.Errorf("NormalizeRepoURL(%q) leaked credentials in %q", test.raw, got)
		}
	}
}

func TestOriginURLHelpers(t *testing.T) {
	for _, test := range []struct{ raw, repo, ref, path string }{
		{"https://github.com/o/r/tree/main/skills/demo", "https://github.com/o/r", "main", "skills/demo"},
		{"https://gitlab.com/g/sub/r/-/tree/v1/plugins/demo", "https://gitlab.com/g/sub/r", "v1", "plugins/demo"},
	} {
		repo, ref, skillPath := parseRepoTreeURL(test.raw)
		if repo != test.repo || ref != test.ref || skillPath != test.path {
			t.Errorf("parseRepoTreeURL(%q) = %q, %q, %q", test.raw, repo, ref, skillPath)
		}
	}
	if repo, ref, skillPath := parseRepoTreeURL("https://github.com/o/r/tree/HEAD/a/../x"); repo != "" || ref != "" || skillPath != "" {
		t.Fatalf("invalid tree = %q, %q, %q", repo, ref, skillPath)
	}
	if repo, _, ok := NormalizeRepoURL("https://github.com/o/r/tree/HEAD/a/../x"); !ok || repo != "https://github.com/o/r" {
		t.Fatalf("invalid tree repository = %q, %v", repo, ok)
	}
	if got := normalizeHomepage("http://user:pass@OpenAI.com/?x=secret#fragment"); got != "https://openai.com" {
		t.Fatalf("homepage = %q", got)
	}
	if got := normalizeHomepage("https://user:token@example.com:8443/docs/?secret=yes#token"); got != "https://example.com:8443/docs" || strings.Contains(got, "token") || strings.Contains(got, "secret") {
		t.Fatalf("credential homepage = %q", got)
	}
	for _, raw := range []string{"git://example.com/repo", "ssh://example.com/repo", "https://localhost/path"} {
		if got := normalizeHomepage(raw); got != "" {
			t.Errorf("normalizeHomepage(%q) = %q", raw, got)
		}
	}
	for raw, want := range map[string]string{"./plugins/codex/": "plugins/codex", `skills\demo`: "skills/demo", "a/../secret": ""} {
		if got := cleanOriginSkillPath(raw); got != want {
			t.Errorf("cleanOriginSkillPath(%q) = %q", raw, got)
		}
	}
	if got := originFolder("https://github.com/o/r", "", "skills/demo"); got != "https://github.com/o/r/tree/HEAD/skills/demo" {
		t.Fatalf("GitHub folder = %q", got)
	}
	if got := originFolder("https://gitlab.com/g/r", "main", "skills/demo"); got != "https://gitlab.com/g/r/-/tree/main/skills/demo" {
		t.Fatalf("GitLab folder = %q", got)
	}
}
