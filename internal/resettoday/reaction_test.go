package resettoday

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"

	"github.com/licoy/quosparr/internal/export"
	"github.com/licoy/quosparr/internal/platform"
)

func okBody(count int) string {
	return fmt.Sprintf(`{"ok":true,"data":{"enabled":true,"ready":true,"polarity":"no","epochId":"abc","seed":10,"count":%d,"remaining":null,"dailyLimit":0,"pollMs":5000}}`, count)
}

func reactionSnap(epoch string, count int, remaining *int, limit int, pol ReactionPolarity) ReactionSnapshot {
	seed := count
	return NewReactionSnapshot(ReactionSnapshot{
		Enabled: true, Ready: true, Polarity: &pol, EpochID: &epoch, Seed: &seed,
		Count: &count, Remaining: remaining, DailyLimit: limit, PollMs: 2000,
	})
}

func TestParseVisitorIDViaStore(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"0ca75a14e041f05a5258f7924fa08914", "0ca75a14e041f05a5258f7924fa08914"},
		{"0CA75A14E041F05A5258F7924FA08914", "0ca75a14e041f05a5258f7924fa08914"},
		{"  abcdef0123456789abcdef0123456789  ", "abcdef0123456789abcdef0123456789"},
		{"", ""}, {"not-a-visitor-id", ""},
		{"0ca75a14e041f05a5258f7924fa0891", ""}, {"0ca75a14e041f05a5258f7924fa0891400", ""},
		{"0ca75a14e041f05a5258f7924fa0891g", ""},
	}
	for _, tc := range tests {
		got, ok := export.ParseVisitorID(tc.in)
		if got != tc.want || ok != (tc.want != "") {
			t.Errorf("ParseVisitorID(%q) = %q, %v", tc.in, got, ok)
		}
	}
}

func TestDecodesLiveEnvelope(t *testing.T) {
	r, err := DecodeReactionEnvelope([]byte(`{"ok":true,"data":{"enabled":true,"ready":true,"polarity":"no",
      "epochId":"f34540ec3645796726174653156e7fe7","seed":202,"count":266,"remaining":null,"dailyLimit":0,"pollMs":5000}}`))
	if err != nil {
		t.Fatal(err)
	}
	s := r.Data
	if !r.OK || r.Error != nil || s == nil || !s.Enabled || !s.Ready || *s.Polarity != PolarityNo ||
		*s.EpochID != "f34540ec3645796726174653156e7fe7" || *s.Seed != 202 || *s.Count != 266 ||
		s.Remaining != nil || s.DailyLimit != 0 || s.PollMs != 5000 || !s.IsVisible() || s.IsExhausted() {
		t.Fatalf("envelope = %+v data=%+v", r, s)
	}
}

func TestPollMsIsClamped(t *testing.T) {
	for in, want := range map[int]int{0: 1000, 999: 1000, 1500: 1500, 10_000: 10_000, 50_000: 10_000} {
		if got := ClampedPollMs(in); got != want {
			t.Errorf("ClampedPollMs(%d) = %d, want %d", in, got, want)
		}
	}
}

func TestUnlimitedNeverExhausts(t *testing.T) {
	zero, one := 0, 1
	tests := []struct {
		remaining *int
		limit     int
		want      bool
	}{
		{&zero, 0, false}, {nil, 0, false}, {&zero, 5, true}, {&one, 5, false}, {nil, 5, false},
	}
	for _, tc := range tests {
		if got := IsExhausted(tc.remaining, tc.limit); got != tc.want {
			t.Errorf("IsExhausted(%v, %d) = %v", tc.remaining, tc.limit, got)
		}
	}
}

func TestDailyLimitRollback(t *testing.T) {
	epoch, one, zero := "abc", 1, 0
	local := ReactionLocal{EpochID: &epoch, PreviousCount: 10, PreviousRemaining: &one}
	daily := "daily_limit"
	rolled := ReconcileAfterPost(local, &ReactionPostResult{OK: false, Error: &daily})
	if rolled.Count != 10 || !rolled.Exhausted || rolled.EpochID == nil || *rolled.EpochID != "abc" {
		t.Errorf("rolled = %+v", rolled)
	}
	snap := reactionSnap("abc", 11, &zero, 2, PolarityNo)
	ok := ReconcileAfterPost(local, &ReactionPostResult{OK: true, Data: &snap})
	if ok.Count != 11 || !ok.Exhausted {
		t.Errorf("ok = %+v", ok)
	}
}

func TestEpochChangeUsesNewPayload(t *testing.T) {
	old, three := "old", 3
	local := ReactionLocal{EpochID: &old, PreviousCount: 140, PreviousRemaining: &three}
	snap := reactionSnap("new", 100, &three, 5, PolarityYes)
	next := ReconcileAfterPost(local, &ReactionPostResult{OK: true, Data: &snap})
	if next.Count != 100 || *next.EpochID != "new" || next.Exhausted {
		t.Errorf("next = %+v", next)
	}
}

func TestPositiveDelta(t *testing.T) {
	i := func(v int) *int { return &v }
	s := func(v string) *string { return &v }
	tests := []struct {
		name         string
		prev, next   *int
		pEpoch, nEpo *string
		want         int
	}{
		{"one", i(10), i(11), s("a"), s("a"), 1},
		{"five", i(10), i(15), s("a"), s("a"), 5},
		{"same", i(10), i(10), s("a"), s("a"), 0},
		{"decrease", i(10), i(9), s("a"), s("a"), 0},
		{"epoch change", i(10), i(50), s("a"), s("b"), 0},
		{"no previous count", nil, i(11), s("a"), s("a"), 0},
		{"no previous epoch", i(10), i(11), nil, s("a"), 0},
	}
	for _, tc := range tests {
		if got := PositiveDelta(tc.prev, tc.next, tc.pEpoch, tc.nEpo); got != tc.want {
			t.Errorf("%s: %d, want %d", tc.name, got, tc.want)
		}
	}
	for in, want := range map[int]string{1: "+1", 12: "+12", 0: "", 1234: "+1,234"} {
		if got := FormatDelta(in); got != want {
			t.Errorf("FormatDelta(%d) = %q", in, got)
		}
	}
}

func TestOptimisticClick(t *testing.T) {
	two := 2
	bumped := OptimisticClick(reactionSnap("abc", 10, &two, 5, PolarityNo))
	if *bumped.Count != 11 || bumped.Remaining == nil || *bumped.Remaining != 1 {
		t.Errorf("limited = %+v", bumped)
	}
	next := OptimisticClick(reactionSnap("abc", 10, nil, 0, PolarityNo))
	if *next.Count != 11 || next.Remaining != nil {
		t.Errorf("unlimited = %+v", next)
	}
}

// Client --------------------------------------------------------------------

type recorder struct {
	mu       sync.Mutex
	requests []*http.Request
}

func (r *recorder) record(req *http.Request) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.requests = append(r.requests, req.Clone(req.Context()))
}

func newReactionClient(t *testing.T, h http.HandlerFunc) (*ReactionClient, *recorder, export.ReactionStore) {
	t.Helper()
	rec := &recorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.record(r)
		h(w, r)
	}))
	t.Cleanup(srv.Close)
	store := export.ReactionStore{Path: filepath.Join(t.TempDir(), "reaction-visitor.json")}
	return &ReactionClient{HTTP: srv.Client(), URL: srv.URL, Store: store}, rec, store
}

func TestFetchStoresMintedCookieAndReusesIt(t *testing.T) {
	visitor := "ac2a42c46b05a49956fc6f7ea0da262e"
	client, rec, store := newReactionClient(t, func(w http.ResponseWriter, r *http.Request) {
		minted := r.Header.Get("Cookie") == ""
		if minted {
			w.Header().Set("Set-Cookie", "hr_react="+visitor+"; HttpOnly; Path=/; SameSite=lax; Secure")
		}
		count := 11
		if minted {
			count = 10
		}
		fmt.Fprint(w, okBody(count))
	})
	first, err := client.Fetch(context.Background())
	if err != nil || *first.Count != 10 {
		t.Fatalf("first = %+v, %v", first, err)
	}
	if id, ok := store.LoadVisitorID(); !ok || id != visitor {
		t.Fatalf("stored = %q, %v", id, ok)
	}
	second, err := client.Fetch(context.Background())
	if err != nil || *second.Count != 11 {
		t.Fatalf("second = %+v, %v", second, err)
	}
	if len(rec.requests) != 2 {
		t.Fatalf("requests = %d", len(rec.requests))
	}
	if rec.requests[0].Header.Get("Cookie") != "" {
		t.Error("first request invented a cookie")
	}
	if got := rec.requests[1].Header.Get("Cookie"); got != "hr_react="+visitor {
		t.Errorf("second cookie = %q", got)
	}
	for i, r := range rec.requests {
		if r.Method != http.MethodGet || r.Header.Get("Origin") != "" || r.Header.Get("Referer") != "" ||
			r.Header.Get("Accept") != "application/json" {
			t.Errorf("request %d headers = %v", i, r.Header)
		}
	}
}

func TestClickSendsOriginRefererAndCookie(t *testing.T) {
	client, rec, store := newReactionClient(t, func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, okBody(12)) })
	if err := store.SaveVisitorID("abcdef0123456789abcdef0123456789"); err != nil {
		t.Fatal(err)
	}
	r, err := client.Click(context.Background())
	if err != nil || !r.OK || *r.Data.Count != 12 {
		t.Fatalf("click = %+v, %v", r, err)
	}
	req := rec.requests[len(rec.requests)-1]
	if req.Method != http.MethodPost || req.Header.Get("Origin") != ReactionOrigin ||
		req.Header.Get("Referer") != ReactionReferer || req.Header.Get("Cookie") != "hr_react=abcdef0123456789abcdef0123456789" {
		t.Errorf("headers = %v", req.Header)
	}
	if err := platform.VerifyOwnerOnly(store.Path, 0o600); err != nil {
		t.Errorf("store permissions: %v", err)
	}
}

func TestFetchStatusMapping(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{"404 is disabled", 404, `{"ok":false,"error":"disabled"}`, ErrReactionDisabled},
		{"500 is unavailable", 500, `oops`, ErrReactionUnavailable},
		{"disabled envelope", 200, `{"ok":false,"error":"disabled"}`, ErrReactionDisabled},
		{"not ok", 200, `{"ok":false,"error":"nope"}`, ErrReactionInvalidPayload},
		{"garbage", 200, `<html>`, ErrReactionInvalidPayload},
		{"ok without data", 200, `{"ok":true}`, ErrReactionInvalidPayload},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client, _, _ := newReactionClient(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			})
			if _, err := client.Fetch(context.Background()); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestClickDailyLimitDoesNotThrow(t *testing.T) {
	client, _, _ := newReactionClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(429)
		fmt.Fprint(w, `{"ok":false,"error":"daily_limit","data":{"enabled":true,"ready":true,"polarity":"no","epochId":"abc","seed":10,"count":12,"remaining":0,"dailyLimit":2,"pollMs":2000}}`)
	})
	r, err := client.Click(context.Background())
	if err != nil || r.OK || r.Error == nil || *r.Error != "daily_limit" || r.Data == nil || *r.Data.Count != 12 {
		t.Fatalf("result = %+v, %v", r, err)
	}
}

func TestClickStatusDerivedErrors(t *testing.T) {
	for status, want := range map[int]string{403: "forbidden", 404: "disabled", 429: "rate_limited", 503: "unavailable"} {
		client, _, _ := newReactionClient(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
			fmt.Fprint(w, `{"ok":false}`)
		})
		r, err := client.Click(context.Background())
		if err != nil || r.OK || r.Error == nil || *r.Error != want {
			t.Errorf("%d: %+v, %v", status, r, err)
		}
	}
	client, _, _ := newReactionClient(t, func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, "not json") })
	if r, err := client.Click(context.Background()); err != nil || r.OK {
		t.Errorf("undecodable: %+v, %v", r, err)
	}
}

func TestReactionDevMockCounts(t *testing.T) {
	kind := MockNo
	client := &ReactionClient{DevMock: &kind}
	a, err := client.Fetch(context.Background())
	if err != nil || *a.Count != 266 {
		t.Fatalf("fetch = %+v, %v", a, err)
	}
	c, err := client.Click(context.Background())
	if err != nil || !c.OK || *c.Data.Count != 267 {
		t.Fatalf("click = %+v, %v", c, err)
	}
}
