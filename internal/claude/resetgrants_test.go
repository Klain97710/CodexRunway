package claude

import (
	"testing"
	"time"
)

func TestParseUsageResetGrants(t *testing.T) {
	snap, err := ParseUsageSnapshot([]byte(`{
		"five_hour":{"utilization":12},
		"cedar_ember":{
			"eligible":true,"at_limit":false,"next_grant_id":"b_grant",
			"grants":[
				{"id":"b_grant","label":"Weekly","resets_total":3,"resets_left":2,
				 "starts_at":"2026-10-01T00:00:00Z","ends_at":"2026-10-20T00:00:00Z",
				 "clears":["five_hour"],"paused":false,"usable_now":false},
				{"id":"a_grant","resets_total":1,"resets_left":1,"ends_at":null}
			]
		}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Windows) != 1 || snap.Windows[0].Kind != WindowFiveHour {
		t.Fatalf("cedar_ember must not become a window: %+v", snap.Windows)
	}
	g := snap.ResetGrants
	if g == nil || !g.Eligible || len(g.Grants) != 2 || g.Grants[0].ID != "a_grant" {
		t.Fatalf("grants: %+v", g)
	}
	b := g.Grants[1]
	if b.ResetsTotal != 3 || b.ResetsLeft != 2 || !b.EndsAt.Equal(time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("grant b: %+v", b)
	}
	none, err := ParseUsageSnapshot([]byte(`{"five_hour":{"utilization":1}}`))
	if err != nil || none.ResetGrants != nil {
		t.Fatal(err, none.ResetGrants)
	}
}

func TestParseResetGrantsRejectsMalformedBlock(t *testing.T) {
	for _, raw := range []string{
		`{"cedar_ember":{"grants":[]}}`,
		`{"cedar_ember":{"eligible":"yes"}}`,
		`{"cedar_ember":{"eligible":true,"grants":{}}}`,
		`{"cedar_ember":{"eligible":true,"grants":[{"id":"Bad ID","resets_total":1,"resets_left":1}]}}`,
		`{"cedar_ember":{"eligible":true,"grants":[{"id":"g","resets_total":1,"resets_left":2}]}}`,
		`{"cedar_ember":{"eligible":true,"grants":[{"id":"g","resets_total":1.5,"resets_left":1}]}}`,
		`{"cedar_ember":{"eligible":true,"grants":[{"id":"g","resets_total":1,"resets_left":-1}]}}`,
		`{"cedar_ember":{"eligible":true,"grants":[{"id":"g","resets_total":1,"resets_left":1,"ends_at":"soon"}]}}`,
		`{"cedar_ember":{"eligible":true,"grants":[{"id":"g","resets_total":1,"resets_left":1,"paused":"no"}]}}`,
		`{"cedar_ember":{"eligible":true,"grants":[{"id":"g","resets_total":1,"resets_left":1},{"id":"g","resets_total":1,"resets_left":1}]}}`,
	} {
		snap, err := ParseUsageSnapshot([]byte(raw))
		if err != nil {
			t.Fatal(raw, err)
		}
		if snap.ResetGrants != nil {
			t.Fatalf("%s: malformed block kept: %+v", raw, snap.ResetGrants)
		}
	}
}

func TestResetGrantsCodexSnapshot(t *testing.T) {
	now := time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC)
	ends := now.Add(48 * time.Hour)
	s := ResetGrantStatus{Eligible: true, Grants: []ResetGrant{
		{ID: "a", ResetsTotal: 3, ResetsLeft: 2, EndsAt: ends},
		{ID: "b", ResetsTotal: 1, ResetsLeft: 1, Paused: true},
		{ID: "c", ResetsTotal: 1, ResetsLeft: 1, StartsAt: now.Add(time.Hour)},
		{ID: "d", ResetsTotal: 1, ResetsLeft: 1, EndsAt: now.Add(-time.Hour)},
	}}
	c := s.CodexSnapshot(now, now)
	want := []string{"available", "available", "used", "unavailable", "unavailable", "unavailable"}
	if c.AvailableCount != 2 || len(c.Credits) != len(want) {
		t.Fatalf("snapshot: %+v", c)
	}
	for i, status := range want {
		if c.Credits[i].Status != status {
			t.Fatalf("credit %d: %+v", i, c.Credits[i])
		}
	}
	if c.Credits[0].ID != "a-1" || c.Credits[0].RemainingSeconds != 48*3600 || c.Credits[3].RemainingSeconds != 0 {
		t.Fatalf("credit timing: %+v", c.Credits)
	}
	s.Eligible = false
	if c := s.CodexSnapshot(now, now); c.AvailableCount != 0 {
		t.Fatalf("ineligible account offered resets: %+v", c)
	}
	if !s.Shown() || (ResetGrantStatus{}).Shown() {
		t.Fatal("shown")
	}
	big := ResetGrantStatus{Eligible: true, Grants: []ResetGrant{{ID: "x", ResetsTotal: 1 << 30, ResetsLeft: 1 << 30}}}
	if n := len(big.CodexSnapshot(now, now).Credits); n != maxGrantResets {
		t.Fatalf("expansion not bounded: %d", n)
	}
}
