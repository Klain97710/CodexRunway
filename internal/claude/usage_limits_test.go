package claude

import (
	"testing"
	"time"
)

func TestParseUsageKindPercentLimits(t *testing.T) {
	raw := []byte(`{
		"five_hour":{"utilization":1},
		"seven_day":{"utilization":2},
		"limits":[
			{"kind":"session","group":"session","percent":3,"resets_at":"2026-10-15T06:30:00.201870+00:00","is_active":false},
			{"kind":"weekly_all","group":"weekly","percent":72,"resets_at":"2026-10-17T19:00:00.201892+00:00","is_active":true},
			{"kind":"future_window","percent":17}
		]
	}`)
	snap, err := ParseUsageSnapshot(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Windows) != 3 {
		t.Fatalf("got %d windows, want 3", len(snap.Windows))
	}
	want := []struct {
		key   string
		kind  WindowKind
		used  float64
		reset string
	}{
		{"session", WindowFiveHour, 3, "2026-10-15T06:30:00.201870+00:00"},
		{"weekly_all", WindowSevenDay, 72, "2026-10-17T19:00:00.201892+00:00"},
		{"future_window", WindowOther, 17, ""},
	}
	for i, w := range snap.Windows {
		x := want[i]
		if w.Key != x.key || w.Kind != x.kind || w.UsedPercent == nil || *w.UsedPercent != x.used || w.Null {
			t.Fatalf("window %d: %+v", i, w)
		}
		if x.reset != "" {
			reset, _ := time.Parse(time.RFC3339Nano, x.reset)
			if w.ResetsAt == nil || !w.ResetsAt.Equal(reset) {
				t.Fatalf("window %s lost reset timestamp", w.Key)
			}
		}
	}
}

func TestParseUsageLimitsFallback(t *testing.T) {
	for _, tc := range []struct {
		name, limits string
		want         int
	}{
		{"unrecognized entries", `[{"label":"new schema","value":9}]`, 1},
		{"non-object entries", `["future"]`, 1},
		{"explicit empty array", `[]`, 0},
		{"explicit empty object", `{}`, 0},
		{"null limits", `null`, 1},
		{"valid limits stay authoritative", `[{"kind":"session","percent":9}]`, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			snap, err := ParseUsageSnapshot([]byte(`{"five_hour":{"utilization":3},"limits":` + tc.limits + `}`))
			if err != nil {
				t.Fatal(err)
			}
			if len(snap.Windows) != tc.want {
				t.Fatalf("got %d windows, want %d", len(snap.Windows), tc.want)
			}
			if tc.want == 1 {
				want := float64(3)
				if tc.name == "valid limits stay authoritative" {
					want = 9
				}
				w := snap.Windows[0]
				if w.Kind != WindowFiveHour || w.UsedPercent == nil || *w.UsedPercent != want {
					t.Fatalf("unexpected window: %+v", w)
				}
			}
		})
	}
}

func TestParseUsagePercentZeroAndNull(t *testing.T) {
	snap, err := ParseUsageSnapshot([]byte(`{"limits":[{"kind":"session","percent":0},{"kind":"weekly_all","percent":null}],"seven_day":{"utilization":99}}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Windows) != 2 {
		t.Fatalf("got %d windows, want 2", len(snap.Windows))
	}
	if w := snap.Windows[0]; w.Null || w.UsedPercent == nil || *w.UsedPercent != 0 {
		t.Fatalf("zero utilization lost: %+v", w)
	}
	if w := snap.Windows[1]; !w.Null || w.UsedPercent != nil {
		t.Fatalf("explicit null must stay authoritative: %+v", w)
	}
}
