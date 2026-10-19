package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
)

var deleteTime = time.Date(2026, 10, 18, 1, 2, 3, 4, time.UTC)

func TestDeleteTOMLBatchPreservesOtherConfiguration(t *testing.T) {
	source := testSource(t, ProviderCodex, "config.toml")
	const original = `model = "keep-model"

[mcp_servers.keep]
command = "keep-command"
args = ["keep-arg"]

[mcp_servers."remove.one"]
command = "remove-command"

[mcp_servers."remove.one".oauth]
client_secret = "REMOVE_SECRET"

[mcp_servers]
"remove two" = { url = "https://remove.example/secret" }

[plugins.keep]
enabled = true
`
	writeConfig(t, source, original)
	snapshot, err := Load(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	selected := selectServers(t, snapshot, "remove two", "remove.one")
	results, err := Delete(context.Background(), source, selected, deleteTime)
	if err != nil {
		t.Fatal(err)
	}
	for _, result := range results {
		if result.Err != nil {
			t.Fatalf("result = %#v", result)
		}
	}
	written, err := os.ReadFile(source.Path)
	if err != nil {
		t.Fatal(err)
	}
	for _, kept := range []string{`model = "keep-model"`, `[mcp_servers.keep]`, `args = ["keep-arg"]`, `[plugins.keep]`} {
		if !strings.Contains(string(written), kept) {
			t.Fatalf("missing preserved text %q:\n%s", kept, written)
		}
	}
	for _, removed := range []string{"remove.one", "remove two", "REMOVE_SECRET", "remove.example"} {
		if strings.Contains(string(written), removed) {
			t.Fatalf("still contains %q:\n%s", removed, written)
		}
	}
	var decoded map[string]any
	if _, err := toml.Decode(string(written), &decoded); err != nil {
		t.Fatal(err)
	}
	servers := decoded["mcp_servers"].(map[string]any)
	if len(servers) != 1 || servers["keep"] == nil {
		t.Fatalf("remaining servers = %#v", servers)
	}
	assertBackup(t, source, original)
}

func TestDeleteLastTOMLServerLeavesValidConfiguration(t *testing.T) {
	source := testSource(t, ProviderGrok, "config.toml")
	const original = `theme = "dark"

[mcp_servers.only]
command = "node"
`
	writeConfig(t, source, original)
	snapshot, err := Load(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Delete(context.Background(), source, snapshot.Servers, deleteTime); err != nil {
		t.Fatal(err)
	}
	written, _ := os.ReadFile(source.Path)
	if strings.Contains(string(written), "mcp_servers") || !strings.Contains(string(written), `theme = "dark"`) {
		t.Fatalf("written = %s", written)
	}
}

func TestDeleteRejectsReplacedFileWithSameContent(t *testing.T) {
	source := testSource(t, ProviderCodex, "config.toml")
	const original = `[mcp_servers.one]
command = "node"
`
	writeConfig(t, source, original)
	snapshot, err := Load(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	temporary := source.Path + ".replacement"
	oldInfo, err := os.Stat(source.Path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(temporary, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(temporary, oldInfo.ModTime(), oldInfo.ModTime()); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(temporary, source.Path); err != nil {
		t.Fatal(err)
	}
	if _, err := Delete(context.Background(), source, snapshot.Servers, deleteTime); err == nil || !strings.Contains(err.Error(), "identity changed") {
		t.Fatalf("err = %v", err)
	}
	if written, _ := os.ReadFile(source.Path); string(written) != original {
		t.Fatalf("replacement changed: %s", written)
	}
}

func TestDeleteClaudePreservesOauthProjectsAndUnselectedServer(t *testing.T) {
	source := testSource(t, ProviderClaude, ".claude.json")
	const original = `{
  "oauthAccount": {"accountId": 9007199254740993123, "token": "KEEP_OAUTH"},
  "projects": {"/work": {"mcpServers": {"project": {"command": "KEEP_PROJECT"}}}},
  "unknown": {"nested": [1, 2, 3]},
  "mcpServers": {
    "keep": {"command": "KEEP_COMMAND"},
    "remove-a": {"command": "REMOVE_A"},
    "remove-b": {"url": "https://example.com/REMOVE_B"}
  }
}`
	writeConfig(t, source, original)
	snapshot, err := Load(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Delete(context.Background(), source, selectServers(t, snapshot, "remove-a", "remove-b"), deleteTime)
	if err != nil {
		t.Fatal(err)
	}
	written, _ := os.ReadFile(source.Path)
	var document map[string]json.RawMessage
	if err := json.Unmarshal(written, &document); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"oauthAccount", "projects", "unknown"} {
		var before, after any
		var originalDocument map[string]json.RawMessage
		if err := json.Unmarshal([]byte(original), &originalDocument); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(originalDocument[field], &before); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(document[field], &after); err != nil {
			t.Fatal(err)
		}
		if string(originalDocument[field]) != string(document[field]) {
			// Raw whitespace can change, but the preserved RawMessage must retain exact numbers and strings.
			if !jsonSemanticEqual(originalDocument[field], document[field]) {
				t.Fatalf("field %s changed: %s -> %s", field, originalDocument[field], document[field])
			}
		}
	}
	if strings.Contains(string(document["mcpServers"]), "remove-") || !strings.Contains(string(document["mcpServers"]), "KEEP_COMMAND") {
		t.Fatalf("mcpServers = %s", document["mcpServers"])
	}
	assertBackup(t, source, original)
}

func TestDeleteRejectsStaleSelectionAsOneBatch(t *testing.T) {
	source := testSource(t, ProviderGrok, "config.toml")
	const original = `[mcp_servers.one]
command = "one"
[mcp_servers.two]
command = "two"
`
	writeConfig(t, source, original)
	snapshot, err := Load(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	changed := original + "# concurrent change\n"
	writeConfig(t, source, changed)
	results, err := Delete(context.Background(), source, snapshot.Servers, deleteTime)
	if err == nil || !strings.Contains(err.Error(), "changed since") {
		t.Fatalf("err = %v", err)
	}
	for _, result := range results {
		if result.Err == nil {
			t.Fatalf("result = %#v", result)
		}
	}
	if written, _ := os.ReadFile(source.Path); string(written) != changed {
		t.Fatalf("configuration changed: %s", written)
	}
	if _, err := os.Stat(source.BackupDir); !os.IsNotExist(err) {
		t.Fatalf("backup created for rejected deletion: %v", err)
	}
}

func TestDeleteBackupFailureAndUnsafeBackupLeaveConfigurationUntouched(t *testing.T) {
	for _, symlink := range []bool{false, true} {
		t.Run(map[bool]string{false: "file", true: "symlink"}[symlink], func(t *testing.T) {
			source := testSource(t, ProviderCodex, "config.toml")
			const original = `[mcp_servers.one]
command = "one"
`
			writeConfig(t, source, original)
			snapshot, err := Load(context.Background(), source)
			if err != nil {
				t.Fatal(err)
			}
			if symlink {
				real := filepath.Join(filepath.Dir(source.Path), "real-backups")
				if err := os.Mkdir(real, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(real, source.BackupDir); err != nil {
					t.Skipf("cannot create symlink: %v", err)
				}
			} else if err := os.WriteFile(source.BackupDir, []byte("not a directory"), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := Delete(context.Background(), source, snapshot.Servers, deleteTime); err == nil {
				t.Fatal("unsafe backup path was accepted")
			}
			if written, _ := os.ReadFile(source.Path); string(written) != original {
				t.Fatalf("configuration changed: %s", written)
			}
		})
	}
}

func selectServers(t *testing.T, snapshot Snapshot, names ...string) []Server {
	t.Helper()
	wanted := make(map[string]bool, len(names))
	for _, name := range names {
		wanted[name] = true
	}
	var selected []Server
	for _, server := range snapshot.Servers {
		if wanted[server.Name] {
			selected = append(selected, server)
			delete(wanted, server.Name)
		}
	}
	if len(wanted) > 0 {
		t.Fatalf("servers not found: %v in %#v", wanted, snapshot.Servers)
	}
	return selected
}

func assertBackup(t *testing.T, source Source, original string) {
	t.Helper()
	entries, err := os.ReadDir(source.BackupDir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("backups = %v, err = %v", entries, err)
	}
	backup, err := os.ReadFile(filepath.Join(source.BackupDir, entries[0].Name()))
	if err != nil || string(backup) != original {
		t.Fatalf("backup = %q, err = %v", backup, err)
	}
	if runtime.GOOS != "windows" {
		fileInfo, _ := entries[0].Info()
		dirInfo, _ := os.Stat(source.BackupDir)
		if fileInfo.Mode().Perm() != 0o600 || dirInfo.Mode().Perm() != 0o700 {
			t.Fatalf("backup modes = %v, %v", fileInfo.Mode().Perm(), dirInfo.Mode().Perm())
		}
	}
}

func jsonSemanticEqual(a, b []byte) bool {
	var left, right any
	leftDecoder := json.NewDecoder(strings.NewReader(string(a)))
	leftDecoder.UseNumber()
	rightDecoder := json.NewDecoder(strings.NewReader(string(b)))
	rightDecoder.UseNumber()
	if leftDecoder.Decode(&left) != nil || rightDecoder.Decode(&right) != nil {
		return false
	}
	return strings.TrimSpace(string(mustJSON(left))) == strings.TrimSpace(string(mustJSON(right)))
}

func mustJSON(value any) []byte {
	data, _ := json.Marshal(value)
	return data
}
