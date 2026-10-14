package grok

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"testing"
	"time"
)

var resetNow = time.Unix(1_786_601_000, 0)

func resetSwiftError(err error) string {
	var rpc *ResetCreditsRPCError
	switch {
	case errors.Is(err, ErrResetCreditsInvalidFrame):
		return "invalidFrame"
	case errors.Is(err, ErrResetCreditsCompressed):
		return "compressedUnsupported"
	case errors.Is(err, ErrResetCreditsUnknownStructure):
		return "unknownStructure"
	case errors.As(err, &rpc):
		return fmt.Sprintf("rpcFailed(%q)", rpc.Message)
	}
	return err.Error()
}

// resetGoldenTime reports whether got is the golden epoch (a "%.9f" string or
// nil) within the precision of a Double at epoch scale.
func resetGoldenTime(want any, got *time.Time) bool {
	if want == nil {
		return got == nil
	}
	if got == nil {
		return false
	}
	var w float64
	fmt.Sscanf(want.(string), "%f", &w)
	if math.Abs(w) >= resetEpochLimit { // clamped
		return math.Abs(float64(got.Unix())) >= resetEpochLimit-1 && (w > 0) == (got.Unix() > 0)
	}
	return math.Abs(w-(float64(got.Unix())+float64(got.Nanosecond())/1e9)) < 1e-6
}

// TestResetCreditsMatchesSwift replays 187 hex-encoded responses (proto,
// gRPC-Web frames, JSON) through GrokResetCreditsSnapshot.decode.
func TestResetCreditsMatchesSwift(t *testing.T) {
	var inputs []string
	var golden []map[string]any
	readJSON(t, "testdata/billing_resetcredits_in.json", &inputs)
	readJSON(t, "testdata/billing_resetcredits_out.json", &golden)
	if len(inputs) != len(golden) {
		t.Fatalf("%d inputs, %d goldens", len(inputs), len(golden))
	}
	for i, h := range inputs {
		data, _ := hex.DecodeString(h)
		want := golden[i]
		snap, err := DecodeResetCredits(data, resetNow)
		if e, isErr := want["err"]; isErr {
			if err == nil {
				t.Errorf("case %d (%s): decoded, Swift error %v", i, h, e)
			} else if got := resetSwiftError(err); got != e {
				t.Errorf("case %d (%s): error %s, Swift %v", i, h, got, e)
			}
			continue
		}
		if err != nil {
			t.Errorf("case %d (%s): %v, Swift decoded", i, h, err)
			continue
		}
		if int(want["n"].(float64)) != snap.AvailableCount {
			t.Errorf("case %d (%s): count %d, Swift %v", i, h, snap.AvailableCount, want["n"])
		}
		wt := want["t"].([]any)
		if len(wt) != len(snap.Tokens) {
			t.Errorf("case %d (%s): %d tokens, Swift %d", i, h, len(snap.Tokens), len(wt))
			continue
		}
		mapped := snap.CodexSnapshot(time.Time{}).Credits
		wc := want["c"].([]any)
		for k, tok := range snap.Tokens {
			row := wt[k].([]any)
			if row[0] != tok.TokenID || row[3] != tok.IsAvailable(resetNow) ||
				!resetGoldenTime(row[1], tok.ValidityStart) || !resetGoldenTime(row[2], tok.ValidityEnd) {
				t.Errorf("case %d (%s) token %d: %+v, Swift %v", i, h, k, tok, row)
			}
			crow := wc[k].([]any)
			if crow[1] != mapped[k].Status {
				t.Errorf("case %d token %d: status %s, Swift %v", i, k, mapped[k].Status, crow[1])
			}
			var wantLeft float64
			fmt.Sscanf(crow[4].(string), "%f", &wantLeft)
			if math.Abs(wantLeft) < 1e12 && math.Abs(mapped[k].RemainingSeconds-wantLeft) > 1e-5 {
				t.Errorf("case %d token %d: remaining %f, Swift %f", i, k, mapped[k].RemainingSeconds, wantLeft)
			}
		}
		if !snap.UpdatedAt.Equal(resetNow) {
			t.Errorf("case %d: updatedAt %v", i, snap.UpdatedAt)
		}
	}
}

func resetVarint(v uint64) []byte {
	var out []byte
	for v >= 0x80 {
		out = append(out, byte(v)|0x80)
		v >>= 7
	}
	return append(out, byte(v))
}

func resetLD(field int, data []byte) []byte {
	out := resetVarint(uint64(field<<3 | 2))
	out = append(out, resetVarint(uint64(len(data)))...)
	return append(out, data...)
}

func resetFixtureProto(id string, start, end time.Time) []byte {
	stamp := func(t time.Time) []byte { return append([]byte{0x08}, resetVarint(uint64(t.Unix()))...) }
	var tok []byte
	tok = append(tok, resetLD(10, []byte(id))...)
	tok = append(tok, resetLD(20, stamp(start))...)
	tok = append(tok, resetLD(30, stamp(end))...)
	return resetLD(10, tok)
}

func resetFixtureWeb(message []byte, status int, text string) []byte {
	out := resetFrame(message, 0)
	trailer := fmt.Sprintf("grpc-status:%d\r\n", status)
	if text != "" {
		trailer += "grpc-message:" + text + "\r\n"
	}
	return append(out, resetFrame([]byte(trailer), 0x80)...)
}

func TestResetCreditsOfficialCases(t *testing.T) {
	start, end := time.Unix(1_786_560_540, 0), time.Unix(1_789_238_940, 0)

	snap, err := DecodeResetCredits(resetFixtureProto("restok_fixture", start, end), resetNow)
	if err != nil || snap.AvailableCount != 1 || len(snap.Tokens) != 1 ||
		snap.Tokens[0].TokenID != "restok_fixture" || !snap.Tokens[0].ValidityStart.Equal(start) ||
		!snap.Tokens[0].ValidityEnd.Equal(end) || !snap.UpdatedAt.Equal(resetNow) {
		t.Fatalf("proto card: %+v %v", snap, err)
	}

	framed := resetFixtureWeb(resetFixtureProto("restok_framed", start, end), 0, "")
	if snap, err = DecodeResetCredits(framed, resetNow); err != nil || snap.AvailableCount != 1 || snap.Tokens[0].TokenID != "restok_framed" {
		t.Fatalf("framed: %+v %v", snap, err)
	}

	for name, data := range map[string][]byte{"empty": nil, "framed empty": resetFixtureWeb(nil, 0, "")} {
		if snap, err = DecodeResetCredits(data, resetNow); err != nil || snap.AvailableCount != 0 || len(snap.Tokens) != 0 {
			t.Fatalf("%s: %+v %v", name, snap, err)
		}
	}

	if snap, err = DecodeResetCredits([]byte(`{"tokens":[]}`), resetNow); err != nil || snap.AvailableCount != 0 {
		t.Fatalf("json empty: %+v %v", snap, err)
	}

	js := `{"tokens":[{"tokenId":"restok_json","validityStart":"2026-08-12T18:49:00Z","validityEnd":"2026-09-12T18:49:00Z"}]}`
	if snap, err = DecodeResetCredits([]byte(js), resetNow); err != nil || snap.AvailableCount != 1 ||
		snap.Tokens[0].TokenID != "restok_json" || !snap.Tokens[0].ValidityEnd.Equal(end) {
		t.Fatalf("json card: %+v %v", snap, err)
	}

	expired := resetFixtureProto("restok_expired", resetNow.Add(-60*time.Second), resetNow.Add(-time.Second))
	if snap, err = DecodeResetCredits(expired, resetNow); err != nil || snap.AvailableCount != 0 ||
		len(snap.Tokens) != 1 || snap.Tokens[0].IsAvailable(resetNow) {
		t.Fatalf("expired: %+v %v", snap, err)
	}

	mapped := ResetCreditSnapshot{AvailableCount: 1, UpdatedAt: resetNow,
		Tokens: []ResetCreditToken{{TokenID: "restok_map", ValidityStart: &start, ValidityEnd: &end}}}.CodexSnapshot(time.Time{})
	if mapped.AvailableCount != 1 || mapped.Credits[0].ID != "restok_map" || mapped.Credits[0].Status != "available" ||
		!mapped.Credits[0].ExpiresAt.Equal(end) || mapped.Credits[0].RemainingSeconds != end.Sub(resetNow).Seconds() {
		t.Fatalf("mapped: %+v", mapped)
	}

	_, err = DecodeResetCredits(resetFixtureWeb(nil, 13, "internal"), resetNow)
	var rpc *ResetCreditsRPCError
	if !errors.As(err, &rpc) || rpc.Message != "internal" {
		t.Fatalf("rpc failure: %v", err)
	}
}

func TestResetCreditsRequestFrame(t *testing.T) {
	if got := ResetCreditsGRPCWebRequest(nil); hex.EncodeToString(got) != "0000000000" {
		t.Fatalf("empty request %x", got)
	}
	if got := ResetCreditsGRPCWebRequest([]byte{1, 2}); hex.EncodeToString(got) != "00000000020102" {
		t.Fatalf("request %x", got)
	}
}
