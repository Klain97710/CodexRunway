package resettoday

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func statusServer(t *testing.T, status int, body string) (*Client, *atomic.Pointer[http.Header]) {
	t.Helper()
	var seen atomic.Pointer[http.Header]
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Clone()
		seen.Store(&h)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return &Client{HTTP: srv.Client(), StatusURL: srv.URL}, &seen
}

func TestFetchStatusDecodesAndSendsAccept(t *testing.T) {
	now := at(t, "2026-07-28T12:00:00Z")
	feed := newFeed(now, newEvent("reset_completed", "2026-07-28T11:00:00Z"))
	c, seen := statusServer(t, 200, feed.JSON())
	s, err := c.FetchStatus(context.Background(), now, time.UTC)
	if err != nil || s.State() != StateYes {
		t.Fatalf("snapshot = %v, err = %v", s, err)
	}
	if got := (*seen.Load()).Get("Accept"); got != "application/json" {
		t.Errorf("Accept = %q", got)
	}
}

func TestFetchStatusRejectsBadResponses(t *testing.T) {
	now := at(t, "2026-07-28T12:00:00Z")
	c, _ := statusServer(t, 503, "down")
	if _, err := c.FetchStatus(context.Background(), now, time.UTC); err != ErrBadServerResponse {
		t.Errorf("503: %v", err)
	}
	c, _ = statusServer(t, 200, "<html>")
	if _, err := c.FetchStatus(context.Background(), now, time.UTC); err == nil {
		t.Error("garbage decoded")
	}
}

func TestFetchStatusHonoursContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	t.Cleanup(srv.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	c := Client{HTTP: srv.Client(), StatusURL: srv.URL}
	if _, err := c.FetchStatus(ctx, time.Now(), time.UTC); err == nil {
		t.Fatal("hung request returned no error")
	}
}

func TestFetchStatusDevMockSkipsNetwork(t *testing.T) {
	kind := MockCompleted
	now := at(t, "2026-07-28T12:00:00Z")
	s, err := (Client{DevMock: &kind, StatusURL: "http://127.0.0.1:1"}).FetchStatus(context.Background(), now, time.UTC)
	if err != nil || s.VerdictPresentation(now, time.UTC).Reason != "completed" {
		t.Fatalf("mock = %v, %v", s, err)
	}
}

func TestStartOfDayAcrossDST(t *testing.T) {
	la := mustLoc(t, "America/Los_Angeles")
	havana := mustLoc(t, "America/Havana") // spring-forward at midnight: 00:00 does not exist
	tests := []struct {
		name string
		loc  *time.Location
		in   string
		want string
	}{
		{"LA spring forward day", la, "2026-03-08T20:00:00Z", "2026-03-08T08:00:00Z"},
		{"LA after spring forward", la, "2026-03-09T20:00:00Z", "2026-03-09T07:00:00Z"},
		{"LA fall back day", la, "2026-11-01T20:00:00Z", "2026-11-01T07:00:00Z"},
		{"Havana midnight gap", havana, "2026-03-08T12:00:00Z", "2026-03-08T05:00:00Z"},
		{"Singapore", mustLoc(t, "Asia/Singapore"), "2026-08-10T10:00:00Z", "2026-08-09T16:00:00Z"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := startOfDay(at(t, tc.in), tc.loc)
			if !got.Equal(at(t, tc.want)) {
				t.Fatalf("startOfDay = %v, want %v", got.UTC(), tc.want)
			}
		})
	}
	if got := startOfNextDay(at(t, "2026-03-08T20:00:00Z"), la); !got.Equal(at(t, "2026-03-09T07:00:00Z")) {
		t.Errorf("startOfNextDay = %v", got.UTC())
	}
}
