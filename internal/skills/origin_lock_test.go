package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOriginLockXDGPriorityAndMatching(t *testing.T) {
	home := t.TempDir()
	state := filepath.Join(home, "state")
	env := Env{Home: home, Getenv: func(key string) string {
		if key == "XDG_STATE_HOME" {
			return state
		}
		return ""
	}}
	homeLock := `{"version":99,"skills":{"linked":{"sourceType":"github","source":"home/repo","version":"home"}}}`
	xdgLock := `{"skills":{"linked":{"sourceType":"github","source":"xdg/repo","skillPath":"skills/linked/SKILL.md","ref":"dev","version":"xdg","installedAt":"2026-01-02T03:04:05.123456789Z"}}}`
	writeOriginCoreTestFile(t, filepath.Join(home, ".agents", ".skill-lock.json"), homeLock)
	writeOriginCoreTestFile(t, filepath.Join(state, "skills", ".skill-lock.json"), xdgLock)
	cache := &originLockCache{env: env}
	wantTime, _ := time.Parse(time.RFC3339Nano, "2026-01-02T03:04:05.123456789Z")
	candidate := cache.candidate(Skill{Name: "front-name", Dir: filepath.Join(home, "skills", "linked"), Root: filepath.Join(home, "skills"), RealFile: filepath.Join(home, "real", "linked", "SKILL.md"), Scope: ScopeUser})
	if candidate.repo != "https://github.com/xdg/repo" || candidate.skillPath != "skills/linked" || candidate.branch != "dev" || candidate.version != "xdg" || candidate.installer != "skills" || !candidate.installedAt.Equal(wantTime) {
		t.Fatalf("lock candidate = %#v", candidate)
	}
	if got := cache.candidate(Skill{Name: "linked", Dir: filepath.Join(home, "linked"), Scope: ScopeSystem}); got != (originCandidate{}) {
		t.Fatalf("system lock candidate = %#v", got)
	}
}

func TestOriginLockFallbackAndManifestLimit(t *testing.T) {
	home := t.TempDir()
	lockPath := filepath.Join(home, ".agents", ".skill-lock.json")
	writeOriginCoreTestFile(t, lockPath, `{"skills":{"demo":{"sourceUrl":"not a URL","sourceType":"github","source":"owner/repo","skillPath":"SKILL.md"}}}`)
	cache := &originLockCache{env: Env{Home: home, Getenv: func(string) string { return "" }}}
	candidate := cache.candidate(Skill{Name: "demo", Dir: filepath.Join(home, "demo"), Root: home, Scope: ScopeUser})
	if candidate.repo != "https://github.com/owner/repo" || candidate.skillPath != "" {
		t.Fatalf("fallback candidate = %#v", candidate)
	}
	writeOriginCoreTestFile(t, lockPath, `{"skills":{"demo":{"sourceType":"github","source":"large/repo"}}`+strings.Repeat(" ", 70<<10)+`}`)
	cache = &originLockCache{env: cache.env}
	if got := cache.candidate(Skill{Name: "demo", Dir: filepath.Join(home, "demo"), Root: home, Scope: ScopeUser}); got.repo != "https://github.com/large/repo" {
		t.Fatalf("manifest over regular limit = %#v", got)
	}
	writeOriginCoreTestFile(t, lockPath, strings.Repeat("x", (1<<20)+1))
	cache = &originLockCache{env: cache.env}
	if got := cache.candidate(Skill{Name: "demo", Dir: filepath.Join(home, "demo"), Root: home, Scope: ScopeUser}); got != (originCandidate{}) {
		t.Fatalf("oversize lock = %#v", got)
	}
}

func TestOriginLockMatchesFoldersOnlyAndIgnoresBadData(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".agents", ".skill-lock.json")
	writeOriginCoreTestFile(t, path, `{"skills":{"real-dir":{"sourceType":"github","source":"real/repo","updatedAt":"not-a-time"},"front-name":{"sourceType":"github","source":"name/repo"}}}`)
	cache := &originLockCache{env: Env{Home: home, Getenv: func(string) string { return "" }}}
	// A link whose target is the CLI's copy in ~/.agents/skills.
	real := cache.candidate(Skill{Name: "front-name", Dir: filepath.Join(home, ".claude", "skills", "link-dir"), Root: filepath.Join(home, ".claude", "skills"),
		RealFile: filepath.Join(home, ".agents", "skills", "real-dir", "SKILL.md"), Scope: ScopeUser})
	if real.repo != "https://github.com/real/repo" || !real.updatedAt.IsZero() {
		t.Fatalf("real directory match = %#v", real)
	}
	// Only the name in its frontmatter matches: not installed by the CLI.
	byName := cache.candidate(Skill{Name: "front-name", Dir: filepath.Join(home, "skills", "mine"), Root: filepath.Join(home, "skills"),
		RealFile: filepath.Join(home, "skills", "mine", "SKILL.md"), Scope: ScopeUser})
	if byName != (originCandidate{}) {
		t.Fatalf("name-only match = %#v", byName)
	}
	// A lock folder name nested below a root is not an install either.
	nested := cache.candidate(Skill{Name: "x", Dir: filepath.Join(home, "skills", "group", "real-dir"), Root: filepath.Join(home, "skills"),
		RealFile: filepath.Join(home, "skills", "group", "real-dir", "SKILL.md"), Scope: ScopeUser})
	if nested != (originCandidate{}) {
		t.Fatalf("nested match = %#v", nested)
	}
	writeOriginCoreTestFile(t, path, "{bad json")
	cache = &originLockCache{env: cache.env}
	if got := cache.candidate(Skill{Name: "front-name", Dir: filepath.Join(home, "x"), Scope: ScopeUser}); got != (originCandidate{}) {
		t.Fatalf("bad JSON candidate = %#v", got)
	}
}

func TestOriginReadersRejectRegularSymlink(t *testing.T) {
	if err := os.Symlink("target", filepath.Join(t.TempDir(), "probe")); err != nil {
		t.Skip("symlink unavailable")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	writeOriginCoreTestFile(t, target, "ok")
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, err := readOriginRegular(link); err == nil {
		t.Fatal("regular reader followed symlink")
	}
	if data, err := readOriginManifest(link); err != nil || string(data) != "ok" {
		t.Fatalf("manifest reader = %q, %v", data, err)
	}
}

func writeOriginCoreTestFile(t *testing.T, path, value string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
		t.Fatal(err)
	}
}
