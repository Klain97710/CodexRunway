package usage

import (
	"bufio"
	"errors"
	"math"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func analyticsWindow() Window {
	return Window{Start: mustTime("2026-06-24T00:00:00Z"), End: mustTime("2026-07-01T00:00:00Z")}
}

func decodeA(t *testing.T, payload string) (Summary, error) {
	t.Helper()
	return DecodeAnalytics([]byte(payload), analyticsWindow(), mustTime("2026-06-30T10:00:00Z"), "", "")
}

// TestAnalyticsNumericGolden replays /testdata/analytics_nums.txt, produced
// by the Swift decoder for each (field, raw JSON value) pair.
func TestAnalyticsNumericGolden(t *testing.T) {
	f, err := os.Open("testdata/analytics_nums.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	n := 0
	for sc.Scan() {
		line := sc.Text()
		field, rest, _ := strings.Cut(line, " ")
		var value, status string
		// The value may contain no space except inside quoted strings; the
		// status token is the first " OK"/" ERR" occurrence after it.
		if i := strings.LastIndex(rest, " ERR"); i >= 0 && strings.HasSuffix(rest, " ERR") {
			value, status = rest[:i], "ERR"
		} else {
			i := strings.Index(rest, " OK ")
			value, status = rest[:i], rest[i+1:]
		}
		payload := `{"data":[{"date":"2026-06-29","totals":{"` + field + `":` + value + `}}]}`
		s, err := DecodeAnalytics([]byte(payload), Window{Start: mustTime("2026-06-29T00:00:00Z"), End: mustTime("2026-06-30T00:00:00Z")},
			time.Unix(0, 0), "2026-06-29", "2026-06-29")
		n++
		if status == "ERR" {
			if err == nil {
				t.Errorf("%s %s: want error", field, value)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s %s: %v", field, value, err)
			continue
		}
		parts := strings.Fields(status)
		bits, _ := strconv.ParseUint(parts[1], 10, 64)
		turns, _ := strconv.ParseInt(parts[2], 10, 64)
		tokens, _ := strconv.ParseInt(parts[3], 10, 64)
		if math.Float64bits(s.RawCredits) != bits || s.Totals.Turns != turns || s.Totals.TotalTokens != tokens ||
			strconv.FormatBool(*s.DailyRows[0].CreditsReported) != parts[4] || string(s.Confidence) != parts[5] {
			t.Errorf("%s %s: got credits=%x turns=%d tokens=%d reported=%v conf=%s; want %s",
				field, value, math.Float64bits(s.RawCredits), s.Totals.Turns, s.Totals.TotalTokens,
				*s.DailyRows[0].CreditsReported, s.Confidence, status)
		}
	}
	if n < 150 {
		t.Fatalf("only %d golden rows", n)
	}
}

func TestAnalyticsCreditsZeroStillPricesTokenParts(t *testing.T) {
	s, err := decodeA(t, `{"data":[{"date":"2026-06-29","totals":{"credits":0,"turns":26,"threads":4,"cached_text_input_tokens":1000000,"uncached_text_input_tokens":2000000,"text_output_tokens":300000}}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if !s.CalculatedAt.Equal(mustTime("2026-06-30T10:00:00Z")) || s.Source != SourceOnlineAnalytics ||
		s.Confidence != ConfidencePriced || s.RawCredits != 0 || s.Totals.TotalTokens != 3_300_000 || s.Totals.Turns != 26 {
		t.Fatalf("unexpected summary %+v", s)
	}
	if s.EstimatedUSD == nil || s.EstimatedUSD.Sign() <= 0 {
		t.Fatalf("estimate = %v", s.EstimatedUSD)
	}
	if len(s.Warnings) != 0 {
		t.Fatalf("warnings = %v", s.Warnings)
	}
}

func TestAnalyticsRejectsInvalidNumericValues(t *testing.T) {
	for _, p := range []string{
		`{"data":[{"date":"2026-06-29","totals":{"turns":1.5}}]}`,
		`{"data":[{"date":"2026-06-29","totals":{"credits":"nan"}}]}`,
		`{"data":[{"date":"2026-06-29","models":[{"model":"gpt-5.5","turns":"invalid"}]}]}`,
	} {
		if _, err := decodeA(t, p); err == nil {
			t.Errorf("%s: want error", p)
		}
	}
	_, err := decodeA(t, `{"data":[
	  {"date":"2026-06-28","totals":{"credits":1.7976931348623157e308}},
	  {"date":"2026-06-29","totals":{"credits":1.7976931348623157e308}}]}`)
	if err == nil || errors.Is(err, ErrInvalidAnalytics) {
		t.Fatalf("overflowing credits: err = %v, want arithmetic error", err)
	}
}

func TestAnalyticsTotalOnlyIsTokensOnly(t *testing.T) {
	s, err := decodeA(t, `{"data":[{"date":"2026-06-29","totals":{"credits":0,"turns":1,"text_total_tokens":12000}}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if s.Confidence != ConfidenceTokensOnly || s.Totals.TotalTokens != 12000 || s.EstimatedUSD != nil {
		t.Fatalf("got %+v", s)
	}
	if len(s.Warnings) != 1 || s.Warnings[0] != WarningAnalyticsTokenPartsMissing {
		t.Fatalf("warnings = %v", s.Warnings)
	}
}

func TestAnalyticsDisplayableCost(t *testing.T) {
	un, _ := decodeA(t, `{"data":[{"date":"2026-06-29","totals":{"credits":0,"turns":0,"text_total_tokens":0}}]}`)
	to, _ := decodeA(t, `{"data":[{"date":"2026-06-29","totals":{"credits":0,"turns":1,"text_total_tokens":12000}}]}`)
	pr, _ := decodeA(t, `{"data":[{"date":"2026-06-29","totals":{"credits":0,"turns":1,"uncached_text_input_tokens":1000,"text_output_tokens":100}}]}`)
	if un.IsDisplayableCost() || !to.IsDisplayableCost() || !pr.IsDisplayableCost() {
		t.Fatal("displayable mismatch")
	}
	if un.Confidence != ConfidenceUnavailable || pr.Confidence != ConfidencePriced {
		t.Fatalf("confidence %s %s", un.Confidence, pr.Confidence)
	}
}

func TestAnalyticsDaysFilterSortAndMissingTotals(t *testing.T) {
	s, err := decodeA(t, `{"data":[
	 {"date":"2026-06-30","totals":{"turns":2,"text_total_tokens":10}},
	 {"date":"2026-05-01","totals":{"turns":9}},
	 {"date":"2026-06-26"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.DailyRows) != 2 || s.DailyRows[0].Date != "2026-06-26" || s.DailyRows[1].Date != "2026-06-30" {
		t.Fatalf("rows = %+v", s.DailyRows)
	}
	if *s.DailyRows[0].TotalsReported || !*s.DailyRows[1].TotalsReported || *s.DailyRows[0].CreditsReported {
		t.Fatal("reported flags")
	}
	if s.Totals.Turns != 2 || s.Confidence != ConfidenceTokensOnly {
		t.Fatalf("got %+v", s)
	}
}

func TestAnalyticsBreakdownNamesAndOrder(t *testing.T) {
	s, err := decodeA(t, `{"data":[
	 {"date":"2026-06-29","models":[
	   {"model":" gpt-5.5 ","text_total_tokens":10,"turns":1},
	   {"model":"gpt-5.5","text_total_tokens":5},
	   {"model":"  ","name":"named","text_total_tokens":100},
	   {"id":"i","text_total_tokens":0,"turns":0},
	   {"text_total_tokens":3}],
	  "clients":[{"client_id":"cli","uncached_text_input_tokens":1000000,"credits":2}]}]}`)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, r := range s.ModelRows {
		names = append(names, r.Name)
	}
	if strings.Join(names, ",") != "named,gpt-5.5,UNKNOWN" {
		t.Fatalf("names = %v", names)
	}
	if s.ModelRows[1].Totals.TotalTokens != 15 {
		t.Fatalf("gpt-5.5 = %+v", s.ModelRows[1])
	}
	if len(s.ClientRows) != 1 || s.ClientRows[0].RawCredits != 2 || s.ClientRows[0].EstimatedUSD == nil ||
		s.ClientRows[0].EstimatedUSD.Sign() <= 0 {
		t.Fatalf("clients = %+v", s.ClientRows)
	}
}

func TestAnalyticsMalformedDocuments(t *testing.T) {
	for _, p := range []string{``, `[]`, `{}`, `{"data":{}}`, `{"data":[{"totals":{}}]}`, `{"data":[{"date":5}]}`, `{"data":[]} x`} {
		if _, err := decodeA(t, p); err == nil {
			t.Errorf("%q: want error", p)
		}
	}
	s, err := decodeA(t, `{"data":[]}`)
	if err != nil || s.Confidence != ConfidenceUnavailable || len(s.DailyRows) != 0 {
		t.Fatalf("empty: %+v %v", s, err)
	}
}

func TestAnalyticsEmptyTotalsObjectCountsAsReported(t *testing.T) {
	s, err := decodeA(t, `{"data":[{"date":"2026-06-29","totals":{}},{"date":"2026-06-28","totals":null}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if *s.DailyRows[0].TotalsReported || !*s.DailyRows[1].TotalsReported {
		t.Fatalf("reported = %v %v", *s.DailyRows[0].TotalsReported, *s.DailyRows[1].TotalsReported)
	}
}
