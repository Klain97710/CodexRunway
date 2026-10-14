package grok

import (
	"fmt"
	"testing"
	"time"
)

var billingGoldenNow = time.Unix(1_800_000_000, 0)

func billingSwiftErr(err error) string {
	switch err {
	case ErrBillingUnknownStructure:
		return "unknownStructure"
	case ErrBillingInvalidPercent:
		return "invalidPercentage"
	}
	return err.Error()
}

func billingCentPair(limit, used *int64) []any {
	f := func(v *int64) any {
		if v == nil {
			return nil
		}
		return fmt.Sprint(*v)
	}
	return []any{f(limit), f(used)}
}

func replayBillingGolden(t *testing.T, inFile, outFile string) {
	t.Helper()
	var inputs []string
	var golden []map[string]any
	readJSON(t, inFile, &inputs)
	readJSON(t, outFile, &golden)
	if len(inputs) != len(golden) {
		t.Fatalf("%d inputs, %d goldens", len(inputs), len(golden))
	}
	for i, in := range inputs {
		want := golden[i]
		data := []byte(in)

		snap, err := DecodeBillingResponse(data, billingGoldenNow)
		if e, isErr := want["billErr"]; isErr {
			if err == nil {
				t.Errorf("case %d: decoded, Swift error %v\n%.200s", i, e, in)
			} else if billingSwiftErr(err) != e {
				t.Errorf("case %d: error %v, Swift %v", i, err, e)
			}
		} else if err != nil {
			t.Errorf("case %d: %v, Swift decoded\n%.200s", i, err, in)
		} else {
			raw, merr := snap.MarshalJSON()
			if merr != nil || string(raw) != want["bill"] {
				t.Errorf("case %d:\n got %s\nwant %v\ninput %.300s", i, raw, want["bill"], in)
			}
			back, perr := ParseBillingSnapshot(raw)
			if perr != nil {
				t.Errorf("case %d: parse own JSON: %v", i, perr)
			} else if again, _ := back.MarshalJSON(); string(again) != string(raw) {
				t.Errorf("case %d: round trip\n got %s\nwant %s", i, again, raw)
			}
		}

		limit, used, merr := DecodeMoneyAllowance(data)
		if e, isErr := want["moneyErr"]; isErr {
			if merr == nil || billingSwiftErr(merr) != e {
				t.Errorf("case %d: money err %v, Swift %v", i, merr, e)
			}
		} else if merr != nil {
			t.Errorf("case %d: money %v, Swift decoded", i, merr)
		} else if got := billingCentPair(limit, used); fmt.Sprint(got) != fmt.Sprint(want["money"]) {
			t.Errorf("case %d: money %v, Swift %v", i, got, want["money"])
		}

		plan, ok := DecodeSettingsPlan(data)
		var gotPlan any
		if ok {
			gotPlan = plan
		}
		if gotPlan != want["plan"] {
			t.Errorf("case %d: plan %v, Swift %v\n%.200s", i, gotPlan, want["plan"], in)
		}
	}
}

// TestBillingDecodeMatchesSwift replays 365 billing payloads (credits, cents,
// legacy, malformed, odd numbers and dates) through
// GrokQuotaSnapshot.decodeBillingResponse / decodeMoneyAllowance /
// decodeSettingsPlan, comparing the encoded snapshot byte for byte.
func TestBillingDecodeMatchesSwift(t *testing.T) {
	replayBillingGolden(t, "testdata/billing_decode_in.json", "testdata/billing_decode_out.json")
}

// TestBillingSettingsPlanMatchesSwift does the same for 71 /v1/settings bodies.
func TestBillingSettingsPlanMatchesSwift(t *testing.T) {
	replayBillingGolden(t, "testdata/billing_settings_in.json", "testdata/billing_settings_out.json")
}
