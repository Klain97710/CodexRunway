package sessions

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/export/swiftjson"
)

// testPricer prices gpt-5.5 at $1 and the spark model at $1.75 per million
// uncached input tokens; any other model is unpriced.
func testPricer(model string, t Totals) (swiftjson.Decimal, bool) {
	rate := map[string]string{"gpt-5.5": "1", "gpt-5.3-codex-spark": "1.75"}[model]
	if rate == "" {
		return swiftjson.Decimal{}, false
	}
	cost := swiftjson.MustDecimal(rate).Rat()
	cost.Mul(cost, swiftjson.DecimalFromInt(int64(t.UncachedInputTokens)).Rat())
	cost.Quo(cost, swiftjson.DecimalFromInt(1_000_000).Rat())
	return swiftjson.DecimalFromRat(cost), true
}

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

type fixture struct {
	t    *testing.T
	home string
}

func newFixture(t *testing.T) *fixture {
	return &fixture{t: t, home: t.TempDir()}
}

func (f *fixture) write(rel, content string) string {
	f.t.Helper()
	path := filepath.Join(f.home, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		f.t.Fatal(err)
	}
	return path
}

func (f *fixture) touch(path, stamp string) {
	f.t.Helper()
	at := mustTime(f.t, stamp)
	if err := os.Chtimes(path, at, at); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) scan(limit int) Summary {
	f.t.Helper()
	sum, err := Scanner{CodexHome: f.home, Pricer: testPricer}.Scan(limit)
	if err != nil {
		f.t.Fatal(err)
	}
	return sum
}

func sessionFile(id, timestamp, title string) string {
	return fmt.Sprintf(`{"timestamp":%q,"type":"session_meta","payload":{"id":%q,"cwd":"/Users/me/dev/quosparr"}}
{"timestamp":%q,"type":"event_msg","payload":{"type":"message","role":"user","content":%q}}
{"timestamp":%q,"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10,"cached_input_tokens":0,"output_tokens":5,"reasoning_output_tokens":0}}},"turn_context":{"model":"gpt-5.5"}}`,
		timestamp, id, timestamp, title, timestamp)
}

func ids(items []Item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.ID
	}
	return out
}

func titles(items []Item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Title
	}
	return out
}

func equal(a, b []string) bool { return strings.Join(a, "\x00") == strings.Join(b, "\x00") }

func TestSummarizesRecentSessions(t *testing.T) {
	f := newFixture(t)
	id := "019f17a5-436d-73b2-a93d-7af3e78cc827"
	f.write("session_index.jsonl", fmt.Sprintf(`{"id":%q,"thread_name":"Status bar quota fix","updated_at":"2026-06-29T00:04:00Z"}`, id))
	f.write("sessions/2026/06/29/rollout-"+id+".jsonl", fmt.Sprintf(`{"timestamp":"2026-06-29T00:00:00Z","type":"session_meta","payload":{"id":%q,"cwd":"/Users/me/dev/quosparr"}}
{"timestamp":"2026-06-29T00:01:00Z","type":"event_msg","payload":{"type":"message","role":"user","content":"Fix the status bar"}}
{"timestamp":"2026-06-29T00:02:00Z","type":"event_msg","payload":{"type":"approval_request"}}
{"timestamp":"2026-06-29T00:03:00Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1000,"cached_input_tokens":100,"output_tokens":50,"reasoning_output_tokens":10}}},"turn_context":{"model":"gpt-5.5"}}
`, id))

	items := f.scan(5).Items
	if len(items) != 1 {
		t.Fatalf("items = %d", len(items))
	}
	s := items[0]
	if s.Title != "Status bar quota fix" || s.ProjectName != "quosparr" || s.State != StateNeedsAttention {
		t.Fatalf("got %+v", s)
	}
	if s.Totals.TotalTokens != 1050 || s.Totals.CachedInputTokens != 100 || s.Totals.OutputTokens != 50 || s.Totals.UncachedInputTokens != 900 {
		t.Fatalf("totals %+v", s.Totals)
	}
	if s.EstimatedUSD == nil || s.EstimatedUSD.Sign() <= 0 {
		t.Fatalf("estimate %v", s.EstimatedUSD)
	}
	if want := mustTime(t, "2026-06-29T00:03:00Z"); !s.UpdatedAt.Equal(want) {
		t.Fatalf("updatedAt %v", s.UpdatedAt)
	}
}

func TestPricesWithInjectedPricer(t *testing.T) {
	f := newFixture(t)
	id := "019f17a5-436d-73b2-a93d-7af3e78cc828"
	f.write("sessions/2026/06/29/rollout-"+id+".jsonl", fmt.Sprintf(`{"timestamp":"2026-06-29T00:00:00Z","type":"session_meta","payload":{"id":%q,"cwd":"/Users/me/dev/spark"}}
{"timestamp":"2026-06-29T00:01:00Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1000000,"cached_input_tokens":0,"output_tokens":0,"reasoning_output_tokens":0}}},"turn_context":{"model":"gpt-5.3-codex-spark"}}`, id))
	got := f.scan(1).Items[0].EstimatedUSD
	if got == nil || !got.Equal(swiftjson.MustDecimal("1.75")) {
		t.Fatalf("estimate %v", got)
	}
	unpriced, err := Scanner{CodexHome: f.home}.Scan(1)
	if err != nil || unpriced.Items[0].EstimatedUSD != nil {
		t.Fatalf("nil Pricer must leave the estimate empty: %v %v", unpriced.Items, err)
	}
}

func TestRejectsInvalidTokenUsage(t *testing.T) {
	f := newFixture(t)
	id := "deadbeef-dead-4eef-8ead-deadbeefdead"
	f.write("sessions/2026/06/29/rollout-"+id+".jsonl", fmt.Sprintf(`{"timestamp":"2026-06-29T00:00:00Z","type":"session_meta","payload":{"id":%q,"cwd":"/tmp/project"}}
{"timestamp":"2026-06-29T00:01:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":1,"cached_input_tokens":0,"output_tokens":9223372036854775807,"reasoning_output_tokens":1}}}}`, id))
	_, err := Scanner{CodexHome: f.home}.Scan(1)
	var arith *ArithmeticError
	if !errors.As(err, &arith) || !arith.Overflow {
		t.Fatalf("want overflow ArithmeticError, got %v", err)
	}

	f2 := newFixture(t)
	f2.write("sessions/2026/06/29/rollout-"+id+".jsonl", fmt.Sprintf(`{"timestamp":"2026-06-29T00:00:00Z","type":"session_meta","payload":{"id":%q}}
{"timestamp":"2026-06-29T00:01:00Z","type":"event_msg","payload":{"info":{"total_token_usage":{"input_tokens":5,"cached_input_tokens":6}}}}`, id))
	if _, err := (Scanner{CodexHome: f2.home}).Scan(1); !errors.As(err, &arith) || arith.Overflow {
		t.Fatalf("cached > input must be an invalid value, got %v", err)
	}
	f3 := newFixture(t)
	f3.write("sessions/2026/06/29/rollout-"+id+".jsonl", fmt.Sprintf(`{"timestamp":"2026-06-29T00:00:00Z","type":"session_meta","payload":{"id":%q}}
{"timestamp":"2026-06-29T00:01:00Z","type":"event_msg","payload":{"info":{"total_token_usage":{"input_tokens":1.5}}}}`, id))
	if _, err := (Scanner{CodexHome: f3.home}).Scan(1); !errors.As(err, &arith) {
		t.Fatalf("non-integer count must fail, got %v", err)
	}
}

func TestIndexTitlesButFileActivityOrder(t *testing.T) {
	f := newFixture(t)
	first, second := "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222"
	f.write("session_index.jsonl", fmt.Sprintf(`{"id":%q,"thread_name":"Index newest","updated_at":"2026-06-30T00:00:00Z"}
{"id":%q,"thread_name":"File newest","updated_at":"2026-06-29T00:00:00Z"}`, first, second))
	f.write("sessions/2026/06/29/rollout-"+first+".jsonl", sessionFile(first, "2026-06-28T00:00:00Z", "Old file title"))
	f.write("sessions/2026/06/29/rollout-"+second+".jsonl", sessionFile(second, "2026-06-29T00:00:00Z", "New file title"))
	items := f.scan(2).Items
	if !equal(ids(items), []string{second, first}) || !equal(titles(items), []string{"File newest", "Index newest"}) {
		t.Fatalf("got %v %v", ids(items), titles(items))
	}
}

func TestIncludesNewerUnindexedSessions(t *testing.T) {
	f := newFixture(t)
	indexed, unindexed := "33333333-3333-4333-8333-333333333333", "44444444-4444-4444-8444-444444444444"
	f.write("session_index.jsonl", fmt.Sprintf(`{"id":%q,"thread_name":"Indexed session","updated_at":"2026-06-29T00:00:00Z"}`, indexed))
	f.write("sessions/2026/06/29/rollout-"+indexed+".jsonl", sessionFile(indexed, "2026-06-29T00:00:00Z", "Indexed file"))
	f.write("sessions/2026/06/29/rollout-"+unindexed+".jsonl", sessionFile(unindexed, "2026-07-01T00:00:00Z", "Unindexed newer file"))
	items := f.scan(5).Items
	if !equal(ids(items), []string{unindexed, indexed}) || !equal(titles(items), []string{"Unindexed newer file", "Indexed session"}) {
		t.Fatalf("got %v %v", ids(items), titles(items))
	}
}

func TestFallsBackToRecentFilesWithoutIndex(t *testing.T) {
	f := newFixture(t)
	oldID, newID := "55555555-5555-4555-8555-555555555555", "66666666-6666-4666-8666-666666666666"
	oldFile := f.write("sessions/2026/06/29/rollout-"+oldID+".jsonl", sessionFile(oldID, "2026-06-29T00:00:00Z", "Old fallback"))
	newFile := f.write("sessions/2026/06/29/rollout-"+newID+".jsonl", sessionFile(newID, "2026-06-29T00:01:00Z", "New fallback"))
	f.touch(oldFile, "2026-06-29T00:00:00Z")
	f.touch(newFile, "2026-06-29T00:10:00Z")
	items := f.scan(1).Items
	if !equal(ids(items), []string{newID}) || items[0].Title != "New fallback" {
		t.Fatalf("got %v %v", ids(items), titles(items))
	}
}

func TestMapsTimestampPrefixedRolloutNamesToIndexIDs(t *testing.T) {
	f := newFixture(t)
	indexedID := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	f.write("session_index.jsonl", fmt.Sprintf(`{"id":%q,"thread_name":"Index-backed candidate","updated_at":"2026-07-01T00:00:00Z"}`, indexedID))
	indexed := f.write("sessions/2026/06/29/rollout-2026-06-29T00-00-00-"+indexedID+".jsonl", sessionFile(indexedID, "2026-07-01T00:00:00Z", "File title"))
	f.touch(indexed, "2026-06-01T00:00:00Z")
	filler := mustTime(t, "2026-06-30T00:00:00Z")
	for i := 0; i < 60; i++ {
		id := fmt.Sprintf("bbbbbbbb-bbbb-4bbb-8bbb-%012d", i)
		p := f.write("sessions/2026/06/29/rollout-"+id+".jsonl", sessionFile(id, "2026-06-30T00:00:00Z", fmt.Sprintf("Filler %d", i)))
		if err := os.Chtimes(p, filler, filler); err != nil {
			t.Fatal(err)
		}
	}
	items := f.scan(1).Items
	if !equal(ids(items), []string{indexedID}) || items[0].Title != "Index-backed candidate" {
		t.Fatalf("got %v %v", ids(items), titles(items))
	}
}

func TestReadsLargeSessionFromFileEdges(t *testing.T) {
	f := newFixture(t)
	id := "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	filler := strings.Repeat("x", 180_000)
	f.write("sessions/2026/06/29/rollout-2026-06-29T00-00-00-"+id+".jsonl", fmt.Sprintf(`{"timestamp":"2026-06-29T00:00:00Z","type":"session_meta","payload":{"id":%q,"cwd":"/Users/me/dev/quosparr"}}
{"timestamp":"2026-06-29T00:01:00Z","type":"event_msg","payload":{"type":"message","role":"user","content":"Large edge scan"}}
{"timestamp":"2026-06-29T00:02:00Z","type":"event_msg","payload":{"type":"message","role":"assistant","content":%q}}
{"timestamp":"2026-07-01T00:00:00Z","type":"event_msg","payload":{"type":"approval_request"}}
{"timestamp":"2026-07-01T00:01:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":1000,"cached_input_tokens":200,"output_tokens":50,"reasoning_output_tokens":25,"total_tokens":1050}}},"turn_context":{"model":"gpt-5.5"}}`, id, filler))
	s := f.scan(1).Items[0]
	if s.ID != id || s.Title != "Large edge scan" || s.ProjectName != "quosparr" || s.State != StateNeedsAttention ||
		s.Totals.TotalTokens != 1050 || s.Totals.CachedInputTokens != 200 || s.Totals.OutputTokens != 50 {
		t.Fatalf("got %+v", s)
	}
}

func TestIgnoresStaleLargeFilesOutsideRecentCandidates(t *testing.T) {
	f := newFixture(t)
	recentID := "77777777-7777-4777-8777-777777777777"
	recent := f.write("sessions/2026/06/29/rollout-"+recentID+".jsonl", sessionFile(recentID, "2026-07-01T00:00:00Z", "Recent candidate"))
	f.touch(recent, "2026-07-01T00:00:00Z")
	fillerAt := mustTime(t, "2026-06-30T00:00:00Z")
	for i := 0; i < 55; i++ {
		id := fmt.Sprintf("99999999-9999-4999-8999-%012d", i)
		p := f.write("sessions/2026/06/29/rollout-"+id+".jsonl", sessionFile(id, "2026-06-30T00:00:00Z", fmt.Sprintf("Filler %d", i)))
		_ = os.Chtimes(p, fillerAt, fillerAt)
	}
	oldAt := mustTime(t, "2026-06-01T00:00:00Z")
	for i := 0; i < 10; i++ {
		id := fmt.Sprintf("88888888-8888-4888-8888-%012d", i)
		p := f.write("sessions/2026/06/29/rollout-"+id+".jsonl", fmt.Sprintf(`{"timestamp":"2035-01-01T00:00:00Z","type":"session_meta","payload":{"id":%q,"cwd":"/Users/me/dev/old-%d"}}
{"timestamp":"2035-01-01T00:00:01Z","type":"event_msg","payload":{"type":"message","role":"user","content":%q}}`, id, i, strings.Repeat("x", 8192)))
		_ = os.Chtimes(p, oldAt, oldAt)
	}
	if got := ids(f.scan(1).Items); !equal(got, []string{recentID}) {
		t.Fatalf("got %v", got)
	}
}

func TestCountsTaskCompleteAsTurns(t *testing.T) {
	f := newFixture(t)
	id := "019f17a5-436d-73b2-a93d-7af3e78cc829"
	token := func(ts string, in, cached, out int) string {
		return fmt.Sprintf(`{"timestamp":%q,"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":%d,"cached_input_tokens":%d,"output_tokens":%d,"reasoning_output_tokens":0}}}}`, ts, in, cached, out)
	}
	f.write("sessions/2026/06/29/rollout-"+id+".jsonl", strings.Join([]string{
		fmt.Sprintf(`{"timestamp":"2026-06-29T00:00:00Z","type":"session_meta","payload":{"id":%q,"cwd":"/Users/me/dev/quosparr"}}`, id),
		token("2026-06-29T00:01:00Z", 1000, 100, 50),
		token("2026-06-29T00:02:00Z", 800, 80, 20),
		`{"timestamp":"2026-06-29T00:03:00Z","type":"event_msg","payload":{"type":"task_complete"}}`,
	}, "\n")+"\n")
	s := f.scan(1).Items[0]
	if s.Totals.Turns != 1 || s.Totals.TotalTokens != 1870 {
		t.Fatalf("totals %+v", s.Totals)
	}
}

func TestSmallFileKeepsCumulativeUsageOverLastUsage(t *testing.T) {
	f := newFixture(t)
	id := "019f17a5-436d-73b2-a93d-7af3e78cc830"
	f.write("sessions/2026/06/29/rollout-"+id+".jsonl", fmt.Sprintf(`{"timestamp":"2026-06-29T00:00:00Z","type":"session_meta","payload":{"id":%q}}
{"timestamp":"2026-06-29T00:01:00Z","type":"turn_context","payload":{"model":"gpt-5.5"}}
{"timestamp":"2026-06-29T00:02:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":500,"cached_input_tokens":0,"output_tokens":5},"last_token_usage":{"input_tokens":9,"cached_input_tokens":0,"output_tokens":9}}}}
`, id))
	if s := f.scan(1).Items[0]; s.Totals.TotalTokens != 505 {
		t.Fatalf("totals %+v", s.Totals)
	}
}

func TestLimitZeroAndMissingHome(t *testing.T) {
	f := newFixture(t)
	if got := f.scan(0); got.Items == nil || len(got.Items) != 0 {
		t.Fatalf("limit 0: %#v", got)
	}
	if got := f.scan(5); len(got.Items) != 0 {
		t.Fatalf("empty home: %#v", got)
	}
}

func TestCRLFSessionFiles(t *testing.T) {
	f := newFixture(t)
	id := "019f17a5-436d-73b2-a93d-7af3e78cc831"
	f.write("sessions/2026/06/29/rollout-"+id+".jsonl", strings.ReplaceAll(sessionFile(id, "2026-06-29T00:00:00Z", "Windows line ends"), "\n", "\r\n"))
	if s := f.scan(1).Items[0]; s.Title != "Windows line ends" || s.Totals.TotalTokens != 15 {
		t.Fatalf("got %+v", s)
	}
}

func TestProjectName(t *testing.T) {
	str := func(s string) *string { return &s }
	for _, c := range []struct {
		in   *string
		want string
	}{
		{nil, UnknownProject},
		{str("  \n"), UnknownProject},
		{str("/Users/me/dev/quosparr"), "quosparr"},
		{str("/Users/me/dev/quosparr/"), "quosparr"},
		{str(`C:\Users\me\dev\quosparr`), "quosparr"},
		{str("/"), "/"},
		{str("project"), "project"},
	} {
		if got := ProjectName(c.in); got != c.want {
			t.Errorf("ProjectName(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestTitleTruncation(t *testing.T) {
	long := strings.Repeat("a", 100)
	if got := cleanTitle("  " + long + "\n"); got != strings.Repeat("a", 80) {
		t.Fatalf("len %d", len(got))
	}
	if cleanTitle(" \t\n") != "" {
		t.Fatal("blank title must be empty")
	}
	// A family emoji (ZWJ sequence), a flag and e + combining acute are one
	// character each, so 80 of them survive intact.
	for _, unit := range []string{"👨‍👩‍👧", "🇨🇳", "e\u0301", "👍🏽"} {
		in := strings.Repeat(unit, 85)
		if got := cleanTitle(in); got != strings.Repeat(unit, 80) {
			t.Errorf("unit %q: got %d bytes, want %d", unit, len(got), len(unit)*80)
		}
	}
}

func TestSessionID(t *testing.T) {
	for in, want := range map[string]string{
		"rollout-abc.jsonl": "abc",
		"rollout-2026-06-29T00-00-00-019f17a5-436d-73b2-a93d-7af3e78cc827.jsonl": "019f17a5-436d-73b2-a93d-7af3e78cc827",
		"other.jsonl":    "",
		"rollout-.jsonl": "",
	} {
		if got := sessionID(in); got != want {
			t.Errorf("sessionID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestContentArrayTitle(t *testing.T) {
	f := newFixture(t)
	id := "019f17a5-436d-73b2-a93d-7af3e78cc832"
	f.write("sessions/2026/06/29/rollout-"+id+".jsonl", fmt.Sprintf(`{"timestamp":"2026-06-29T00:00:00Z","type":"session_meta","payload":{"id":%q,"cwd":"/tmp/x"}}
{"timestamp":"2026-06-29T00:01:00Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"first"},{"type":"input_text","text":"second"}]}}`, id))
	if s := f.scan(1).Items[0]; s.Title != "first second" {
		t.Fatalf("title %q", s.Title)
	}
}
