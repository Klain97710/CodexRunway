package grok

import (
	"errors"
	"testing"
	"time"
)

// Ports of the Swift releases' Grok billing tests.

func billingMustDecode(t *testing.T, body string) BillingSnapshot {
	t.Helper()
	s, err := DecodeBillingResponse([]byte(body), billingGoldenNow)
	if err != nil {
		t.Fatalf("decode %.80s: %v", body, err)
	}
	return s
}

func billingDate(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func ptrEq[T comparable](p *T, want T) bool { return p != nil && *p == want }

func TestBillingCurrentCreditsConfig(t *testing.T) {
	s := billingMustDecode(t, `{"config":{"creditUsagePercent":42.5,
	 "currentPeriod":{"type":"USAGE_PERIOD_TYPE_WEEKLY","start":"2026-06-01T00:00:00Z","end":"2026-06-08T00:00:00Z"},
	 "monthlyLimit":{"val":9999},"used":{"val":9999},"onDemandCap":{"val":5000},"onDemandUsed":{"val":300},
	 "prepaidBalance":{"val":1250},
	 "productUsage":[{"product":"GrokBuild","usagePercent":30.0},{"product":"GrokImagine","usagePercent":8.5},{"product":"GrokChat","usagePercent":4.0}],
	 "isUnifiedBillingUser":true},"on_demand_enabled":true,"subscription_tier":"SuperGrok Heavy"}`)
	if !ptrEq(s.Plan, "SuperGrok Heavy") || s.IncludedUsagePercent != 42.5 || s.Source != BillingSourceCurrent ||
		!s.UpdatedAt.Equal(billingGoldenNow) {
		t.Fatalf("%+v", s)
	}
	if s.Period == nil || s.Period.Kind != BillingWeekly ||
		!s.Period.StartsAt.Equal(billingDate(t, "2026-06-01T00:00:00Z")) || !s.Period.ResetsAt.Equal(billingDate(t, "2026-06-08T00:00:00Z")) {
		t.Fatalf("period %+v", s.Period)
	}
	if !ptrEq(s.IncludedLimitCents, 9999) || !ptrEq(s.IncludedUsedCents, 9999) || !ptrEq(s.PrepaidBalanceCents, 1250) ||
		!ptrEq(s.OnDemandEnabled, true) || !ptrEq(s.OnDemandUsedCents, 300) || !ptrEq(s.OnDemandLimitCents, 5000) ||
		!ptrEq(s.IsUnifiedBillingUser, true) {
		t.Fatalf("%+v", s)
	}
	names := []string{"GrokBuild", "GrokImagine", "GrokChat"}
	for i, p := range s.ProductUsage {
		if p.Product != names[i] {
			t.Fatalf("product order %+v", s.ProductUsage)
		}
	}
	if len(s.ProductUsage) != 3 || s.ProductUsage[1].UsagePercent != 8.5 {
		t.Fatalf("%+v", s.ProductUsage)
	}
}

func TestBillingPostResetWithoutPercentIsZero(t *testing.T) {
	s := billingMustDecode(t, `{"config":{"currentPeriod":{"type":"USAGE_PERIOD_TYPE_WEEKLY",
	 "start":"2026-08-06T16:46:56.082611+00:00","end":"2026-08-13T16:46:56.082611+00:00"},
	 "onDemandCap":{"val":0},"onDemandUsed":{"val":0},"isUnifiedBillingUser":true,"prepaidBalance":{"val":0}}}`)
	if s.IncludedUsagePercent != 0 || s.Period == nil || s.Period.Kind != BillingWeekly || s.Period.StartsAt == nil ||
		s.Period.ResetsAt == nil || len(s.ProductUsage) != 0 || !ptrEq(s.PrepaidBalanceCents, 0) || s.Source != BillingSourceCurrent {
		t.Fatalf("%+v", s)
	}
	if got := s.Period.ResetsAt.Sub(*s.Period.StartsAt); got != 7*24*time.Hour {
		t.Fatalf("period length %v", got)
	}
}

func TestBillingExplicitZeroStaysCurrent(t *testing.T) {
	s := billingMustDecode(t, `{"config":{"creditUsagePercent":0,"currentPeriod":{"type":"USAGE_PERIOD_TYPE_WEEKLY","start":"2026-08-06T16:46:56Z","end":"2026-08-13T16:46:56Z"}}}`)
	if s.IncludedUsagePercent != 0 || s.Period == nil || s.Source != BillingSourceCurrent {
		t.Fatalf("%+v", s)
	}
}

func TestBillingDeprecatedWrapper(t *testing.T) {
	s := billingMustDecode(t, `{"config":{"monthlyLimit":{"val":2000},"used":{"val":850},"onDemandCap":{"val":500},"onDemandUsed":{"val":25},
	 "billingPeriodStart":"2026-06-01T00:00:00Z","billingPeriodEnd":"2026-07-01T00:00:00Z"},"onDemandEnabled":false,"subscriptionTier":"SuperGrok"}`)
	if !ptrEq(s.Plan, "SuperGrok") || s.IncludedUsagePercent != 42.5 || s.Period == nil || s.Period.Kind != BillingMonthly ||
		!ptrEq(s.IncludedLimitCents, 2000) || !ptrEq(s.IncludedUsedCents, 850) || !ptrEq(s.OnDemandEnabled, false) ||
		!ptrEq(s.OnDemandUsedCents, 25) || !ptrEq(s.OnDemandLimitCents, 500) || s.Source != BillingSourceDeprecated {
		t.Fatalf("%+v", s)
	}
}

func TestBillingLegacyFlat(t *testing.T) {
	s := billingMustDecode(t, `{"billingCycle":{"billingPeriodStart":"2026-06-01T00:00:00Z","billingPeriodEnd":"2026-07-01T00:00:00Z"},
	 "monthlyLimit":{"val":4000},"onDemandCap":{"val":900},"on_demand_enabled":true,
	 "usage":{"includedUsed":{"val":1000},"onDemandUsed":{"val":75},"totalUsed":{"val":1200}}}`)
	if s.IncludedUsagePercent != 30 || s.Period == nil || s.Period.Kind != BillingMonthly || !ptrEq(s.OnDemandEnabled, true) ||
		!ptrEq(s.OnDemandUsedCents, 75) || !ptrEq(s.OnDemandLimitCents, 900) || s.Source != BillingSourceLegacyFlat {
		t.Fatalf("%+v", s)
	}
}

func TestBillingEmptyCentsAreZero(t *testing.T) {
	s := billingMustDecode(t, `{"config":{"creditUsagePercent":12.25,"currentPeriod":{"type":"USAGE_PERIOD_TYPE_MONTHLY","start":"2026-06-01T00:00:00Z","end":"2026-07-01T00:00:00Z"},
	 "monthlyLimit":{"val":100},"used":{"val":99},"prepaidBalance":{},"onDemandCap":{},"onDemandUsed":{}}}`)
	if s.IncludedUsagePercent != 12.25 || s.Period.Kind != BillingMonthly || !ptrEq(s.PrepaidBalanceCents, 0) ||
		!ptrEq(s.OnDemandUsedCents, 0) || !ptrEq(s.OnDemandLimitCents, 0) || s.Source != BillingSourceCurrent {
		t.Fatalf("%+v", s)
	}
}

func TestBillingBadPeriodKeepsPercent(t *testing.T) {
	for name, period := range map[string]string{
		"bad date":     `"type":"USAGE_PERIOD_TYPE_WEEKLY","start":"not-a-date","end":"2026-07-01T00:00:00Z"`,
		"unknown type": `"type":"USAGE_PERIOD_TYPE_DAILY","start":"2026-07-01T00:00:00Z","end":"2026-07-02T00:00:00Z"`,
	} {
		s := billingMustDecode(t, `{"config":{"creditUsagePercent":10,"currentPeriod":{`+period+`}}}`)
		if s.IncludedUsagePercent != 10 || s.Period != nil || s.Source != BillingSourceCurrent {
			t.Errorf("%s: %+v", name, s)
		}
	}
}

func TestBillingFractionalOffsetTimestamps(t *testing.T) {
	s := billingMustDecode(t, `{"config":{"currentPeriod":{"type":"USAGE_PERIOD_TYPE_WEEKLY","start":"2026-07-30T16:46:56.082611+00:00","end":"2026-08-06T16:46:56.082611+00:00"},
	 "creditUsagePercent":15.0,"productUsage":[{"product":"GrokBuild","usagePercent":15.0}],"isUnifiedBillingUser":true,"prepaidBalance":{"val":0}}}`)
	if s.IncludedUsagePercent != 15 || s.Period == nil || s.Period.StartsAt == nil || s.Period.ResetsAt == nil ||
		len(s.ProductUsage) != 1 || s.ProductUsage[0].Product != "GrokBuild" {
		t.Fatalf("%+v", s)
	}
}

func TestBillingMalformedProductUsageIsIgnored(t *testing.T) {
	s := billingMustDecode(t, `{"config":{"creditUsagePercent":33,"productUsage":{"GrokBuild":33}}}`)
	if s.IncludedUsagePercent != 33 || len(s.ProductUsage) != 0 || s.Source != BillingSourceCurrent {
		t.Fatalf("%+v", s)
	}
}

func TestBillingFailures(t *testing.T) {
	for name, body := range map[string]string{
		"empty config": `{"config": {}}`,
		"array":        `["not","an","object"]`,
		"overflow":     `{"config":{"creditUsagePercent":1e400}}`,
		"garbage":      `not json`,
	} {
		if _, err := DecodeBillingResponse([]byte(body), billingGoldenNow); !errors.Is(err, ErrBillingUnknownStructure) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := DecodeBillingResponse([]byte(`{"config":{"creditUsagePercent":"nan"}}`), billingGoldenNow); !errors.Is(err, ErrBillingInvalidPercent) {
		t.Errorf("nan percent: %v", err)
	}
}

func TestBillingStringPercentAndLegacyFallthrough(t *testing.T) {
	if s := billingMustDecode(t, `{"config":{"creditUsagePercent":"42.5"}}`); s.IncludedUsagePercent != 42.5 || s.Source != BillingSourceCurrent {
		t.Fatalf("%+v", s)
	}
	s := billingMustDecode(t, `{"config":{},"monthlyLimit":{"val":1000},"usage":{"totalUsed":{"val":250}}}`)
	if s.IncludedUsagePercent != 25 || s.Source != BillingSourceLegacyFlat {
		t.Fatalf("%+v", s)
	}
}

func TestBillingMoneyAllowanceAndSettingsPlan(t *testing.T) {
	limit, used, err := DecodeMoneyAllowance([]byte(`{"config":{"monthlyLimit":{"val":15000},"used":{"val":277},"billingPeriodStart":"2026-08-01T00:00:00+00:00"}}`))
	if err != nil || !ptrEq(limit, 15000) || !ptrEq(used, 277) {
		t.Fatalf("%v %v %v", limit, used, err)
	}
	for body, want := range map[string]string{
		`{"subscription_tier_display":"SuperGrok","force_update":false}`: "SuperGrok",
		`{"subscription_tier_display":"SuperGrok Heavy"}`:                "SuperGrok Heavy",
	} {
		if got, ok := DecodeSettingsPlan([]byte(body)); !ok || got != want {
			t.Errorf("%s: %q %v", body, got, ok)
		}
	}
}

func TestBillingDoesNotInventResetCards(t *testing.T) {
	s := billingMustDecode(t, `{"config":{"creditUsagePercent":12,"currentPeriod":{"type":"USAGE_PERIOD_TYPE_WEEKLY","start":"2026-08-06T00:00:00Z","end":"2026-08-13T00:00:00Z"},
	 "resetCredits":{"available_count":9},"tokens":[{"tokenId":"should-not-be-read-from-billing"}]}}`)
	if s.IncludedUsagePercent != 12 || s.ResetCredits != nil {
		t.Fatalf("%+v", s)
	}
}

func TestBillingMergesMoneyAllowance(t *testing.T) {
	s := billingMustDecode(t, `{"config":{"creditUsagePercent":6.0,"currentPeriod":{"type":"USAGE_PERIOD_TYPE_WEEKLY","start":"2026-07-30T16:46:56Z","end":"2026-08-06T16:46:56Z"}}}`)
	limit, used := int64(15000), int64(277)
	s = s.MergingMoneyAllowance(&limit, &used).MergingPlan("SuperGrok", true)
	remaining, ok := s.IncludedRemainingCents()
	if s.IncludedUsagePercent != 6 || !ptrEq(s.IncludedLimitCents, 15000) || !ptrEq(s.IncludedUsedCents, 277) ||
		!ok || remaining != 14723 || !ptrEq(s.Plan, "SuperGrok") {
		t.Fatalf("%+v", s)
	}
	// Existing values are kept; the plan is only filled when empty.
	other, otherUsed := int64(1), int64(2)
	again := s.MergingMoneyAllowance(&other, &otherUsed).MergingPlan("Free", false)
	if !ptrEq(again.IncludedLimitCents, 15000) || !ptrEq(again.Plan, "SuperGrok") {
		t.Fatalf("%+v", again)
	}
}

func TestBillingProductUsageOrder(t *testing.T) {
	s := billingMustDecode(t, `{"config":{"creditUsagePercent":1,"productUsage":[
	 {"product":"b","usagePercent":5},{"product":"  ","usagePercent":9},{"product":"A","usagePercent":5},{"product":"c"},{"product":"nan","usagePercent":"nan"},{"product":"hi","usagePercent":"7.5"}]}}`)
	var names []string
	for _, p := range s.ProductUsage {
		names = append(names, p.Product)
	}
	if got := names; len(got) != 4 || got[0] != "hi" || got[1] != "A" || got[2] != "b" || got[3] != "c" {
		t.Fatalf("%v", names)
	}
}

func TestBillingSnapshotJSONRoundTripWithResetCredits(t *testing.T) {
	start, end := billingDate(t, "2026-08-12T18:49:00Z"), billingDate(t, "2026-09-12T18:49:00Z")
	in := billingMustDecode(t, `{"config":{"creditUsagePercent":6.5}}`).MergingResetCredits(&ResetCreditSnapshot{
		AvailableCount: 1, UpdatedAt: billingGoldenNow,
		Tokens: []ResetCreditToken{{TokenID: "t/1", ValidityStart: &start, ValidityEnd: &end}, {TokenID: "t2"}},
	})
	raw, err := in.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	const want = `{"includedUsagePercent":6.5,"productUsage":[],"resetCredits":{"availableCount":1,"tokens":[{"tokenID":"t/1","validityEnd":"2026-09-12T18:49:00.000Z","validityStart":"2026-08-12T18:49:00.000Z"},{"tokenID":"t2"}],"updatedAt":"2027-01-15T08:00:00.000Z"},"source":"current","updatedAt":"2027-01-15T08:00:00.000Z"}`
	if string(raw) != want {
		t.Fatalf("got  %s\nwant %s", raw, want)
	}
	back, err := ParseBillingSnapshot(raw)
	if err != nil || back.ResetCredits == nil || len(back.ResetCredits.Tokens) != 2 || back.ResetCredits.Tokens[0].TokenID != "t/1" {
		t.Fatalf("%+v %v", back, err)
	}
	for _, bad := range []string{`{}`, `[]`, `{"includedUsagePercent":1,"source":"nope","updatedAt":"2027-01-15T08:00:00.000Z"}`,
		`{"includedUsagePercent":"1","source":"current","updatedAt":"2027-01-15T08:00:00.000Z"}`,
		`{"includedUsagePercent":1,"source":"current"}`} {
		if _, err := ParseBillingSnapshot([]byte(bad)); err == nil {
			t.Errorf("accepted %s", bad)
		}
	}
}

// Ports of GrokSubscriptionTierTests.

func TestTierJWTClaims(t *testing.T) {
	for claim, want := range map[uint64]string{0: "Free", 1: "SuperGrok", 5: "SuperGrok Heavy", 4: "X Premium+", 99: "Tier 99"} {
		if got := TierDisplayNameFromJWTClaim(claim); got != want {
			t.Errorf("claim %d: %q", claim, got)
		}
	}
	if TierFromJWTClaim(1) != TierSuperGrok || TierFromJWTClaim(5) != TierSuperGrokHeavy || TierFromJWTClaim(99) != TierUnknown {
		t.Error("claim resolution")
	}
}

func TestTierNames(t *testing.T) {
	for in, want := range map[string]string{"supergrok": "SuperGrok", "GrokPro": "SuperGrok", "SuperGrokPro": "SuperGrok Heavy", "SuperGrok Heavy": "SuperGrok Heavy"} {
		if got, ok := TierDisplayName(in); !ok || got != want {
			t.Errorf("%q: %q %v", in, got, ok)
		}
	}
	if _, ok := TierDisplayName("  "); ok {
		t.Error("blank name resolved")
	}
	if TierResolve("SuperGrok Lite") != TierSuperGrokLite || TierResolve("X Premium+") != TierXPremiumPlus ||
		TierResolve("api_key") != TierAPIKey || TierResolve("") != TierUnknown {
		t.Error("resolve")
	}
}

func TestTierFromAccessToken(t *testing.T) {
	const token = "eyJhbGciOiJub25lIn0.eyJ0aWVyIjoxfQ.sig" // payload {"tier":1}
	if got, ok := TierDisplayNameFromAccessToken(token); !ok || got != "SuperGrok" {
		t.Errorf("%q %v", got, ok)
	}
	if claim, ok := TierJWTClaim(token); !ok || claim != 1 {
		t.Errorf("claim %d %v", claim, ok)
	}
	if k, ok := TierFromAccessToken(token); !ok || k != TierSuperGrok {
		t.Errorf("%v %v", k, ok)
	}
	if _, ok := TierJWTClaim("no-dots"); ok {
		t.Error("token without payload accepted")
	}
}
