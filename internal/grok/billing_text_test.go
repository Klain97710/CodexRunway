package grok

import (
	"encoding/json"
	"os"
	"testing"
)

// TestBillingLocalizedCompareMatchesSwift replays the pairwise order of 160
// names (model ids, products, punctuation, digits, accents) through
// String.localizedCaseInsensitiveCompare. The comparison is an approximation
// of the system collation; the golden marks where it is exact.
func TestBillingLocalizedCompareMatchesSwift(t *testing.T) {
	var names, rows []string
	readJSON(t, "testdata/billing_collate_names.json", &names)
	readJSON(t, "testdata/billing_collate_golden.json", &rows)
	bad := 0
	for i, a := range names {
		for j, b := range names {
			want := rows[i][j]
			got := byte('=')
			if c := billingLocalizedCompare(a, b); c < 0 {
				got = '<'
			} else if c > 0 {
				got = '>'
			}
			if got != want {
				bad++
				if bad <= 20 {
					t.Errorf("compare(%q, %q) = %c, Swift %c", a, b, got, want)
				}
			}
		}
	}
	if bad > 0 {
		t.Errorf("%d of %d pairs differ", bad, len(names)*len(names))
	}
}

func readJSON(t *testing.T, path string, v any) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatal(err)
	}
}

// The accent level compares like the system collation: an accent followed by
// more text sorts before the plain spelling, a trailing one after it.
func TestBillingLocalizedCompareAccents(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want int
	}{
		{"éclair", "eclair", -1}, {"é", "e", 1}, {"éa", "ea", -1}, {"aé", "ae", 1}, {"éé", "ee", -1}, {"eé", "ee", 1},
		{"aéa", "aea", -1}, {"alpha", "Alpha", 0}, {"éf", "ée", 1},
	} {
		got := billingLocalizedCompare(tc.a, tc.b)
		if (got < 0) != (tc.want < 0) || (got > 0) != (tc.want > 0) {
			t.Errorf("compare(%q, %q) = %d, want sign %d", tc.a, tc.b, got, tc.want)
		}
	}
}
