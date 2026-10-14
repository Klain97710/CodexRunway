package usage

import (
	"bufio"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/usage/decimal"
)

func TestProgressFractionAndActivity(t *testing.T) {
	if _, ok := ProgressPreparing().Fraction(); ok {
		t.Fatal("preparing has no fraction")
	}
	if _, ok := ProgressRefreshingIndex(0, 0, "").Fraction(); ok {
		t.Fatal("zero total has no fraction")
	}
	if f, ok := ProgressRefreshingIndex(2, 4, "").Fraction(); !ok || f != 0.5 {
		t.Fatalf("got %v %v", f, ok)
	}
	if f, ok := ProgressRefreshingIndex(9, 4, "").Fraction(); !ok || f != 1 {
		t.Fatalf("got %v %v", f, ok)
	}
	for _, p := range []Progress{ProgressPreparing(), ProgressRefreshingIndex(1, 2, ""), ProgressAggregating(0, 1), ProgressFetchingOnline()} {
		if !p.IsActive() {
			t.Fatalf("%v should be active", p.Phase)
		}
	}
	for _, p := range []Progress{ProgressIdle(), ProgressFinished(), ProgressFailed("x")} {
		if p.IsActive() {
			t.Fatalf("%v should be inactive", p.Phase)
		}
	}
}

func TestProgressReporterThrottlesIndexUpdates(t *testing.T) {
	var mu sync.Mutex
	var phases []Phase
	r := NewProgressReporter(time.Second, nil)
	r.SetHandler(func(p Progress) {
		mu.Lock()
		phases = append(phases, p.Phase)
		mu.Unlock()
	})
	r.Report(ProgressRefreshingIndex(0, 10, ""), true)
	r.Report(ProgressRefreshingIndex(1, 10, ""), false)
	r.Report(ProgressRefreshingIndex(2, 10, ""), false)
	r.Report(ProgressAggregating(1, 1), true)
	n := 0
	for _, p := range phases {
		if p == PhaseRefreshingIndex {
			n++
		}
	}
	if n != 1 || phases[len(phases)-1] != PhaseAggregating {
		t.Fatalf("phases %v", phases)
	}
	var nilReporter *ProgressReporter
	nilReporter.Report(ProgressPreparing(), true) // must not panic
}

func TestProjectDisplayName(t *testing.T) {
	cases := map[string]string{
		"/Users/me/dev/quosparr":  "quosparr",
		"/Users/me/dev/quosparr/": "quosparr",
		"  /tmp/x \n":             "x",
		"":                        UnknownProject,
		"   ":                     UnknownProject,
		"/":                       "/",
		"relative/dir":            "dir",
		`C:\Users\me\proj`:        "proj",
		`\\server\share\proj`:     "proj",
		"/a/..":                   "..",
	}
	for in, want := range cases {
		if got := ProjectDisplayName(in); got != want {
			t.Errorf("%q: got %q, want %q", in, got, want)
		}
	}
}

// TestBuiltInPricesMatchSwiftGolden compares every entry of the built-in
// table with the table printed by the Swift app (testdata/prices_golden.txt:
// model, input, cachedInput, cacheWrite, output, long-context x4, "-" = nil).
func TestBuiltInPricesMatchSwiftGolden(t *testing.T) {
	f, err := os.Open("testdata/prices_golden.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	got := BuiltInPrices()
	seen := map[string]bool{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) != 9 {
			t.Fatalf("bad golden line %q", sc.Text())
		}
		model := fields[0]
		seen[model] = true
		p, ok := got[model]
		if !ok {
			t.Errorf("%s missing from the Go table", model)
			continue
		}
		cells := []struct {
			name string
			have *decimal.Decimal
		}{
			{"input", &p.InputPerMillion}, {"cached", &p.CachedInputPerMillion},
			{"write", p.CacheWritePerMillion}, {"output", &p.OutputPerMillion},
			{"lc input", p.LongContextInputPerMillion}, {"lc cached", p.LongContextCachedInputPerMillion},
			{"lc write", p.LongContextCacheWritePerMillion}, {"lc output", p.LongContextOutputPerMillion},
		}
		for i, c := range cells {
			want := fields[i+1]
			switch {
			case want == "-" && c.have != nil:
				t.Errorf("%s %s: got %s, want nil", model, c.name, c.have)
			case want != "-" && c.have == nil:
				t.Errorf("%s %s: got nil, want %s", model, c.name, want)
			case want != "-" && c.have.String() != want:
				t.Errorf("%s %s: got %s, want %s", model, c.name, c.have, want)
			}
		}
	}
	for m := range got {
		if !seen[m] {
			t.Errorf("%s in the Go table but not in the Swift golden", m)
		}
	}
	if PricingVersion != "openai-builtin-2026-09-27" {
		t.Errorf("pricing version %q", PricingVersion)
	}
}

func TestUnknownModelIsNeverEstimated(t *testing.T) {
	for _, m := range []string{"unknown-model", "codex-auto-review", "", "gpt-5.5-turbo-x", "gpt-7"} {
		if _, ok := CostForUsage(m, TokenUsage{InputTokens: 10}); ok {
			t.Errorf("%q must not be priced", m)
		}
		if _, ok := CurrentPriceBook().Cost(m, Totals{UncachedInputTokens: 10}); ok {
			t.Errorf("%q must not be priced by the book", m)
		}
	}
}

func TestBundledLookupIsExactExceptForDatedSnapshots(t *testing.T) {
	for _, m := range []string{"gpt-6", "gpt-6-astra-mini", "gpt-6-solstice", "gpt-6-luna-mini", "gpt-5.6-solstice"} {
		if _, ok := PriceFor(m); ok {
			t.Errorf("%s must be unknown", m)
		}
	}
	for m, want := range map[string][3]string{
		"gpt-5.6-sol":            {"4", "0.4", "20"},
		"gpt-5.6":                {"4", "0.4", "20"},
		"gpt-5.6-sol-2026-08-13": {"4", "0.4", "20"},
		"gpt-5.5-pro":            {"30", "30", "180"},
		"gpt-5.3-codex-spark":    {"1.75", "0.175", "14"},
	} {
		p, ok := PriceFor(m)
		if !ok {
			t.Errorf("%s missing", m)
			continue
		}
		if p.InputPerMillion.String() != want[0] || p.CachedInputPerMillion.String() != want[1] || p.OutputPerMillion.String() != want[2] {
			t.Errorf("%s: %s %s %s", m, p.InputPerMillion, p.CachedInputPerMillion, p.OutputPerMillion)
		}
	}
}
