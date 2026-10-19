package grok

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestSetSkillDisabled(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		skill    string
		disabled bool
		want     string
	}{
		{name: "empty", skill: "review", disabled: true, want: "[skills]\ndisabled = [\"review\"]\n"},
		{name: "append block", input: "model = \"grok\"\n", skill: "review", disabled: true, want: "model = \"grok\"\n\n[skills]\ndisabled = [\"review\"]\n"},
		{name: "append after implicit parent", input: "[skills.team]\nvalue = 1\n", skill: "review", disabled: true, want: "[skills.team]\nvalue = 1\n\n[skills]\ndisabled = [\"review\"]\n"},
		{name: "insert before trailing trivia", input: "[skills]\npaths = [\"~/x\"]\n\n# keep\n[other]\nx = 1\n", skill: "review", disabled: true, want: "[skills]\npaths = [\"~/x\"]\ndisabled = [\"review\"]\n\n# keep\n[other]\nx = 1\n"},
		{name: "insert after header", input: "[skills]\n# keep\n", skill: "review", disabled: true, want: "[skills]\ndisabled = [\"review\"]\n# keep\n"},
		{name: "crlf", input: "[skills]\r\npaths = []\r\n", skill: "review", disabled: true, want: "[skills]\r\npaths = []\r\ndisabled = [\"review\"]\r\n"},
		{name: "rewrite multiline", input: "[skills]\ndisabled = [\n  \"one\",\n  \"two\",\n]\n", skill: "three", disabled: true, want: "[skills]\ndisabled = [\"one\", \"two\", \"three\"]\n"},
		{name: "dedupe disable", input: "[skills]\ndisabled = [\"one\", \"one\"] # untouched\n", skill: "one", disabled: true, want: "[skills]\ndisabled = [\"one\", \"one\"] # untouched\n"},
		{name: "enable removes all", input: "[skills]\ndisabled = [\"one\", \"two\", \"one\"]\n", skill: "one", want: "[skills]\ndisabled = [\"two\"]\n"},
		{name: "enable absent exact no-op", input: "[skills]\ndisabled = [\"one\"] # untouched\n", skill: "two", want: "[skills]\ndisabled = [\"one\"] # untouched\n"},
		{name: "remove empty table at end", input: "model = \"grok\"\n\n[skills]\ndisabled = [\"one\"]\n", skill: "one", want: "model = \"grok\"\n"},
		{name: "remove explicit parent after subtable", input: "[skills.team]\nvalue = 1\n\n[skills]\ndisabled = [\"one\"]\n", skill: "one", want: "[skills.team]\nvalue = 1\n"},
		{name: "retain comment only table", input: "[skills]\ndisabled = [\"one\"]\n# keep\n", skill: "one", want: "[skills]\n# keep\n"},
		{name: "retain header comment", input: "[skills] # keep\ndisabled = [\"one\"]\n", skill: "one", want: "[skills] # keep\n"},
		{name: "retain other key", input: "[skills]\npaths = []\ndisabled = [\"one\"]\n", skill: "one", want: "[skills]\npaths = []\n"},
		{name: "preserve subtable disabled", input: "[skills.extra]\ndisabled = [\"other\"]\n", skill: "one", disabled: true, want: "[skills.extra]\ndisabled = [\"other\"]\n\n[skills]\ndisabled = [\"one\"]\n"},
		{name: "quoted literal unrelated", input: "\"skills.disabled\" = 1\n", skill: "one", disabled: true, want: "\"skills.disabled\" = 1\n\n[skills]\ndisabled = [\"one\"]\n"},
		{name: "trim name", skill: "  one  ", disabled: true, want: "[skills]\ndisabled = [\"one\"]\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SetSkillDisabled([]byte(tc.input), tc.skill, tc.disabled)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Error("result mismatch")
			}
		})
	}
}

func TestSetSkillDisabledRoundTrip(t *testing.T) {
	inputs := []string{"", "model = \"grok\"\n", "model = \"grok\"\r\n", "[skills.team]\nvalue = 1\n"}
	for _, input := range inputs {
		disabled, err := SetSkillDisabled([]byte(input), "review", true)
		if err != nil {
			t.Fatal(err)
		}
		enabled, err := SetSkillDisabled(disabled, "review", false)
		if err != nil {
			t.Fatal(err)
		}
		if string(enabled) != input {
			t.Error("round trip mismatch")
		}
	}
}

func TestSetSkillDisabledRejectsUnsafeInput(t *testing.T) {
	tests := []struct {
		name  string
		input string
		err   error
	}{
		{name: "invalid toml", input: "[skills\n", err: ErrSkillConfigInvalid},
		{name: "duplicate table", input: "[skills]\npaths=[]\n[skills]\ndisabled=[]\n", err: ErrSkillConfigUnsupported},
		{name: "root dotted", input: "skills.disabled = []\n", err: ErrSkillConfigUnsupported},
		{name: "inline", input: "skills = { disabled = [] }\n", err: ErrSkillConfigUnsupported},
		{name: "dotted disabled", input: "[skills]\ndisabled.extra = []\n", err: ErrSkillConfigUnsupported},
		{name: "disabled subtable", input: "[skills.disabled]\nvalue = 1\n", err: ErrSkillConfigUnsupported},
		{name: "scalar", input: "[skills]\ndisabled = \"one\"\n", err: ErrSkillConfigUnsupported},
		{name: "mixed array", input: "[skills]\ndisabled = [\"one\", 2]\n", err: ErrSkillConfigUnsupported},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SetSkillDisabled([]byte(tc.input), "review", true)
			if !errors.Is(err, tc.err) {
				t.Fatalf("error = %v, want %v", err, tc.err)
			}
			if got != nil {
				t.Error("unsafe edit returned content")
			}
		})
	}
}

func TestSetSkillDisabledRejectsInvalidName(t *testing.T) {
	for _, name := range []string{"", "   ", "bad\nname", string([]byte{0xff})} {
		if _, err := SetSkillDisabled(nil, name, true); !errors.Is(err, ErrSkillNameInvalid) {
			t.Errorf("name %q: error = %v", name, err)
		}
	}
}

func TestStoreSetSkillEnabledFileBacksUpAndPreservesMode(t *testing.T) {
	dir := t.TempDir()
	official := filepath.Join(dir, "official")
	accounts := filepath.Join(dir, "accounts")
	if err := os.MkdirAll(official, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(official, "config.toml")
	original := []byte("model = \"grok\"\n")
	if err := os.WriteFile(path, original, 0o640); err != nil {
		t.Fatal(err)
	}
	store := NewStoreAt(accounts, official)
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	if err := store.SetSkillEnabledFile("review", false, now); err != nil {
		t.Fatal(err)
	}
	backups, err := filepath.Glob(filepath.Join(dir, "backups", "grok-config-*.toml"))
	if err != nil || len(backups) != 1 {
		t.Fatalf("backups = %v, error = %v", backups, err)
	}
	backup, err := os.ReadFile(backups[0])
	if err != nil || !bytes.Equal(backup, original) {
		t.Fatalf("backup mismatch or unreadable: %v", err)
	}
	if runtime.GOOS != "windows" {
		if info, err := os.Stat(backups[0]); err != nil {
			t.Errorf("stat backup: %v", err)
		} else if info.Mode().Perm() != 0o600 {
			t.Errorf("backup mode = %v", info.Mode().Perm())
		}
		if info, err := os.Stat(path); err != nil {
			t.Errorf("stat config: %v", err)
		} else if info.Mode().Perm() != 0o640 {
			t.Errorf("config mode = %v", info.Mode().Perm())
		}
	}

	beforeNoOp, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetSkillEnabledFile("review", false, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	afterNoOp, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(beforeNoOp, afterNoOp) || !beforeNoOp.ModTime().Equal(afterNoOp.ModTime()) {
		t.Error("no-op rewrote config")
	}
	backups, _ = filepath.Glob(filepath.Join(dir, "backups", "grok-config-*.toml"))
	if len(backups) != 1 {
		t.Fatalf("no-op created a backup: %v", backups)
	}
}

func TestStoreSetSkillEnabledFileMissingConfig(t *testing.T) {
	dir := t.TempDir()
	official := filepath.Join(dir, "official")
	if err := os.MkdirAll(official, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(official, "config.toml")
	store := NewStoreAt(filepath.Join(dir, "accounts"), official)
	if err := store.SetSkillEnabledFile("review", true, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("enabling absent skill created config: %v", err)
	}
	if err := store.SetSkillEnabledFile("review", false, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Errorf("new config mode = %v", info.Mode().Perm())
		}
	}
}

func TestStoreSetSkillEnabledFileUnsupportedDoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	official := filepath.Join(dir, "official")
	if err := os.MkdirAll(official, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(official, "config.toml")
	const marker = "PRIVATE_CONFIG_MARKER"
	original := []byte("skills = { disabled = [\"" + marker + "\"] }\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewStoreAt(filepath.Join(dir, "accounts"), official)
	err := store.SetSkillEnabledFile("review", false, time.Now())
	if !errors.Is(err, ErrSkillConfigUnsupported) {
		t.Fatalf("error kind = %v", err)
	}
	if err != nil && bytes.Contains([]byte(err.Error()), []byte(marker)) {
		t.Error("error disclosed config content")
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil || !bytes.Equal(got, original) {
		t.Fatalf("config changed or unreadable: %v", readErr)
	}
	backups, _ := filepath.Glob(filepath.Join(dir, "backups", "grok-config-*.toml"))
	if len(backups) != 0 {
		t.Fatalf("unsupported config created backups: %v", backups)
	}
}

func TestStoreSetSkillEnabledFileDoesNotWriteInvalidConfig(t *testing.T) {
	dir := t.TempDir()
	official := filepath.Join(dir, "official")
	if err := os.MkdirAll(official, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(official, "config.toml")
	original := []byte("[skills\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewStoreAt(filepath.Join(dir, "accounts"), official)
	if err := store.SetSkillEnabledFile("review", false, time.Now()); !errors.Is(err, ErrSkillConfigInvalid) {
		t.Fatalf("error = %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, original) {
		t.Fatalf("config changed or unreadable: %v", err)
	}
	backups, _ := filepath.Glob(filepath.Join(dir, "backups", "grok-config-*.toml"))
	if len(backups) != 0 {
		t.Fatalf("invalid config created backups: %v", backups)
	}
}
