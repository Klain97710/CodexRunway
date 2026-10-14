package grok

import (
	"testing"

	"github.com/licoy/quosparr/internal/usage/decimal"
)

// TestPricingMatchesSwift replays 1554 (model, tokens) combinations through
// GrokPricingTable.price / cost.
func TestPricingMatchesSwift(t *testing.T) {
	var inputs []struct {
		M       string
		I, C, O int
	}
	var golden []struct {
		Price []any
		Cost  *string
	}
	readJSON(t, "testdata/billing_pricing_in.json", &inputs)
	readJSON(t, "testdata/billing_pricing_out.json", &golden)
	if len(inputs) != len(golden) {
		t.Fatalf("%d inputs, %d goldens", len(inputs), len(golden))
	}
	for i, in := range inputs {
		want := golden[i]
		price, ok := PricingFor(in.M)
		if ok != (want.Price != nil) {
			t.Errorf("%q: priced %v, Swift %v", in.M, ok, want.Price != nil)
			continue
		}
		cost, costOK := PricingCost(in.M, in.I, in.C, in.O)
		if costOK != ok || (want.Cost != nil) != ok {
			t.Errorf("%q: cost presence %v, Swift %v", in.M, costOK, want.Cost != nil)
			continue
		}
		if !ok {
			continue
		}
		rates := []decimal.Decimal{price.InputPerMillion, price.CachedInputPerMillion, price.OutputPerMillion}
		for _, r := range []*decimal.Decimal{price.LongContextInputPerMillion, price.LongContextCachedInputPerMillion, price.LongContextOutputPerMillion} {
			if r == nil {
				rates = append(rates, decimal.Decimal{})
			} else {
				rates = append(rates, *r)
			}
		}
		for k, r := range rates {
			if s, isStr := want.Price[k].(string); isStr && r.String() != s {
				t.Errorf("%q: rate %d = %s, Swift %s", in.M, k, r, s)
			} else if !isStr && want.Price[k] != nil {
				t.Errorf("%q: rate %d odd golden %v", in.M, k, want.Price[k])
			}
		}
		if want.Price[3] == nil && price.LongContextInputPerMillion != nil {
			t.Errorf("%q: unexpected long context price", in.M)
		}
		if price.LongContextThresholdTokens != 200_000 {
			t.Errorf("%q: threshold %d", in.M, price.LongContextThresholdTokens)
		}
		if cost.String() != *want.Cost {
			t.Errorf("%q %d/%d/%d: cost %s, Swift %s", in.M, in.I, in.C, in.O, cost, *want.Cost)
		}
	}
}

func TestPricingPorts(t *testing.T) {
	d := decimal.MustParse
	check := func(name string, got decimal.Decimal, ok bool, want string) {
		t.Helper()
		if !ok || !got.Equal(d(want)) {
			t.Errorf("%s = %s (%v), want %s", name, got, ok, want)
		}
	}
	got, ok := PricingCost("grok-4.5-build", 1000, 400, 200)
	check("short grok-4.5", got, ok, "0.00252")
	got, ok = PricingCost("grok-4.5", 199_999, 0, 0)
	check("199999", got, ok, "0.399998")
	got, ok = PricingCost("grok-4.5", 200_000, 0, 0)
	check("200000", got, ok, "0.8")
	got, ok = PricingCost("grok-4.6", 200_000, 200_000, 0)
	check("long cached", got, ok, "0.2")
	got, ok = PricingCost("grok-4.7-build-fast", 200_000, 0, 0)
	check("fast long", got, ok, "1.2")
	if _, ok := PricingCost("mystery-model", 10_000, 0, 100); ok {
		t.Error("unknown model was priced")
	}
	if PricingVersion != "xai-builtin-2026-09-27" {
		t.Error(PricingVersion)
	}
}
