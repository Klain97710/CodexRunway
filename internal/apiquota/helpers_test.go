package apiquota

import (
	"encoding/json"
	"math"
	"testing"
	"time"
)

var testNow = time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)

func fp(v float64) *float64 { return &v }

func mustJSON(t *testing.T, body string) any {
	t.Helper()
	v, ok := decodeJSON(body)
	if !ok {
		t.Fatalf("invalid test JSON: %s", body)
	}
	return v
}

func wantFloat(t *testing.T, name string, got *float64, want float64) {
	t.Helper()
	if got == nil {
		t.Fatalf("%s = nil, want %v", name, want)
	}
	if math.Abs(*got-want) > 1e-9*math.Max(1, math.Abs(want)) {
		t.Fatalf("%s = %v, want %v", name, *got, want)
	}
}

func wantNil(t *testing.T, name string, got *float64) {
	t.Helper()
	if got != nil {
		t.Fatalf("%s = %v, want nil", name, *got)
	}
}

func parseToken(t *testing.T, body string) tokenUsage {
	t.Helper()
	tok, ok := parseTokenUsage(mustJSON(t, body))
	if !ok {
		t.Fatalf("token body not parsed: %s", body)
	}
	return tok
}

func parseStatus(t *testing.T, body string) quotaStatus {
	t.Helper()
	s, ok := parseQuotaStatus(mustJSON(t, body))
	if !ok {
		t.Fatalf("status body not parsed: %s", body)
	}
	return s
}

// interpret runs one round with the token hit already normalized.
func interpret(grants, sub, usage, token hit) Quota {
	r := round{
		urls:     billingURLs{creditGrants: "g", subscription: "s", usage: "u"},
		tokenURL: "t",
		grants:   grants, sub: sub, usage: usage, token: token,
	}
	return r.interpret(testNow)
}

func notFound() hit { return hit{kind: hitNotFound} }

func unsupported() hit { return hit{kind: hitUnsupported} }

func subHit(limit *float64, expires int64) hit {
	return hit{kind: hitSubscription, sub: subscription{limitUSD: limit, expiresAt: expires}}
}

func usageHit(total float64) hit { return hit{kind: hitUsage, usage: usage{totalUsage: total}} }

func grantsHit(remaining, used, total float64) hit {
	return hit{kind: hitGrants, grants: grants{remaining: &remaining, used: &used, total: &total}}
}

func marshal(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
