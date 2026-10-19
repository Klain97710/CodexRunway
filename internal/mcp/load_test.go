package mcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testSource(t *testing.T, provider Provider, name string) Source {
	t.Helper()
	dir := t.TempDir()
	return Source{Provider: provider, Path: filepath.Join(dir, name), BackupDir: filepath.Join(dir, "backups")}
}

func writeConfig(t *testing.T, source Source, content string) {
	t.Helper()
	if err := os.WriteFile(source.Path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadTOMLRedactsSecretsAndReportsState(t *testing.T) {
	source := testSource(t, ProviderCodex, "config.toml")
	const config = `model = "gpt-test"

[mcp_servers."docs.prod"]
url = "https://token-user:token-pass@example.com/private/API_SECRET?q=QUERY_SECRET"
http_headers = { Authorization = "Bearer HEADER_SECRET" }
enabled = false
enabled_tools = ["search", "read"]
startup_timeout_sec = 12

[mcp_servers."local tool"]
command = "${COMMAND_BIN:-COMMAND_SECRET}"
args = ["--token", "ARG_SECRET"]
env = { API_KEY = "ENV_SECRET" }
numeric_token = 123456789

[mcp_servers."docs.prod".oauth]
client_secret = "OAUTH_SECRET"
callback_url = "https://example.com/CALLBACK_SECRET"
`
	writeConfig(t, source, config)
	snapshot, err := Load(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Servers) != 2 || snapshot.Servers[0].Name != "docs.prod" {
		t.Fatalf("servers = %#v", snapshot.Servers)
	}
	docs := snapshot.Servers[0]
	if docs.Enabled || docs.Transport != "http" || docs.URL != "https://example.com" {
		t.Fatalf("docs summary = %#v", docs)
	}
	local := snapshot.Servers[1]
	if local.Command != hidden || local.Transport != "stdio" {
		t.Fatalf("local summary = %#v", local)
	}
	visible := docs.Command + docs.URL + docs.Details + local.Command + local.URL + local.Details
	for _, secret := range []string{"token-user", "token-pass", "API_SECRET", "QUERY_SECRET", "HEADER_SECRET", "ARG_SECRET", "ENV_SECRET", "COMMAND_SECRET", "OAUTH_SECRET", "CALLBACK_SECRET", "123456789"} {
		if strings.Contains(visible, secret) {
			t.Fatalf("public summary leaked %q: %s", secret, visible)
		}
	}
	for _, expected := range []string{`"Authorization": "<hidden>"`, `"enabled_tools": [`, `"args": [`} {
		if !strings.Contains(docs.Details+local.Details, expected) {
			t.Fatalf("details missing %s: %s %s", expected, docs.Details, local.Details)
		}
	}
}

func TestLoadReportsTransportSchemaProblems(t *testing.T) {
	source := testSource(t, ProviderClaude, ".claude.json")
	writeConfig(t, source, `{"mcpServers":{
  "missing-type":{"url":"https://example.com/mcp"},
  "stdio-url":{"type":"stdio","url":"https://example.com/mcp"},
  "http-command":{"type":"http","command":"node"},
  "secret-type-both":{"type":"TYPE_SECRET","command":"node","url":"https://example.com/mcp"},
  "valid-stdio":{"command":"node"},
  "valid-http":{"type":"http","url":"https://example.com/mcp"}
}}`)
	snapshot, err := Load(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	problems := map[string]string{}
	for _, server := range snapshot.Servers {
		problems[server.Name] = server.Problem
	}
	for name, want := range map[string]string{"missing-type": "missingType", "stdio-url": "transportMismatch", "http-command": "transportMismatch", "secret-type-both": "invalidTransport", "valid-stdio": "", "valid-http": ""} {
		if problems[name] != want {
			t.Fatalf("problem[%s] = %q, want %q", name, problems[name], want)
		}
	}
	for _, server := range snapshot.Servers {
		if server.Name == "secret-type-both" && (server.Transport != "unknown" || strings.Contains(server.Transport, "TYPE_SECRET")) {
			t.Fatalf("transport leaked unknown type: %#v", server)
		}
	}
}

func TestLoadFormatsNonFiniteWhitelistedNumber(t *testing.T) {
	source := testSource(t, ProviderCodex, "config.toml")
	writeConfig(t, source, "[mcp_servers.test]\ncommand = \"node\"\nstartup_timeout_sec = nan\n")
	snapshot, err := Load(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(snapshot.Servers[0].Details, "<non-finite>") {
		t.Fatalf("details = %s", snapshot.Servers[0].Details)
	}
}

func TestLoadClaudeOnlyReadsTopLevelMCPServers(t *testing.T) {
	source := testSource(t, ProviderClaude, ".claude.json")
	const config = `{
  "oauthAccount": {"token": "OAUTH_TOKEN"},
  "projects": {"/work": {"mcpServers": {"project-only": {"command": "secret-project"}}}},
  "mcpServers": {
    "user-http": {"type": "http", "url": "https://example.com/private/URL_SECRET", "headers": {"Authorization": "HEADER_SECRET"}},
    "user-local": {"command": "/usr/local/bin/node", "args": ["ARG_SECRET"], "env": {"TOKEN": "ENV_SECRET"}}
  }
}`
	writeConfig(t, source, config)
	snapshot, err := Load(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Servers) != 2 || snapshot.Servers[0].Name != "user-http" || snapshot.Servers[1].Name != "user-local" {
		t.Fatalf("servers = %#v", snapshot.Servers)
	}
	all := snapshot.Servers[0].Details + snapshot.Servers[1].Details
	for _, hiddenValue := range []string{"OAUTH_TOKEN", "project-only", "URL_SECRET", "HEADER_SECRET", "ARG_SECRET", "ENV_SECRET"} {
		if strings.Contains(all, hiddenValue) {
			t.Fatalf("details leaked %q: %s", hiddenValue, all)
		}
	}
}

func TestLoadRejectsMalformedAndUnsafeFilesWithoutLeakingContent(t *testing.T) {
	for _, tc := range []struct {
		provider Provider
		name     string
		content  string
	}{
		{ProviderCodex, "config.toml", `[mcp_servers.bad]\nsecret = "DO_NOT_REPORT"\nbroken = [`},
		{ProviderClaude, ".claude.json", `{"mcpServers":{"bad":"DO_NOT_REPORT"}}`},
		{ProviderClaude, ".claude.json", `{"mcpServers":{"DO_NOT_REPORT":"bad"}}`},
		{ProviderClaude, ".claude.json", `{"mcpServers":{},"mcpServers":{"bad":{"command":"DO_NOT_REPORT"}}}`},
		{ProviderClaude, ".claude.json", `{"mcpServers":{}} DO_NOT_REPORT`},
	} {
		t.Run(string(tc.provider), func(t *testing.T) {
			source := testSource(t, tc.provider, tc.name)
			writeConfig(t, source, tc.content)
			_, err := Load(context.Background(), source)
			if err == nil || strings.Contains(err.Error(), "DO_NOT_REPORT") {
				t.Fatalf("err = %v", err)
			}
		})
	}

	source := testSource(t, ProviderGrok, "config.toml")
	if err := os.Mkdir(source.Path, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(context.Background(), source); err == nil {
		t.Fatal("directory configuration was accepted")
	}
}

func TestLoadRejectsConfigurationSymlink(t *testing.T) {
	if testing.Short() {
		t.Skip("symlink test")
	}
	source := testSource(t, ProviderCodex, "config.toml")
	real := filepath.Join(filepath.Dir(source.Path), "real.toml")
	writeConfig(t, Source{Path: real}, `[mcp_servers.test]\ncommand = "node"\n`)
	if err := os.Symlink(real, source.Path); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}
	if _, err := Load(context.Background(), source); err == nil {
		t.Fatal("symlink configuration was accepted")
	}
}
