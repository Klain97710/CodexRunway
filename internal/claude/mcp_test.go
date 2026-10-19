package claude

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestMCPConfigPathKeepsCustomConfigHomeIsolated(t *testing.T) {
	home := t.TempDir()
	runway := filepath.Join(home, ".quosparr")
	custom := filepath.Join(home, "work-config")
	defaultPath := filepath.Join(home, ".claude.json")
	if err := os.WriteFile(defaultPath, []byte(`{"mcpServers":{"default":{"command":"wrong"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewStore(custom, runway, home, custom, "")
	snapshot, err := store.LoadMCP(context.Background())
	want := filepath.Join(custom, ".claude.json")
	if err != nil || snapshot.Path != want || len(snapshot.Servers) != 0 {
		t.Fatalf("custom LoadMCP = %#v, %v; want empty %s", snapshot, err, want)
	}
	if err := os.MkdirAll(custom, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(want, []byte(`{"mcpServers":{"work":{"command":"tool"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	snapshot, err = store.LoadMCP(context.Background())
	if err != nil || len(snapshot.Servers) != 1 || snapshot.Servers[0].Name != "work" {
		t.Fatalf("custom LoadMCP = %#v, %v", snapshot, err)
	}
}

func TestMCPConfigPathUsesDefaultUserFile(t *testing.T) {
	home := t.TempDir()
	configDir := filepath.Join(home, ".claude")
	for _, test := range []struct {
		name, env, want string
	}{
		{"implicit default", "", filepath.Join(home, ".claude.json")},
		{"explicit default", configDir, filepath.Join(configDir, ".claude.json")},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := NewStore(configDir, filepath.Join(home, ".quosparr"), home, test.env, "")
			if got := store.MCPConfigPath(); got != test.want {
				t.Fatalf("MCPConfigPath = %s, want %s", got, test.want)
			}
		})
	}
}
