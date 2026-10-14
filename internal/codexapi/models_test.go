package codexapi

import (
	"errors"
	"testing"
	"time"
)

const usageFixture = `{
  "plan_type": "pro",
  "rate_limit": {
    "primary_window": {"used_percent": 72, "reset_at": 1782711351, "limit_window_seconds": 18000},
    "secondary_window": {"used_percent": 40, "reset_at": 1783298151, "limit_window_seconds": 604800}
  },
  "credits": {"has_credits": true, "unlimited": false, "balance": "12.5"},
  "additional_rate_limits": [
    {
      "limit_name": "Codex Spark",
      "metered_feature": "spark",
      "rate_limit": {"primary_window": {"used_percent": 10, "reset_at": 1782711351, "limit_window_seconds": 18000}}
    }
  ]
}`

func TestDecodeQuotaWindowsAndAdditionalLimits(t *testing.T) {
	q, err := DecodeQuota([]byte(usageFixture), fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	if q.Plan != "pro" || q.Primary.UsedPercent != 72 || q.Primary.WindowMinutes != 300 {
		t.Fatalf("primary %+v plan %q", q.Primary, q.Plan)
	}
	if q.Secondary == nil || q.Secondary.WindowMinutes != 10_080 || q.Secondary.UsedPercent != 40 {
		t.Fatalf("secondary %+v", q.Secondary)
	}
	if q.CreditsBalance == nil || *q.CreditsBalance != 12.5 {
		t.Fatalf("credits %v", q.CreditsBalance)
	}
	if len(q.AdditionalWindows) != 1 || q.AdditionalWindows[0].Name != "Codex Spark" {
		t.Fatalf("additional %+v", q.AdditionalWindows)
	}
	if !q.Primary.ResetsAt.Equal(time.Unix(1_782_711_351, 0)) || !q.UpdatedAt.Equal(fixedNow) {
		t.Fatalf("times %v %v", q.Primary.ResetsAt, q.UpdatedAt)
	}
}

func TestDecodeQuotaFlexibleAndLenientFields(t *testing.T) {
	cases := []struct {
		name, body string
		check      func(t *testing.T, q QuotaSnapshot)
	}{
		{"fractional percent keeps exact value", `{"rate_limit":{"primary_window":{"used_percent":72.5,"reset_at":0,"limit_window_seconds":0}}}`,
			func(t *testing.T, q QuotaSnapshot) {
				if q.Primary.UsedPercent != 73 || q.Primary.UsedPercentExact != 72.5 || !q.Primary.ResetsAt.IsZero() || q.Primary.WindowMinutes != 0 {
					t.Fatalf("%+v", q.Primary)
				}
			}},
		{"string percent", `{"rate_limit":{"primary_window":{"used_percent":"10","reset_at":1,"limit_window_seconds":60}}}`,
			func(t *testing.T, q QuotaSnapshot) {
				if q.Primary.UsedPercent != 10 || q.Primary.WindowMinutes != 1 {
					t.Fatalf("%+v", q.Primary)
				}
			}},
		{"null secondary and credits", `{"rate_limit":{"primary_window":{"used_percent":1,"reset_at":1,"limit_window_seconds":60},"secondary_window":null},"credits":null}`,
			func(t *testing.T, q QuotaSnapshot) {
				if q.Secondary != nil || q.CreditsBalance != nil {
					t.Fatalf("%+v", q)
				}
			}},
		{"unparseable balance", `{"rate_limit":{"primary_window":{"used_percent":1,"reset_at":1,"limit_window_seconds":60}},"credits":{"balance":"n/a"}}`,
			func(t *testing.T, q QuotaSnapshot) {
				if q.CreditsBalance != nil {
					t.Fatal("balance should be nil")
				}
			}},
		{"malformed additional limits are ignored", `{"rate_limit":{"primary_window":{"used_percent":1,"reset_at":1,"limit_window_seconds":60}},"additional_rate_limits":"oops"}`,
			func(t *testing.T, q QuotaSnapshot) {
				if len(q.AdditionalWindows) != 0 {
					t.Fatal("expected none")
				}
			}},
		{"additional limit falls back to secondary window and default name", `{"rate_limit":{"primary_window":{"used_percent":1,"reset_at":1,"limit_window_seconds":60}},"additional_rate_limits":[{"rate_limit":{"secondary_window":{"used_percent":5,"reset_at":1,"limit_window_seconds":600}}},{"limit_name":"x"}]}`,
			func(t *testing.T, q QuotaSnapshot) {
				if len(q.AdditionalWindows) != 1 || q.AdditionalWindows[0].Name != "Codex extra limit" || q.AdditionalWindows[0].Window.UsedPercent != 5 {
					t.Fatalf("%+v", q.AdditionalWindows)
				}
			}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q, err := DecodeQuota([]byte(tc.body), fixedNow)
			if err != nil {
				t.Fatal(err)
			}
			tc.check(t, q)
		})
	}
}

func TestDecodeQuotaRejectsBrokenResponses(t *testing.T) {
	for _, body := range []string{``, `[]`, `{}`, `{"rate_limit":{}}`,
		`{"rate_limit":{"primary_window":{"used_percent":"x","reset_at":1,"limit_window_seconds":1}}}`,
		`{"rate_limit":{"primary_window":{"used_percent":1,"reset_at":"1","limit_window_seconds":1}}}`,
		`{"rate_limit":{"primary_window":{"used_percent":1,"reset_at":1.5,"limit_window_seconds":1}}}`,
		`{"rate_limit":{"primary_window":{"reset_at":1,"limit_window_seconds":1}}}`,
	} {
		if _, err := DecodeQuota([]byte(body), fixedNow); !errors.Is(err, ErrDecode) {
			t.Errorf("%q: err = %v", body, err)
		}
	}
}

func TestDecodeResetCredits(t *testing.T) {
	body := `{"available_count":1,"credits":[
	  {"id":"c1","status":"available","created_at":"2026-06-01T00:00:00Z","expires_at":"2026-07-01T00:00:00Z"},
	  {"id":"c2","status":"used","created_at":1780272000,"expires_at":1782864000}]}`
	now := time.Unix(1_782_000_000, 0)
	s, err := DecodeResetCredits([]byte(body), now)
	if err != nil {
		t.Fatal(err)
	}
	if s.AvailableCount != 1 || len(s.Credits) != 2 || s.Credits[0].Status != "available" || s.Credits[0].RemainingSeconds <= 0 {
		t.Fatalf("%+v", s)
	}
	if !s.Credits[1].ExpiresAt.Equal(time.Unix(1_782_864_000, 0)) || !s.Credits[1].CreatedAt.Equal(time.Unix(1_780_272_000, 0)) {
		t.Fatalf("epoch dates %+v", s.Credits[1])
	}
}

func TestDecodeResetCreditsShapes(t *testing.T) {
	now := time.Unix(1_000, 0)
	list, err := DecodeResetCredits([]byte(`[{"id":"a","status":"available"},{"id":"b"}]`), now)
	if err != nil || list.AvailableCount != 1 || list.Credits[1].Status != "unknown" {
		t.Fatalf("bare list: %+v %v", list, err)
	}
	object, err := DecodeResetCredits([]byte(`{"credits":[{"status":"available","expires_at":"bad"},{"status":"available","expires_at":900}]}`), now)
	if err != nil || object.AvailableCount != 2 || !object.Credits[0].ExpiresAt.IsZero() || object.Credits[1].RemainingSeconds != 0 {
		t.Fatalf("derived count: %+v %v", object, err)
	}
	broken, err := DecodeResetCredits([]byte(`{"available_count":3,"credits":"nope"}`), now)
	if err != nil || broken.AvailableCount != 3 || len(broken.Credits) != 0 {
		t.Fatalf("malformed credits: %+v %v", broken, err)
	}
	if _, err := DecodeResetCredits([]byte(`{"available_count":"3"}`), now); !errors.Is(err, ErrDecode) {
		t.Fatalf("string count: %v", err)
	}
	if _, err := DecodeResetCredits([]byte(`[{"id":5}]`), now); !errors.Is(err, ErrDecode) {
		t.Fatalf("bad item: %v", err)
	}
}

func TestCycleWindowsPreferLongestCompleteWindow(t *testing.T) {
	weekly := fixedNow.Add(72 * time.Hour)
	five := fixedNow.Add(2 * time.Hour)
	q := QuotaSnapshot{
		Primary:   RateWindow{UsedPercent: 10, WindowMinutes: 300, ResetsAt: five},
		Secondary: &RateWindow{UsedPercent: 40, WindowMinutes: 10_080, ResetsAt: weekly},
	}
	full, elapsed, ok := q.CycleWindows(fixedNow)
	if !ok || !full.End.Equal(weekly) || full.End.Sub(full.Start) != 10_080*time.Minute || !elapsed.End.Equal(fixedNow) {
		t.Fatalf("%+v %+v %v", full, elapsed, ok)
	}
}

func TestCycleWindowsFallBackToPrimaryWhenSecondaryIncomplete(t *testing.T) {
	weekly := fixedNow.Add(48 * time.Hour)
	q := QuotaSnapshot{
		Primary:   RateWindow{UsedPercent: 72, WindowMinutes: 10_080, ResetsAt: weekly},
		Secondary: &RateWindow{UsedPercent: 5},
	}
	full, elapsed, ok := q.CycleWindows(fixedNow)
	if !ok || !full.End.Equal(weekly) || !full.Start.Equal(weekly.Add(-10_080*time.Minute)) || !elapsed.End.Equal(fixedNow) {
		t.Fatalf("%+v %+v %v", full, elapsed, ok)
	}
	if _, _, ok := (QuotaSnapshot{}).CycleWindows(fixedNow); ok {
		t.Fatal("no complete window must report unavailable")
	}
}

func TestNextDueResetFiresOneSecondAfterOnce(t *testing.T) {
	primary, weekly := time.Unix(1_000, 0), time.Unix(2_000, 0)
	q := QuotaSnapshot{
		Primary:   RateWindow{ResetsAt: primary},
		Secondary: &RateWindow{ResetsAt: weekly},
	}
	at := func(sec float64) time.Time { return time.Unix(0, int64(sec*1e9)) }
	if !q.NextDueReset(time.Time{}, at(1000.9)).IsZero() {
		t.Fatal("due before one second")
	}
	if got := q.NextDueReset(time.Time{}, at(1001)); !got.Equal(primary) {
		t.Fatalf("got %v", got)
	}
	if !q.NextDueReset(primary, at(1001)).IsZero() {
		t.Fatal("already triggered reset fired again")
	}
	if got := q.NextDueReset(primary, at(2001)); !got.Equal(weekly) {
		t.Fatalf("got %v", got)
	}
}

func TestDecodeTokenUsageProfile(t *testing.T) {
	ok := `{"stats":{"daily_usage_buckets":[{"start_date":"2026-06-01","tokens":10},{"start_date":"2026-06-02","tokens":0}]},
	  "metadata":{"stats_as_of":"2026-06-02","generated_at":"2026-06-02T01:02:03.5Z"}}`
	p, err := DecodeTokenUsageProfile([]byte(ok))
	if err != nil || p.DailyTokens["2026-06-01"] != 10 || p.StatsAsOf != "2026-06-02" || p.GeneratedAt.IsZero() {
		t.Fatalf("%+v %v", p, err)
	}
	bad := []string{
		`{"stats":{}}`,
		`{"stats":{"daily_usage_buckets":[]},"metadata":{"stats_error":"boom"}}`,
		`{"stats":{"daily_usage_buckets":[{"start_date":"2026-02-30","tokens":1}]}}`,
		`{"stats":{"daily_usage_buckets":[{"start_date":"2026-6-1","tokens":1}]}}`,
		`{"stats":{"daily_usage_buckets":[{"start_date":"2026-06-01","tokens":-1}]}}`,
		`{"stats":{"daily_usage_buckets":[{"start_date":"2026-06-01","tokens":1},{"start_date":"2026-06-01","tokens":2}]}}`,
		`{"stats":{"daily_usage_buckets":[]},"metadata":{"stats_as_of":"yesterday"}}`,
		`{"stats":{"daily_usage_buckets":[]},"metadata":{"generated_at":"soon"}}`,
		`{"metadata":{}}`,
	}
	for _, body := range bad {
		if _, err := DecodeTokenUsageProfile([]byte(body)); !errors.Is(err, ErrDecode) {
			t.Errorf("%s: err = %v", body, err)
		}
	}
}
