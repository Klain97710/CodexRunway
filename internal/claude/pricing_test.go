package claude

import (
	"testing"

	"github.com/licoy/quosparr/internal/usage/decimal"
)

func TestNormalizeAndPrice(t *testing.T) {
	cases := []struct{ in, want string }{
		{" US.Anthropic.Claude-Haiku-4-5-20251001-v1:0 ", "claude-haiku-4-5"},
		{"claude-sonnet-4-6@20251001", "claude-sonnet-4-6"},
		{"claude-opus-4-5[1m]", "claude-opus-4-5"},
		{"global.anthropic.claude-3-5-haiku", "claude-3-5-haiku"},
		{"eu.anthropic.claude-haiku-3-5-v1:0", "claude-haiku-3-5"},
		{"apac.claude-opus-4-1-20250101", "claude-opus-4-1"},
	}
	for _, tc := range cases {
		if got := NormalizeModel(tc.in); got != tc.want {
			t.Fatalf("%s -> %s want %s", tc.in, got, tc.want)
		}
	}
	if PricingVersion != "anthropic-builtin-2026-10-14" {
		t.Fatal(PricingVersion)
	}
	mustPrice(t, "claude-opus-5-5", TokenParts{Output: 1_000_000, Fast: true}, "", "40")
	mustPrice(t, "claude-opus-5-5", TokenParts{Output: 1_000_000, Fast: true}, "us", "44")
	mustPrice(t, "claude-opus-4-6", TokenParts{Output: 1_000_000, Fast: true}, "", "25")
	if _, ok := PricingCost("claude-opus-4-7", TokenParts{Output: 1, Fast: true}, ""); ok {
		t.Fatal("opus 4.7 fast priced")
	}
	if _, ok := PricingCost("claude-not-a-model", TokenParts{Output: 10}, ""); ok {
		t.Fatal("unknown priced")
	}
	mustPrice(t, "us.anthropic.claude-haiku-4-5-20251001-v1:0", TokenParts{Input: 1_000_000}, "", "1")
	mustPrice(t, "claude-sonnet-4-6", TokenParts{Input: 1_000_000}, "US", "3.3")
	mustPrice(t, "claude-sonnet-4-6", TokenParts{Input: 1_000_000}, "eu", "3")
	mustPrice(t, "claude-haiku-5-5", TokenParts{Input: 100_000}, "", "0.01")
	mustPrice(t, "claude-haiku-5-5", TokenParts{Input: 100_001}, "", "0.0500005")
	mustPrice(t, "claude-opus-4-1", TokenParts{Input: 1_000_000}, "us", "15")
	mustPrice(t, "claude-opus-4-20250514", TokenParts{Output: 1_000_000}, "", "75")
	mustPrice(t, "claude-sonnet-4-20250514", TokenParts{Input: 100_000}, "", "0.3")
	mustPrice(t, "claude-sonnet-4-5-20250929", TokenParts{Input: 200_000, Output: 1_000_000}, "", "15.6")
	mustPrice(t, "claude-sonnet-4-5-20250929", TokenParts{Input: 200_001, Output: 1_000_000}, "", "23.700006")
}

func mustPrice(t *testing.T, model string, parts TokenParts, geo, want string) {
	t.Helper()
	got, ok := PricingCost(model, parts, geo)
	if !ok {
		t.Fatalf("%s not priced", model)
	}
	if !got.Equal(decimal.MustParse(want)) {
		t.Fatalf("%s geo %s = %s want %s", model, geo, got, want)
	}
}
