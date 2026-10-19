package claude

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestSetSkillOverride(t *testing.T) {
	tests := []struct {
		name    string
		in      []byte
		skill   string
		enabled bool
		want    string
	}{
		{name: "disable empty input", skill: "review", want: `{"skillOverrides":{"review":"off"}}`},
		{name: "disable trims name", in: []byte(`{}`), skill: "  review  ", want: `{"skillOverrides":{"review":"off"}}`},
		{name: "enable removes off", in: []byte(`{"skillOverrides":{"review":"off"},"theme":"dark"}`), skill: "review", enabled: true, want: `{"theme":"dark"}`},
		{name: "enable removes name-only", in: []byte(`{"skillOverrides":{"review":"name-only","keep":"off"}}`), skill: "review", enabled: true, want: `{"skillOverrides":{"keep":"off"}}`},
		{name: "enable removes user-invocable-only", in: []byte(`{"skillOverrides":{"review":"user-invocable-only"}}`), skill: "review", enabled: true, want: `{}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SetSkillOverride(tt.in, tt.skill, tt.enabled)
			if err != nil {
				t.Fatal(err)
			}
			assertJSONEqual(t, got, []byte(tt.want))
		})
	}
}

func TestSetSkillOverrideNoOpPreservesOriginalBytes(t *testing.T) {
	tests := []struct {
		name    string
		in      []byte
		enabled bool
	}{
		{name: "missing enabled nil", enabled: true},
		{name: "missing enabled CRLF", in: []byte("{\r\n  \"theme\": \"dark\"\r\n}\r\n"), enabled: true},
		{name: "already off CRLF", in: []byte("{\r\n  \"skillOverrides\": {\"review\": \"off\"}\r\n}\r\n")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SetSkillOverride(tt.in, "review", tt.enabled)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, tt.in) {
				t.Fatal("no-op changed the original bytes")
			}
		})
	}
	first, err := SetSkillOverride([]byte(`{"theme":"dark"}`), "review", false)
	if err != nil {
		t.Fatal(err)
	}
	second, err := SetSkillOverride(first, "review", false)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(second, first) {
		t.Fatal("repeated disable changed the first disable output")
	}
}

func TestSetSkillOverridePreservesOtherRawMembers(t *testing.T) {
	in := []byte(`{"big":12345678901234567890,"nested":{"items":[true,null,{"x":"<a & b>"}]},"skillOverrides":{"keep":{"future":1}}}`)
	out, err := SetSkillOverride(in, "review", false)
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(out, &root); err != nil {
		t.Fatal(err)
	}
	if string(root["big"]) != "12345678901234567890" || !bytes.Contains(root["nested"], []byte(`<a & b>`)) {
		t.Fatal("unrelated raw members changed")
	}
	var overrides map[string]json.RawMessage
	if err := json.Unmarshal(root["skillOverrides"], &overrides); err != nil {
		t.Fatal(err)
	}
	var keep map[string]int
	if err := json.Unmarshal(overrides["keep"], &keep); err != nil {
		t.Fatal(err)
	}
	if stringField(overrides["review"]) != "off" || keep["future"] != 1 {
		t.Fatal("skill overrides changed unexpectedly")
	}
}

func TestSetSkillOverrideRejectsInvalidInput(t *testing.T) {
	for _, in := range []string{`[1]`, `null`, `{`, `{"skillOverrides":null}`, `{"skillOverrides":[]}`, `{"skillOverrides":"off"}`} {
		if _, err := SetSkillOverride([]byte(in), "review", true); !errors.Is(err, ErrSettingsInvalid) {
			t.Fatalf("error = %v, want ErrSettingsInvalid", err)
		}
	}
	for _, name := range []string{"", "   ", "review\n", "re\u0000view", "review\u007f"} {
		if _, err := SetSkillOverride(nil, name, false); !errors.Is(err, ErrSkillNameInvalid) {
			t.Fatalf("error = %v, want ErrSkillNameInvalid", err)
		}
	}
}

func TestStoreSetSkillEnabledFileMissingLifecycle(t *testing.T) {
	s, _, runway := newSettingsStore(t)
	if err := s.SetSkillEnabledFile("review", true, settingsNow); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.SettingsPath()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("enable created missing settings: %v", err)
	}
	if err := s.SetSkillEnabledFile("review", false, settingsNow); err != nil {
		t.Fatal(err)
	}
	if len(settingsBackups(t, runway)) != 0 {
		t.Fatal("new settings file created a backup")
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(s.SettingsPath())
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("new settings mode = %v", info.Mode().Perm())
		}
	}
}

func TestStoreSetSkillEnabledFileBackupModeAndNoOp(t *testing.T) {
	s, configDir, runway := newSettingsStore(t)
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	original := []byte("{\r\n  \"theme\": \"dark\"\r\n}\r\n")
	if err := os.WriteFile(s.SettingsPath(), original, 0o640); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Unix(1_700_000_000, 0)
	if err := os.Chtimes(s.SettingsPath(), oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(s.SettingsPath())
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetSkillEnabledFile("review", true, settingsNow); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(s.SettingsPath())
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) || !after.ModTime().Equal(oldTime) || len(settingsBackups(t, runway)) != 0 {
		t.Fatal("no-op rewrote settings or made a backup")
	}
	if err := s.SetSkillEnabledFile("review", false, settingsNow.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	backups := settingsBackups(t, runway)
	if len(backups) != 1 {
		t.Fatalf("backups = %d, want 1", len(backups))
	}
	backup, err := os.ReadFile(backups[0])
	if err != nil || !bytes.Equal(backup, original) {
		t.Fatal("backup does not contain the original bytes")
	}
	if runtime.GOOS != "windows" {
		backupInfo, backupErr := os.Stat(backups[0])
		settingsInfo, settingsErr := os.Stat(s.SettingsPath())
		if backupErr != nil || settingsErr != nil || backupInfo.Mode().Perm() != 0o600 || settingsInfo.Mode().Perm() != 0o640 {
			t.Fatal("backup or settings mode is incorrect")
		}
	}
	if err := os.Chtimes(s.SettingsPath(), oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	beforeRepeat, err := os.Stat(s.SettingsPath())
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetSkillEnabledFile("review", false, settingsNow.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	afterRepeat, err := os.Stat(s.SettingsPath())
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(beforeRepeat, afterRepeat) || !afterRepeat.ModTime().Equal(oldTime) || len(settingsBackups(t, runway)) != 1 {
		t.Fatal("repeated disable rewrote settings or made a backup")
	}
}

func TestStoreSetSkillEnabledFileInvalidDoesNotWrite(t *testing.T) {
	const marker = "SENSITIVE_FIXTURE_MARKER"
	tests := []struct {
		name    string
		data    string
		skill   string
		wantErr error
	}{
		{name: "syntax", data: `{"marker":"` + marker + `",`, skill: "review", wantErr: ErrSettingsInvalid},
		{name: "null overrides", data: `{"marker":"` + marker + `","skillOverrides":null}`, skill: "review", wantErr: ErrSettingsInvalid},
		{name: "invalid name", data: `{"marker":"` + marker + `"}`, skill: "review\n", wantErr: ErrSkillNameInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, configDir, runway := newSettingsStore(t)
			if err := os.MkdirAll(configDir, 0o700); err != nil {
				t.Fatal(err)
			}
			original := []byte(tt.data)
			if err := os.WriteFile(s.SettingsPath(), original, 0o600); err != nil {
				t.Fatal(err)
			}
			before, err := os.Stat(s.SettingsPath())
			if err != nil {
				t.Fatal(err)
			}
			err = s.SetSkillEnabledFile(tt.skill, true, settingsNow)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if strings.Contains(err.Error(), marker) {
				t.Fatal("error leaked fixture marker")
			}
			after, statErr := os.Stat(s.SettingsPath())
			data, readErr := os.ReadFile(s.SettingsPath())
			if statErr != nil || readErr != nil || !bytes.Equal(data, original) || !os.SameFile(before, after) || len(settingsBackups(t, runway)) != 0 {
				t.Fatal("invalid settings were written or backed up")
			}
		})
	}
}

func assertJSONEqual(t *testing.T, got, want []byte) {
	t.Helper()
	var gotValue, wantValue any
	if err := json.Unmarshal(got, &gotValue); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(want, &wantValue); err != nil {
		t.Fatal(err)
	}
	gotJSON, _ := json.Marshal(gotValue)
	wantJSON, _ := json.Marshal(wantValue)
	if !bytes.Equal(gotJSON, wantJSON) {
		t.Fatal("JSON values differ")
	}
}
