package codexapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

func newQuotaClient(srv *recorder) QuotaClient {
	return QuotaClient{HTTP: srv.Client(), BaseURL: srv.URL + "/", Now: func() time.Time { return fixedNow }}
}

func TestFetchQuotaRequestShapeAndDecode(t *testing.T) {
	srv := newRecorder(t, func(recorded) (int, string) { return 200, usageFixture })
	auth := Auth{AccessToken: "tok-secret", AccountID: "acct-selected", IDToken: jwt(t, map[string]any{
		"https://api.openai.com/auth": map[string]any{"chatgpt_account_id": "claim-account"},
	})}
	q, err := newQuotaClient(srv).FetchQuota(context.Background(), auth)
	if err != nil || q.Plan != "pro" {
		t.Fatalf("%+v %v", q, err)
	}
	r := srv.last(t)
	if r.Method != "GET" || r.Path != "/wham/usage" ||
		r.Header.Get("Authorization") != "Bearer tok-secret" ||
		r.Header.Get("ChatGPT-Account-Id") != "acct-selected" ||
		r.Header.Get("User-Agent") != "Quosparr/1" ||
		r.Header.Get("Accept") != "application/json" {
		t.Fatalf("request %+v", r)
	}
}

func TestFetchStatusMapping(t *testing.T) {
	for status, check := range map[int]func(error) bool{
		401: func(err error) bool { return errors.Is(err, ErrAuthenticationRequired) && NeedsReauth(err) },
		403: func(err error) bool {
			var h *HTTPError
			return errors.As(err, &h) && h.StatusCode == 403 && errors.Is(err, ErrBadServerResponse)
		},
		429: func(err error) bool { var h *HTTPError; return errors.As(err, &h) && h.StatusCode == 429 },
		500: func(err error) bool { var h *HTTPError; return errors.As(err, &h) && h.StatusCode == 500 },
	} {
		srv := newRecorder(t, func(recorded) (int, string) { return status, `{"error":"secret-body-text"}` })
		_, err := newQuotaClient(srv).FetchQuota(context.Background(), Auth{AccessToken: "t"})
		if err == nil || !check(err) || strings.Contains(err.Error(), "secret-body-text") {
			t.Errorf("status %d: err = %v", status, err)
		}
	}
}

func TestFetchQuotaHonoursContextCancellation(t *testing.T) {
	release := make(chan struct{})
	srv := newRecorder(t, func(recorded) (int, string) { <-release; return 200, usageFixture })
	defer close(release)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, err := newQuotaClient(srv).FetchQuota(ctx, Auth{AccessToken: "t"}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v", err)
	}
}

func TestFetchResetCreditsAndProfileAndDaily(t *testing.T) {
	srv := newRecorder(t, func(r recorded) (int, string) {
		switch r.Path {
		case "/wham/rate-limit-reset-credits":
			return 200, `{"available_count":2,"credits":[]}`
		case "/wham/profiles/me":
			return 200, `{"stats":{"daily_usage_buckets":[{"start_date":"2026-06-01","tokens":5}]}}`
		default:
			return 200, `{"data":[]}`
		}
	})
	c := newQuotaClient(srv)
	ctx := context.Background()
	credits, err := c.FetchResetCredits(ctx, Auth{AccessToken: "t"})
	if err != nil || credits.AvailableCount != 2 {
		t.Fatalf("%+v %v", credits, err)
	}
	profile, err := c.FetchTokenUsageProfile(ctx, Auth{AccessToken: "t"})
	if err != nil || profile.DailyTokens["2026-06-01"] != 5 {
		t.Fatalf("%+v %v", profile, err)
	}
	raw, err := c.FetchDailyWorkspaceUsageRaw(ctx, Auth{AccessToken: "t"}, "2026-06-01", "2026-06-30")
	if err != nil || string(raw) != `{"data":[]}` {
		t.Fatalf("%s %v", raw, err)
	}
	last := srv.last(t)
	if last.Path != "/wham/analytics/daily-workspace-usage-counts" ||
		last.Query.Get("start_date") != "2026-06-01" || last.Query.Get("end_date") != "2026-06-30" || last.Query.Get("group_by") != "day" {
		t.Fatalf("daily request %+v", last)
	}
}

func TestDecodeWorkspaceName(t *testing.T) {
	body := []byte(`{"items":[{"id":"a","name":"Other"},{"id":"acct-1","name":"  Acme Team  "},{"id":"blank","name":"   "},{"id":"null","name":null}]}`)
	for id, want := range map[string]string{"acct-1": "Acme Team", "blank": "", "null": "", "missing": "", "acct": ""} {
		got, err := DecodeWorkspaceName(body, id)
		if err != nil || got != want {
			t.Errorf("%s: %q %v", id, got, err)
		}
	}
	if _, err := DecodeWorkspaceName([]byte(`nope`), "x"); !errors.Is(err, ErrDecode) {
		t.Fatalf("err = %v", err)
	}
}

func TestFetchWorkspaceNameRequest(t *testing.T) {
	srv := newRecorder(t, func(recorded) (int, string) {
		return 200, `{"items":[{"id":"acct-1","name":" Acme "}]}`
	})
	c := newQuotaClient(srv)
	claim := jwt(t, map[string]any{"https://api.openai.com/auth": map[string]any{"chatgpt_account_id": "claim-id"}})
	name, err := c.FetchWorkspaceName(context.Background(), Auth{AccessToken: "tok", AccountID: "acct-1", IDToken: claim})
	if err != nil || name != "Acme" {
		t.Fatalf("%q %v", name, err)
	}
	r := srv.last(t)
	if r.Path != "/accounts" || r.Header.Get("Authorization") != "Bearer tok" || r.Header.Get("ChatGPT-Account-Id") != "acct-1" {
		t.Fatalf("%+v", r)
	}
	before := len(srv.all())
	if name, err := c.FetchWorkspaceName(context.Background(), Auth{AccessToken: "tok"}); name != "" || err != nil {
		t.Fatalf("no account id: %q %v", name, err)
	}
	if name, err := c.FetchWorkspaceName(context.Background(), Auth{AccountID: "acct-1"}); name != "" || err != nil {
		t.Fatalf("no token: %q %v", name, err)
	}
	if len(srv.all()) != before {
		t.Fatal("request sent without credentials")
	}
}

func TestFetchWorkspaceNameForbidden(t *testing.T) {
	srv := newRecorder(t, func(recorded) (int, string) { return 403, `{}` })
	_, err := newQuotaClient(srv).FetchWorkspaceName(context.Background(), Auth{AccessToken: "t", AccountID: "a"})
	var h *HTTPError
	if !errors.As(err, &h) || h.StatusCode != http.StatusForbidden {
		t.Fatalf("err = %v", err)
	}
}
