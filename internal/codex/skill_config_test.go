package codex

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func skillTarget(t *testing.T) (string, string) {
	t.Helper()
	return filepath.Join(t.TempDir(), "demo", "SKILL.md"), "demo"
}

func TestSetSkillConfigEmptyAndRoundTrip(t *testing.T) {
	path, name := skillTarget(t)
	for _, original := range []string{"", "answer = 42\n"} {
		enabled, err := SetSkillConfigEnabled([]byte(original), path, name, true)
		if err != nil || string(enabled) != original {
			t.Fatalf("enable changed default state: %v", err)
		}
		disabled, err := SetSkillConfigEnabled([]byte(original), path, name, false)
		if err != nil || !strings.Contains(string(disabled), "[[skills.config]]") || !strings.Contains(string(disabled), "enabled = false") {
			t.Fatalf("disable mismatch: %v", err)
		}
		again, err := SetSkillConfigEnabled(disabled, path, name, false)
		if err != nil || !bytes.Equal(again, disabled) {
			t.Fatalf("disable is not idempotent: %v", err)
		}
		restored, err := SetSkillConfigEnabled(disabled, path, name, true)
		if err != nil || string(restored) != original {
			t.Fatalf("round trip mismatch: %v", err)
		}
	}
}

func TestSetSkillConfigOrderedRules(t *testing.T) {
	path, name := skillTarget(t)
	tests := []struct {
		name    string
		in      string
		enabled bool
		wantEnd string
	}{
		{"name disables enable", "[[skills.config]]\nname = \"  " + name + "  \"\nenabled = false\n", true, "path = " + tomlQuote(path) + "\nenabled = true\n"},
		{"path then name disables enable", "[[skills.config]]\npath = " + tomlQuote(path) + "\nenabled = true\n\n[[skills.config]]\nname = \"" + name + "\"\nenabled = false\n", true, "path = " + tomlQuote(path) + "\nenabled = true\n"},
		{"name enables disable", "[[skills.config]]\npath = " + tomlQuote(path) + "\nenabled = true\n\n[[skills.config]]\nname = \"" + name + "\"\nenabled = true\n", false, "path = " + tomlQuote(path) + "\nenabled = false\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := SetSkillConfigEnabled([]byte(tt.in), path, name, tt.enabled)
			if err != nil || !strings.HasSuffix(string(out), tt.wantEnd) {
				t.Fatalf("ordered rule result mismatch: %v", err)
			}
			again, err := SetSkillConfigEnabled(out, path, name, tt.enabled)
			if err != nil || !bytes.Equal(again, out) {
				t.Fatalf("ordered rule not idempotent: %v", err)
			}
		})
	}
}

func TestSetSkillConfigEditsMatchingPathBlocks(t *testing.T) {
	path, name := skillTarget(t)
	dir := filepath.Dir(path)
	in := "# keep\r\n[skills.bundled]\r\nx = 1\r\n\r\n" +
		"[[skills.config]]\r\npath = " + tomlQuote(dir) + "\r\nenabled = true\r\nextra = 7\r\n\r\n" +
		"[[skills.config]]\r\npath = " + tomlQuote(path) + "\r\nenabled = true\r\n"
	disabled, err := SetSkillConfigEnabled([]byte(in), path, name, false)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Count(disabled, []byte("enabled = false\r\n")) != 2 || !bytes.Contains(disabled, []byte("# keep\r\n[skills.bundled]\r\nx = 1\r\n")) {
		t.Fatal("matching path blocks were not edited as expected")
	}
	enabled, err := SetSkillConfigEnabled(disabled, path, name, true)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(enabled, []byte("path = "+tomlQuote(path))) || !bytes.Contains(enabled, []byte("extra = 7\r\n")) || !bytes.Contains(enabled, []byte("enabled = true\r\n")) {
		t.Fatal("pure block was not deleted or extra block was lost")
	}
}

func TestSetSkillConfigIgnoredSelectors(t *testing.T) {
	path, name := skillTarget(t)
	other := filepath.Join(t.TempDir(), "SKILL.md")
	in := "[[skills.config]]\npath = " + tomlQuote(other) + "\nname = \"" + name + "\"\nenabled = false\nextra = \"keep\"\n\n" +
		"[[skills.config]]\nenabled = false\nother = \"keep\"\n\n" +
		"[[skills.config]]\npath = 4\nenabled = false\n"
	out, err := SetSkillConfigEnabled([]byte(in), path, name, true)
	if err != nil || string(out) != in {
		t.Fatalf("ignored entries changed: %v", err)
	}
}

func TestSetSkillConfigRejectsUnsafeForms(t *testing.T) {
	path, name := skillTarget(t)
	cases := []string{
		"skills = { config = [] }\n",
		"skills.config = []\n",
		"[skills]\nconfig = []\n",
		"[skills.config]\nenabled = true\n",
		"[[skills]]\nenabled = true\n",
		"[[skills.config]]\npath = \"x\"\nenabled = 1\n",
		"[[skills.config]]\npath = " + tomlQuote(path) + "\nenabled = true\n[skills.config.extra]\nx = 1\n",
	}
	for i, in := range cases {
		if _, err := SetSkillConfigEnabled([]byte(in), path, name, false); !errors.Is(err, ErrSkillConfigUnsupported) {
			t.Fatalf("case %d: err = %v", i, err)
		}
	}
	if _, err := SetSkillConfigEnabled([]byte("[broken\n"), path, name, false); !errors.Is(err, ErrProviderConfigInvalid) {
		t.Fatalf("invalid TOML err = %v", err)
	}
}

func TestSetSkillConfigCRLFAppendRoundTrip(t *testing.T) {
	path, name := skillTarget(t)
	original := "# top\r\n[skills]\r\nother = true\r\n\r\n[unrelated]\r\n# keep\r\nvalue = 7\r\n"
	want := original + "\r\n[[skills.config]]\r\npath = " + tomlQuote(path) + "\r\nenabled = false\r\n"
	disabled, err := SetSkillConfigEnabled([]byte(original), path, name, false)
	if err != nil || string(disabled) != want {
		t.Fatalf("CRLF append mismatch: %v", err)
	}
	restored, err := SetSkillConfigEnabled(disabled, path, name, true)
	if err != nil || string(restored) != original {
		t.Fatalf("CRLF round trip mismatch: %v", err)
	}
}

func TestSetSkillConfigPathOnlyRule(t *testing.T) {
	path, name := skillTarget(t)
	in := "[[skills.config]]\npath = " + tomlQuote(path) + "\n"
	out, err := SetSkillConfigEnabled([]byte(in), path, name, true)
	if err != nil || len(out) != 0 {
		t.Fatalf("path-only enable mismatch: %v", err)
	}
	out, err = SetSkillConfigEnabled([]byte(in), path, name, false)
	if err != nil || !bytes.Contains(out, []byte("enabled = false\n")) {
		t.Fatalf("path-only disable mismatch: %v", err)
	}
}

func TestSetSkillConfigRejectsEditedMultilineAndAllowsUnrelated(t *testing.T) {
	path, name := skillTarget(t)
	matched := "[[skills.config]]\npath = \"\"\"" + path + "\"\"\"\nenabled = true\n"
	if _, err := SetSkillConfigEnabled([]byte(matched), path, name, false); !errors.Is(err, ErrSkillConfigUnsupported) {
		t.Fatalf("matched multiline err = %v", err)
	}
	unrelated := "note = \"\"\"multi\nline\"\"\"\n"
	if _, err := SetSkillConfigEnabled([]byte(unrelated), path, name, false); err != nil {
		t.Fatalf("unrelated multiline: %v", err)
	}
}

func TestSetSkillConfigTargetValidation(t *testing.T) {
	path, _ := skillTarget(t)
	for _, tc := range []struct{ path, name string }{{"relative/SKILL.md", "x"}, {filepath.Dir(path), "x"}, {path, " "}, {path, "\nname"}} {
		if _, err := SetSkillConfigEnabled(nil, tc.path, tc.name, false); !errors.Is(err, ErrSkillConfigTarget) {
			t.Fatalf("target %#v: %v", tc, err)
		}
	}
}

func TestStoreSetSkillEnabledFileBackupAndNoop(t *testing.T) {
	dir := t.TempDir()
	codexHome, runwayHome := filepath.Join(dir, "codex"), filepath.Join(dir, "runway")
	store := NewStore(codexHome, runwayHome)
	path := filepath.Join(dir, "skill", "SKILL.md")
	now := time.Unix(1_800_000_000, 0)
	if err := store.SetSkillEnabledFile(path, "demo", true, now); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(store.ConfigPath()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("enable created missing config: %v", err)
	}
	if err := store.SetSkillEnabledFile(path, "demo", false, now); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(store.ConfigPath())
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		info, _ := os.Stat(store.ConfigPath())
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("new mode = %v", info.Mode().Perm())
		}
	}
	beforeInfo, err := os.Stat(store.ConfigPath())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetSkillEnabledFile(path, "demo", false, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	afterInfo, err := os.Stat(store.ConfigPath())
	if err != nil || !os.SameFile(beforeInfo, afterInfo) || !beforeInfo.ModTime().Equal(afterInfo.ModTime()) {
		t.Fatalf("no-op replaced config: %v", err)
	}
	if entries, err := os.ReadDir(filepath.Join(runwayHome, "backups")); !errors.Is(err, os.ErrNotExist) && (err != nil || len(entries) != 0) {
		t.Fatalf("no-op backups = %v, %v", entries, err)
	}
	if err := os.Chmod(store.ConfigPath(), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := store.SetSkillEnabledFile(path, "demo", true, now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(runwayHome, "backups"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("backups = %v, %v", entries, err)
	}
	backup, _ := os.ReadFile(filepath.Join(runwayHome, "backups", entries[0].Name()))
	if !bytes.Equal(backup, original) {
		t.Fatalf("backup differs")
	}
	if runtime.GOOS != "windows" {
		info, _ := os.Stat(store.ConfigPath())
		backupInfo, _ := entries[0].Info()
		if info.Mode().Perm() != 0o640 || backupInfo.Mode().Perm() != 0o600 {
			t.Fatalf("modes = %v, %v", info.Mode().Perm(), backupInfo.Mode().Perm())
		}
	}
}

func TestStoreSetSkillEnabledFileErrorWritesNothing(t *testing.T) {
	tests := []struct {
		name string
		data string
		want error
	}{
		{"invalid", "secret-marker = \"hidden\" oops\n", ErrProviderConfigInvalid},
		{"unsupported", "skills = { config = [], marker = \"hidden\" }\n", ErrSkillConfigUnsupported},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			store := NewStore(filepath.Join(dir, "codex"), filepath.Join(dir, "runway"))
			if err := os.MkdirAll(filepath.Dir(store.ConfigPath()), 0o700); err != nil {
				t.Fatal(err)
			}
			original := []byte(tt.data)
			if err := os.WriteFile(store.ConfigPath(), original, 0o600); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "skill", "SKILL.md")
			err := store.SetSkillEnabledFile(path, "demo", false, time.Unix(1_800_000_000, 0))
			if !errors.Is(err, tt.want) || strings.Contains(err.Error(), "hidden") {
				t.Fatalf("unexpected safe error: %v", err)
			}
			got, readErr := os.ReadFile(store.ConfigPath())
			if readErr != nil || !bytes.Equal(got, original) {
				t.Fatalf("failed edit changed file: %v", readErr)
			}
			if _, err := os.Stat(filepath.Join(dir, "runway", "backups")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("failed edit created backups: %v", err)
			}
		})
	}
}
