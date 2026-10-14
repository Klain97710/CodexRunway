package grok

import (
	"encoding/json"
	"math"
	"os"
	"regexp"
	"strconv"
	"testing"
)

// billingDateExotic matches the mutated inputs whose system-parser result is
// an artefact of its number handling and not worth reproducing: exponents on
// the year, in a fraction or on a huge seconds value, a fraction of ten or
// more digits (the system parser overflows a 32-bit integer), and a dash
// right behind a separator that the system parser reads as an ignorable.
var billingDateExotic = regexp.MustCompile(`^\s*-?\d*[eE]|\.\d*[eE]|:\d+[eE][+-]?\d{4,}|\.\d{10,}|\.-|\s--`)

// TestBillingParseDateMatchesSwift replays the frozen results of the Swift
// releases' ISO8601DateFormatter: 4000 mutated timestamps plus hand-picked
// edge cases.
func TestBillingParseDateMatchesSwift(t *testing.T) {
	raw, err := os.ReadFile("testdata/billing_dates_golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		In  string  `json:"in"`
		Out *string `json:"out"`
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	bad := 0
	skipped := 0
	for _, c := range cases {
		if billingDateExotic.MatchString(c.In) {
			skipped++
			continue
		}
		got, ok := BillingParseDate(c.In)
		if c.Out == nil {
			if ok {
				bad++
				if bad <= 40 {
					t.Errorf("%q: parsed %v, Swift rejects", c.In, got)
				}
			}
			continue
		}
		want, _ := strconv.ParseFloat(*c.Out, 64)
		if !ok {
			bad++
			if bad <= 40 {
				t.Errorf("%q: rejected, Swift parses to %s", c.In, *c.Out)
			}
			continue
		}
		wantMs := int64(math.Round(want * 1000))
		if gotMs := got.UnixMilli(); gotMs != wantMs {
			bad++
			if bad <= 40 {
				t.Errorf("%q: %d ms, Swift %d ms", c.In, gotMs, wantMs)
			}
		}
	}
	if skipped > 200 {
		t.Errorf("%d inputs skipped as exotic, expected only a handful", skipped)
	}
	if bad > 0 {
		t.Errorf("%d of %d inputs differ", bad, len(cases))
	}
}
