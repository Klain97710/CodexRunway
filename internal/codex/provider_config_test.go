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

	"github.com/BurntSushi/toml"
)

var relayCfg = ProviderConfig{Name: "My Relay", BaseURL: "https://relay.example/v1", Model: "gpt-5.5"}

func mustApply(t *testing.T, in string, cfg ProviderConfig) string {
	t.Helper()
	out, err := ApplyManagedProvider([]byte(in), cfg)
	if err != nil {
		t.Fatalf("ApplyManagedProvider: %v\ninput:\n%s", err, in)
	}
	return string(out)
}

func mustRemove(t *testing.T, in, model string) (string, bool) {
	t.Helper()
	out, changed, err := RemoveManagedProvider([]byte(in), model)
	if err != nil {
		t.Fatalf("RemoveManagedProvider: %v\ninput:\n%s", err, in)
	}
	return string(out), changed
}

func parsed(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := toml.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("invalid TOML: %v\n%s", err, s)
	}
	return m
}

const managedTableBody = `[model_providers.quosparr]
name = "My Relay"
base_url = "https://relay.example/v1"
wire_api = "responses"
requires_openai_auth = true
`

func TestApplyToEmptyInput(t *testing.T) {
	want := "model_provider = \"quosparr\"\nmodel = \"gpt-5.5\"\n\n" + managedTableBody
	for _, in := range []string{"", "\n"} {
		got := mustApply(t, in, relayCfg)
		if in == "" && got != want {
			t.Fatalf("got:\n%s\nwant:\n%s", got, want)
		}
		parsed(t, got)
	}
	out, err := ApplyManagedProvider(nil, ProviderConfig{BaseURL: "https://r.example"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `name = "Quosparr"`) || strings.Contains(string(out), "\nmodel = ") {
		t.Fatalf("default name / omitted model wrong:\n%s", out)
	}
}

func TestApplyRequiresBaseURL(t *testing.T) {
	if _, err := ApplyManagedProvider(nil, ProviderConfig{Name: "x", BaseURL: "  "}); !errors.Is(err, ErrProviderBaseURLRequired) {
		t.Fatalf("err = %v", err)
	}
}

const richConfig = `# Codex settings
approval_policy = "never"   # keep
sandbox_mode = "workspace-write"

# trailing note about mcp
[mcp_servers.docs]
command = "npx"
args = [
  "-y",
  "docs",   # inline comment
]

[projects."/tmp/x"]
trust_level = "trusted"
`

func TestApplyPreservesEverythingElseAndRoundTrips(t *testing.T) {
	for _, cfg := range []ProviderConfig{relayCfg, {Name: "R", BaseURL: "https://r.example/v1"}} {
		got := mustApply(t, richConfig, cfg)
		// Top-level lines go after the last existing top-level key; the
		// comment above [mcp_servers.docs] stays attached to it.
		wantPrefix := "# Codex settings\napproval_policy = \"never\"   # keep\nsandbox_mode = \"workspace-write\"\nmodel_provider = \"quosparr\"\n"
		if !strings.HasPrefix(got, wantPrefix) {
			t.Fatalf("prefix wrong:\n%s", got)
		}
		if !strings.Contains(got, "\n\n# trailing note about mcp\n[mcp_servers.docs]\n") {
			t.Fatalf("comment/table order changed:\n%s", got)
		}
		if !strings.HasSuffix(got, "trust_level = \"trusted\"\n\n"+strings.Replace(strings.Replace(managedTableBody, "My Relay", cfg.Name, 1), "relay.example/v1", strings.TrimPrefix(cfg.BaseURL, "https://"), 1)) {
			t.Fatalf("table not appended after one blank line:\n%s", got)
		}
		back, changed := mustRemove(t, got, cfg.Model)
		if !changed || back != richConfig {
			t.Fatalf("round trip differs (changed=%v):\n%s", changed, back)
		}
	}
}

func TestApplyInsertsAtStartWhenNoTopLevelKeys(t *testing.T) {
	in := "# about [x]\n[x]\na = 1\n"
	got := mustApply(t, in, relayCfg)
	if !strings.HasPrefix(got, "model_provider = \"quosparr\"\nmodel = \"gpt-5.5\"\n# about [x]\n[x]\n") {
		t.Fatalf("got:\n%s", got)
	}
	if back, _ := mustRemove(t, got, "gpt-5.5"); back != in {
		t.Fatalf("round trip:\n%s", back)
	}
}

func TestApplyOnlyComments(t *testing.T) {
	in := "# nothing here\n"
	got := mustApply(t, in, relayCfg)
	if back, _ := mustRemove(t, got, "gpt-5.5"); back != in {
		t.Fatalf("round trip:\n%s", back)
	}
}

func TestApplyReplacesInPlace(t *testing.T) {
	in := `model = "old-model"
model_provider = "openai"
effort = "high"

# my providers
[model_providers.other]
name = "Other"

[model_providers.quosparr]
name = "Stale"
base_url = "https://stale.example"
env_key = "STALE"

[tui]
theme = "dark"
`
	got := mustApply(t, in, relayCfg)
	want := `model = "gpt-5.5"
model_provider = "quosparr"
effort = "high"

# my providers
[model_providers.other]
name = "Other"

` + managedTableBody + `
[tui]
theme = "dark"
`
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
	// Idempotent.
	if again := mustApply(t, got, relayCfg); again != got {
		t.Fatalf("second apply changed output:\n%s", again)
	}
}

func TestApplyAddsModelAfterExistingProviderLine(t *testing.T) {
	got := mustApply(t, "a = 1\nmodel_provider = \"openai\"\nb = 2\n", relayCfg)
	if !strings.HasPrefix(got, "a = 1\nmodel_provider = \"quosparr\"\nmodel = \"gpt-5.5\"\nb = 2\n") {
		t.Fatalf("got:\n%s", got)
	}
}

func TestApplyWithoutModelLeavesUserModel(t *testing.T) {
	got := mustApply(t, "model = \"mine\"\n", ProviderConfig{Name: "R", BaseURL: "https://r.example"})
	m := parsed(t, got)
	if m["model"] != "mine" || m["model_provider"] != "quosparr" {
		t.Fatalf("got:\n%s", got)
	}
}

func TestApplyKeepsCRLF(t *testing.T) {
	in := "# note\r\napproval_policy = \"never\"\r\n\r\n[tui]\r\ntheme = \"dark\"\r\n"
	got := mustApply(t, in, relayCfg)
	if bytes.Count([]byte(got), []byte("\n")) != bytes.Count([]byte(got), []byte("\r\n")) {
		t.Fatalf("bare LF introduced: %q", got)
	}
	parsed(t, got)
	if back, _ := mustRemove(t, got, "gpt-5.5"); back != in {
		t.Fatalf("round trip:\n%q", back)
	}
	// In-place replacement keeps CRLF too.
	again := mustApply(t, got, ProviderConfig{Name: "Z", BaseURL: "https://z.example"})
	if bytes.Count([]byte(again), []byte("\n")) != bytes.Count([]byte(again), []byte("\r\n")) {
		t.Fatalf("bare LF after replace: %q", again)
	}
}

func TestApplyNoTrailingNewline(t *testing.T) {
	got := mustApply(t, "a = 1", relayCfg)
	m := parsed(t, got)
	if m["a"] != int64(1) || m["model_provider"] != "quosparr" {
		t.Fatalf("got:\n%s", got)
	}
	got = mustApply(t, "[tui]\ntheme = \"dark\"", relayCfg)
	parsed(t, got)
}

func TestApplyIgnoresHeadersInsideStringsAndArrays(t *testing.T) {
	in := "instructions = \"\"\"\n[model_providers.quosparr]\nname = \"fake\"\n\"\"\"\nmatrix = [\n  [1, 2],\n  [3, 4],\n]\nliteral = '''\n[tui]\n'''\n\n[tui]\ntheme = \"dark\"\n"
	got := mustApply(t, in, relayCfg)
	m := parsed(t, got)
	if m["instructions"] == nil || m["model_provider"] != "quosparr" {
		t.Fatalf("got:\n%s", got)
	}
	// The new lines come after the last top-level key (the literal block).
	if !strings.Contains(got, "[tui]\n'''\nmodel_provider = \"quosparr\"\nmodel = \"gpt-5.5\"\n\n[tui]") {
		t.Fatalf("insert position wrong:\n%s", got)
	}
	if back, _ := mustRemove(t, got, "gpt-5.5"); back != in {
		t.Fatalf("round trip:\n%s", back)
	}
}

func TestApplyEscapesValues(t *testing.T) {
	cfg := ProviderConfig{Name: "He said \"hi\" \\ back\ttab", BaseURL: "https://r.example/v1?a=\"b\"", Model: "m\u00e9\u4e2d"}
	got := mustApply(t, "", cfg)
	m := parsed(t, got)
	table := m["model_providers"].(map[string]any)["quosparr"].(map[string]any)
	if table["name"] != cfg.Name || table["base_url"] != cfg.BaseURL || m["model"] != cfg.Model {
		t.Fatalf("escaped values did not round trip:\n%s", got)
	}
	nl := mustApply(t, "", ProviderConfig{Name: "a\nb\x01", BaseURL: "https://r.example"})
	if parsed(t, nl)["model_providers"].(map[string]any)["quosparr"].(map[string]any)["name"] != "a\nb\x01" {
		t.Fatalf("control chars not escaped:\n%s", nl)
	}
}

func TestApplyRefusesUnsupportedForms(t *testing.T) {
	cases := map[string]string{
		"dotted key at root":       "model_providers.quosparr.name = \"x\"\n",
		"inline table in header":   "[model_providers]\nquosparr = { name = \"x\" }\n",
		"dotted key in header":     "[model_providers]\nquosparr.name = \"x\"\n",
		"root inline model_provs":  "model_providers = { quosparr = { name = \"x\" } }\n",
		"sub-table":                "[model_providers.quosparr]\nname = \"x\"\n[model_providers.quosparr.http_headers]\nA = \"b\"\n",
		"sub-table only":           "[model_providers.quosparr.http_headers]\nA = \"b\"\n",
		"array of tables":          "[[model_providers.quosparr]]\nname = \"x\"\n",
		"multiline in table":       "[model_providers.quosparr]\nname = \"\"\"\nx\n\"\"\"\n",
		"multiline model_provider": "model_provider = \"\"\"\nopenai\"\"\"\n",
	}
	for name, in := range cases {
		out, err := ApplyManagedProvider([]byte(in), relayCfg)
		if !errors.Is(err, ErrProviderConfigUnsupported) || out != nil {
			t.Errorf("%s: err = %v, out = %q; want ErrProviderConfigUnsupported", name, err, out)
		}
	}
}

func TestRemoveRefusesSubTable(t *testing.T) {
	in := "model_provider = \"quosparr\"\n[model_providers.quosparr]\nname = \"x\"\n[model_providers.quosparr.http_headers]\nA = \"b\"\n"
	if _, _, err := RemoveManagedProvider([]byte(in), ""); !errors.Is(err, ErrProviderConfigUnsupported) {
		t.Fatalf("err = %v", err)
	}
}

func TestInvalidInputIsRejectedWithoutEchoingContent(t *testing.T) {
	const secret = "sk-SECRET-TOKEN-123"
	in := "api_key = \"" + secret + "\" oops\n[x\n"
	_, errA := ApplyManagedProvider([]byte(in), relayCfg)
	_, _, errR := RemoveManagedProvider([]byte(in), "m")
	_, _, errD := ReadManagedProvider([]byte(in))
	for _, err := range []error{errA, errR, errD} {
		if !errors.Is(err, ErrProviderConfigInvalid) {
			t.Fatalf("err = %v, want ErrProviderConfigInvalid", err)
		}
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("error echoes file content: %v", err)
		}
	}
}

func TestRemoveOnlyWhatMatches(t *testing.T) {
	in := "model = \"mine\"\nmodel_provider = \"openai\"\n\n[model_providers.other]\nname = \"o\"\n"
	out, changed := mustRemove(t, in, "not-mine")
	if changed || out != in {
		t.Fatalf("a foreign provider/model was touched (changed=%v):\n%s", changed, out)
	}
	// Active with a user-changed model: keep the model, drop provider and table.
	applied := mustApply(t, "model = \"mine\"\n", ProviderConfig{BaseURL: "https://r.example"})
	out, changed = mustRemove(t, applied, "other-model")
	m := parsed(t, out)
	if !changed || m["model"] != "mine" || m["model_provider"] != nil || m["model_providers"] != nil {
		t.Fatalf("got (changed=%v):\n%s", changed, out)
	}
	// Empty appliedModel never removes model.
	withModel := mustApply(t, "", relayCfg)
	out, _ = mustRemove(t, withModel, "")
	if parsed(t, out)["model"] != "gpt-5.5" {
		t.Fatalf("model removed with empty appliedModel:\n%s", out)
	}
	if out, changed := mustRemove(t, "", "m"); changed || out != "" {
		t.Fatalf("empty input changed")
	}
}

func TestRemoveKeepsTableInMiddleTidy(t *testing.T) {
	in := "[a]\nx = 1\n\n" + managedTableBody + "\n[b]\ny = 2\n"
	out, changed := mustRemove(t, in, "")
	if !changed || out != "[a]\nx = 1\n\n[b]\ny = 2\n" {
		t.Fatalf("got (changed=%v):\n%q", changed, out)
	}
}

func TestRemoveDanglingProviderWithoutTable(t *testing.T) {
	out, changed := mustRemove(t, "# c\nmodel_provider = \"quosparr\"\na = 1\n", "")
	if !changed || out != "# c\na = 1\n" {
		t.Fatalf("got (changed=%v): %q", changed, out)
	}
}

func TestReadManagedProvider(t *testing.T) {
	active, url, err := ReadManagedProvider([]byte(mustApply(t, richConfig, relayCfg)))
	if err != nil || !active || url != "https://relay.example/v1" {
		t.Fatalf("active=%v url=%q err=%v", active, url, err)
	}
	active, url, err = ReadManagedProvider([]byte("model_provider = \"openai\"\n"))
	if err != nil || active || url != "" {
		t.Fatalf("active=%v url=%q err=%v", active, url, err)
	}
	// The table alone (provider switched away by the user) still reports its URL.
	active, url, _ = ReadManagedProvider([]byte(managedTableBody))
	if active || url != "https://relay.example/v1" {
		t.Fatalf("active=%v url=%q", active, url)
	}
	if active, url, err = ReadManagedProvider(nil); err != nil || active || url != "" {
		t.Fatalf("empty: %v %q %v", active, url, err)
	}
}

// --- file wrappers ----------------------------------------------------------

func newConfigStore(t *testing.T) (*Store, string, string) {
	t.Helper()
	root := t.TempDir()
	codexHome, runwayHome := filepath.Join(root, ".codex"), filepath.Join(root, ".quosparr")
	return NewStore(codexHome, runwayHome), codexHome, runwayHome
}

func backupFiles(t *testing.T, runwayHome string) []string {
	t.Helper()
	m, err := filepath.Glob(filepath.Join(runwayHome, "backups", "codex-config-*.toml"))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestStoreConfigPathIsNextToAuth(t *testing.T) {
	s, home, _ := newConfigStore(t)
	if s.ConfigPath() != filepath.Join(home, "config.toml") {
		t.Fatalf("ConfigPath = %s", s.ConfigPath())
	}
}

func TestStoreFileCreateApplyRemove(t *testing.T) {
	s, _, runway := newConfigStore(t)
	if active, url, err := s.ReadManagedProviderFile(); err != nil || active || url != "" {
		t.Fatalf("missing file: %v %q %v", active, url, err)
	}
	if err := s.RemoveManagedProviderFile("", testNow); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.ConfigPath()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("remove on a missing file created it: %v", err)
	}
	if err := s.ApplyManagedProviderFile(relayCfg, testNow); err != nil {
		t.Fatal(err)
	}
	if n := len(backupFiles(t, runway)); n != 0 {
		t.Fatalf("a new file needs no backup, got %d", n)
	}
	if runtime.GOOS != "windows" {
		if info, _ := os.Stat(s.ConfigPath()); info.Mode().Perm() != 0o600 {
			t.Fatalf("new config mode = %v", info.Mode().Perm())
		}
	}
	active, url, err := s.ReadManagedProviderFile()
	if err != nil || !active || url != relayCfg.BaseURL {
		t.Fatalf("read: %v %q %v", active, url, err)
	}
	if err := s.RemoveManagedProviderFile(relayCfg.Model, testNow.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(s.ConfigPath()); len(got) != 0 {
		t.Fatalf("config after remove = %q", got)
	}
	if n := len(backupFiles(t, runway)); n != 1 {
		t.Fatalf("backups = %d, want 1", n)
	}
}

func TestStoreFileBackupAndNoRewriteWhenUnchanged(t *testing.T) {
	s, home, runway := newConfigStore(t)
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.ConfigPath(), []byte(richConfig), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := s.ApplyManagedProviderFile(relayCfg, testNow); err != nil {
		t.Fatal(err)
	}
	backups := backupFiles(t, runway)
	if len(backups) != 1 {
		t.Fatalf("backups = %v", backups)
	}
	if b, _ := os.ReadFile(backups[0]); string(b) != richConfig {
		t.Fatalf("backup is not the pre-edit file:\n%s", b)
	}
	if runtime.GOOS != "windows" {
		bi, _ := os.Stat(backups[0])
		di, _ := os.Stat(filepath.Dir(backups[0]))
		ci, _ := os.Stat(s.ConfigPath())
		if bi.Mode().Perm() != 0o600 || di.Mode().Perm() != 0o700 || ci.Mode().Perm() != 0o640 {
			t.Fatalf("modes: backup %v dir %v config %v", bi.Mode().Perm(), di.Mode().Perm(), ci.Mode().Perm())
		}
	}
	before, _ := os.ReadFile(s.ConfigPath())
	if err := s.ApplyManagedProviderFile(relayCfg, testNow.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(s.ConfigPath())
	if !bytes.Equal(before, after) || len(backupFiles(t, runway)) != 1 {
		t.Fatalf("unchanged apply wrote or backed up (backups=%d)", len(backupFiles(t, runway)))
	}
}

func TestStoreFileBackupRotationKeepsNewestTen(t *testing.T) {
	s, _, runway := newConfigStore(t)
	for i := 0; i < 14; i++ {
		cfg := ProviderConfig{Name: "R", BaseURL: "https://r" + string(rune('a'+i)) + ".example"}
		if err := s.ApplyManagedProviderFile(cfg, testNow.Add(time.Duration(i)*time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	backups := backupFiles(t, runway)
	if len(backups) != 10 {
		t.Fatalf("backups = %d, want 10", len(backups))
	}
	// Apply #i backs up the file written by apply #i-1; apply 0 had none.
	// 13 backups were made (urls a..m); the newest ten are d..m.
	oldest := ""
	for _, b := range backups {
		data, _ := os.ReadFile(b)
		if oldest == "" || bytes.Contains(data, []byte("https://rd.example")) {
			oldest = string(data)
		}
	}
	if !strings.Contains(oldest, "https://rd.example") {
		t.Fatalf("oldest retained backup is not d:\n%s", oldest)
	}
	for _, b := range backups {
		if data, _ := os.ReadFile(b); bytes.Contains(data, []byte("https://rc.example")) {
			t.Fatalf("backup c should have been pruned: %s", b)
		}
	}
}

func TestStoreFileErrorsNeverLeakFileContent(t *testing.T) {
	s, home, runway := newConfigStore(t)
	const secret = "sk-SECRET-TOKEN-123"
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatal(err)
	}
	bad := "key = \"" + secret + "\" oops\n"
	if err := os.WriteFile(s.ConfigPath(), []byte(bad), 0o600); err != nil {
		t.Fatal(err)
	}
	errs := []error{
		s.ApplyManagedProviderFile(relayCfg, testNow),
		s.RemoveManagedProviderFile("m", testNow),
	}
	_, _, readErr := s.ReadManagedProviderFile()
	errs = append(errs, readErr)
	for _, err := range errs {
		if err == nil || strings.Contains(err.Error(), secret) {
			t.Fatalf("err = %v", err)
		}
	}
	if got, _ := os.ReadFile(s.ConfigPath()); string(got) != bad {
		t.Fatalf("bad config was modified")
	}
	if n := len(backupFiles(t, runway)); n != 0 {
		t.Fatalf("a failed edit made %d backups", n)
	}
}

func TestApplyRecognisesQuotedKeysAndHeaders(t *testing.T) {
	in := "\"model_provider\" = \"openai\" # was openai\n'model' = \"old\"\n\n[ \"model_providers\" . 'quosparr' ] # mine\nname = \"Stale\"\n\n[tui]\ntheme = \"dark\"\n"
	got := mustApply(t, in, relayCfg)
	want := "model_provider = \"quosparr\"\nmodel = \"gpt-5.5\"\n\n" + managedTableBody + "\n[tui]\ntheme = \"dark\"\n"
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}
