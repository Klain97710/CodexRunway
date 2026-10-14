package codexapi

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func newRefresher(srv *recorder) TokenRefresher {
	return TokenRefresher{HTTP: srv.Client(), TokenURL: srv.URL + "/oauth/token"}
}

func TestRefreshSuccessSendsClientID(t *testing.T) {
	srv := newRecorder(t, func(recorded) (int, string) {
		return 200, `{"id_token":"new-id","access_token":"new-access","refresh_token":"new-refresh"}`
	})
	got, err := newRefresher(srv).Refresh(context.Background(), "old-refresh")
	if err != nil || got != (TokenSet{IDToken: "new-id", AccessToken: "new-access", RefreshToken: "new-refresh"}) {
		t.Fatalf("%+v %v", got, err)
	}
	r := srv.last(t)
	if r.Method != "POST" || r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" ||
		r.Form.Get("grant_type") != "refresh_token" || r.Form.Get("refresh_token") != "old-refresh" || r.Form.Get("client_id") != ClientID {
		t.Fatalf("%+v", r)
	}
}

func TestRefreshKeepsStoredTokensWhenOmitted(t *testing.T) {
	srv := newRecorder(t, func(recorded) (int, string) { return 200, `{"access_token":"new-access"}` })
	current := TokenSet{IDToken: "id", AccessToken: "old", RefreshToken: "rt"}
	response, err := newRefresher(srv).Refresh(context.Background(), "rt")
	if err != nil {
		t.Fatal(err)
	}
	if merged := response.MergedInto(current); merged != (TokenSet{IDToken: "id", AccessToken: "new-access", RefreshToken: "rt"}) {
		t.Fatalf("%+v", merged)
	}
}

func TestRefreshFallsBackWithoutClientID(t *testing.T) {
	srv := newRecorder(t, func(r recorded) (int, string) {
		if r.Form.Get("client_id") != "" {
			return 400, `{"error":"invalid_client"}`
		}
		return 200, `{"access_token":"a"}`
	})
	got, err := newRefresher(srv).Refresh(context.Background(), "rt")
	if err != nil || got.AccessToken != "a" {
		t.Fatalf("%+v %v", got, err)
	}
	reqs := srv.all()
	if len(reqs) != 2 || reqs[0].Form.Get("client_id") == "" || reqs[1].Form.Has("client_id") {
		t.Fatalf("requests %+v", reqs)
	}
}

func TestRefreshFallbackAcceptsObjectErrorAnd401(t *testing.T) {
	srv := newRecorder(t, func(r recorded) (int, string) {
		if r.Form.Get("client_id") != "" {
			return 401, `{"error":{"code":"invalid_client"}}`
		}
		return 200, `{"access_token":"a"}`
	})
	if _, err := newRefresher(srv).Refresh(context.Background(), "rt"); err != nil || len(srv.all()) != 2 {
		t.Fatalf("%v %d", err, len(srv.all()))
	}
}

func TestRefreshRevokedGrantNeedsReauth(t *testing.T) {
	for _, tc := range []struct {
		status int
		body   string
	}{{400, `{"error":"invalid_grant"}`}, {400, `{"error":"invalid_token"}`}, {401, `{}`}, {401, `{"error":"token_expired"}`}} {
		srv := newRecorder(t, func(recorded) (int, string) { return tc.status, tc.body })
		_, err := newRefresher(srv).Refresh(context.Background(), "rt")
		if !NeedsReauth(err) || len(srv.all()) != 1 {
			t.Errorf("%d %s: err=%v requests=%d", tc.status, tc.body, err, len(srv.all()))
		}
	}
}

func TestRefreshTransientStatusesAreNotReauth(t *testing.T) {
	for _, status := range []int{400, 403, 429, 500, 503} {
		srv := newRecorder(t, func(recorded) (int, string) { return status, `{"error":"server_error"}` })
		_, err := newRefresher(srv).Refresh(context.Background(), "rt")
		var h *HTTPError
		if !errors.As(err, &h) || h.StatusCode != status || NeedsReauth(err) || len(srv.all()) != 1 {
			t.Errorf("%d: err=%v requests=%d", status, err, len(srv.all()))
		}
	}
}

func TestRefreshRejectsEmptyTokenAndBrokenResponse(t *testing.T) {
	srv := newRecorder(t, func(recorded) (int, string) { return 200, `{"id_token":"x"}` })
	if _, err := newRefresher(srv).Refresh(context.Background(), ""); !NeedsReauth(err) || len(srv.all()) != 0 {
		t.Fatalf("empty token: %v", err)
	}
	if _, err := newRefresher(srv).Refresh(context.Background(), "rt"); !errors.Is(err, ErrDecode) {
		t.Fatalf("missing access token: %v", err)
	}
}

type fakePersister struct {
	err   error
	calls []persisted
}

type persisted struct {
	account string
	tokens  TokenSet
	at      time.Time
}

func (p *fakePersister) SaveRefreshed(_ context.Context, account string, tokens TokenSet, at time.Time) error {
	p.calls = append(p.calls, persisted{account, tokens, at})
	return p.err
}

func TestRefreshAndPersist(t *testing.T) {
	srv := newRecorder(t, func(recorded) (int, string) { return 200, `{"access_token":"new","refresh_token":"rotated"}` })
	p := &fakePersister{}
	current := TokenSet{IDToken: "id", AccessToken: "old", RefreshToken: "rt"}
	got, err := newRefresher(srv).RefreshAndPersist(context.Background(), p, "acct", current, fixedNow)
	want := TokenSet{IDToken: "id", AccessToken: "new", RefreshToken: "rotated"}
	if err != nil || got != want || len(p.calls) != 1 || p.calls[0] != (persisted{"acct", want, fixedNow}) {
		t.Fatalf("%+v %v %+v", got, err, p.calls)
	}
}

func TestRefreshAndPersistFailureKeepsRotatedTokens(t *testing.T) {
	srv := newRecorder(t, func(recorded) (int, string) { return 200, `{"access_token":"new","refresh_token":"rotated"}` })
	p := &fakePersister{err: errors.New("disk full")}
	got, err := newRefresher(srv).RefreshAndPersist(context.Background(), p, "acct", TokenSet{RefreshToken: "rt"}, fixedNow)
	if err == nil || !strings.Contains(err.Error(), "disk full") || got.RefreshToken != "rotated" || got.AccessToken != "new" {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestRefreshAndPersistErrorDoesNotPersist(t *testing.T) {
	srv := newRecorder(t, func(recorded) (int, string) { return 401, `{}` })
	p := &fakePersister{}
	current := TokenSet{AccessToken: "old", RefreshToken: "rt"}
	got, err := newRefresher(srv).RefreshAndPersist(context.Background(), p, "acct", current, fixedNow)
	if !NeedsReauth(err) || got != current || len(p.calls) != 0 {
		t.Fatalf("%+v %v", got, err)
	}
}
