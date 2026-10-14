package usage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/plugins/sqlite"
	"github.com/licoy/quosparr/internal/sqliteruntime"
	"github.com/licoy/quosparr/internal/usage/decimal"
)

// requireSQLite skips (or, with USAGE_REQUIRE_SQLITE=1, fails) when the
// native SQLite library of the mygo plugin cannot be loaded, for example on
// an offline machine with a cold cache. Set MYGO_SQLITE_LIBRARY to a local
// library to run offline.
func requireSQLite(t testing.TB) {
	t.Helper()
	if err := sqliteruntime.Load(); err != nil {
		if os.Getenv("USAGE_REQUIRE_SQLITE") == "1" {
			t.Fatalf("sqlite native library unavailable: %v", err)
		}
		t.Skipf("sqlite native library unavailable (set MYGO_SQLITE_LIBRARY or USAGE_REQUIRE_SQLITE=1): %v", err)
	}
}

var fixedNow = mustTime("2026-06-30T12:00:00Z")

func mustTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func mustLoc(name string) *time.Location {
	l, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return l
}

type repoFixture struct {
	t         testing.TB
	root      string
	codexHome string
	dbPath    string
}

func newFixture(t testing.TB) *repoFixture {
	t.Helper()
	requireSQLite(t)
	root := t.TempDir()
	f := &repoFixture{
		t:         t,
		root:      root,
		codexHome: filepath.Join(root, ".codex"),
		dbPath:    filepath.Join(root, ".quosparr", "index.sqlite3"),
	}
	for _, d := range []string{
		filepath.Join(f.codexHome, "sessions", "2026", "06", "29"),
		filepath.Join(f.codexHome, "archived_sessions", "2026", "06", "29"),
		filepath.Dir(f.dbPath),
	} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

type repoOptions struct {
	parserVersion int64
	prices        PriceSource
	loc           *time.Location
	rebuild       RebuildPolicy
	beforeFlight  func(context.Context)
	chunk, maxLn  int
}

func (f *repoFixture) repo(opts ...func(*repoOptions)) *Repository {
	f.t.Helper()
	o := repoOptions{loc: time.UTC}
	for _, fn := range opts {
		fn(&o)
	}
	r, err := NewRepository(Config{
		CodexHome:     f.codexHome,
		DatabasePath:  f.dbPath,
		Location:      o.loc,
		ParserVersion: o.parserVersion,
		Prices:        o.prices,
		Rebuild:       o.rebuild,
		ChunkSize:     o.chunk,
		MaxLineBytes:  o.maxLn,
	})
	if err != nil {
		f.t.Fatal(err)
	}
	r.beforeFlight = o.beforeFlight
	f.t.Cleanup(func() { r.Close() })
	return r
}

func withParser(v int64) func(*repoOptions)       { return func(o *repoOptions) { o.parserVersion = v } }
func withLoc(l *time.Location) func(*repoOptions) { return func(o *repoOptions) { o.loc = l } }
func withBook(b PriceBook) func(*repoOptions) {
	return func(o *repoOptions) { o.prices = FixedPrices(b) }
}
func withRebuild(p RebuildPolicy) func(*repoOptions) { return func(o *repoOptions) { o.rebuild = p } }
func withGate(fn func(context.Context)) func(*repoOptions) {
	return func(o *repoOptions) { o.beforeFlight = fn }
}

func (f *repoFixture) sessionPath(base string) string {
	return filepath.Join(f.codexHome, "sessions", "2026", "06", "29", base)
}

func (f *repoFixture) archivedPath(base string) string {
	return filepath.Join(f.codexHome, "archived_sessions", "2026", "06", "29", base)
}

func (f *repoFixture) write(contents, base string) string {
	f.t.Helper()
	p := f.sessionPath(base)
	if err := os.WriteFile(p, []byte(contents), 0o644); err != nil {
		f.t.Fatal(err)
	}
	return p
}

// sum runs one query with the default policy.
func sum(t testing.TB, r *Repository, q Query) Summary {
	t.Helper()
	m, err := r.Summaries(context.Background(), []Query{q}, fixedNow, RefreshIfChanged, nil)
	if err != nil {
		t.Fatalf("Summaries: %v", err)
	}
	s, ok := m[q.ID]
	if !ok {
		t.Fatalf("no summary for %q", q.ID)
	}
	return s
}

func fullQuery() Query {
	return mkQuery("full", "2026-06-29T00:00:00Z", "2026-06-30T00:00:00Z")
}

func mkQuery(id, start, end string) Query {
	return Query{ID: id, Window: Window{Start: mustTime(start), End: mustTime(end)}}
}

func tokenLine(ts string, input int64, opts ...func(*tokenOpts)) string {
	o := tokenOpts{output: 5, model: "gpt-5.5"}
	for _, fn := range opts {
		fn(&o)
	}
	return fmt.Sprintf(`{"timestamp":"%s","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":%d,"cached_input_tokens":%d,"output_tokens":%d,"reasoning_output_tokens":0}}},"turn_context":{"model":"%s"}}`,
		ts, input, o.cached, o.output, o.model)
}

type tokenOpts struct {
	cached, output int64
	model          string
}

func cachedTokens(n int64) func(*tokenOpts) { return func(o *tokenOpts) { o.cached = n } }
func outputTokens(n int64) func(*tokenOpts) { return func(o *tokenOpts) { o.output = n } }
func modelName(m string) func(*tokenOpts)   { return func(o *tokenOpts) { o.model = m } }

func taskCompleteLine(ts, turnID string) string {
	return fmt.Sprintf(`{"timestamp":"%s","type":"event_msg","payload":{"type":"task_complete","turn_id":"%s"}}`, ts, turnID)
}

func lines(ls ...string) string { return strings.Join(ls, "\n") + "\n" }

func appendFile(t testing.TB, path, text string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(text); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

// replaceKeepingIdentity rewrites the file in place (same inode).
func replaceKeepingIdentity(t testing.TB, path, contents string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(contents); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func mtimeOf(t testing.TB, path string) time.Time {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return fi.ModTime()
}

func setMtime(t testing.TB, path string, mt time.Time) {
	t.Helper()
	if err := os.Chtimes(path, mt, mt); err != nil {
		t.Fatal(err)
	}
}

func fileID(t testing.TB, path string) [2]int64 {
	t.Helper()
	m, err := lstatMeta(path)
	if err != nil {
		t.Fatal(err)
	}
	return [2]int64{m.Device, m.Inode}
}

func priceBookOf(version, in, cached, out string) PriceBook {
	p := simplePrice(in, cached, out)
	return NewPriceBook(version, map[string]Price{"gpt-5.5": p}, p)
}

// execSQL opens the index directly, the way an outside tool would.
func execSQL(t testing.TB, path, stmt string) {
	t.Helper()
	db, err := sqlite.Open(context.Background(), path, sqlite.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Execute(context.Background(), stmt); err != nil {
		t.Fatalf("%s: %v", stmt, err)
	}
}

// gate blocks flights until opened and counts how many are parked.
type gate struct {
	mu      sync.Mutex
	opened  chan struct{}
	waiting int
	once    sync.Once
}

func newGate() *gate { return &gate{opened: make(chan struct{})} }

func (g *gate) wait(ctx context.Context) {
	g.mu.Lock()
	g.waiting++
	g.mu.Unlock()
	select {
	case <-g.opened:
	case <-ctx.Done():
	}
}

func (g *gate) open() { g.once.Do(func() { close(g.opened) }) }

func (g *gate) waitingCount() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.waiting
}

func eventually(t testing.TB, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func simplePrice(in, cached, out string) Price {
	return Price{InputPerMillion: decimal.MustParse(in), CachedInputPerMillion: decimal.MustParse(cached), OutputPerMillion: decimal.MustParse(out)}
}
