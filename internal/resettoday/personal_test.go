package resettoday

import (
	"context"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"
)

func TestPublicStatusNeverUsesCookiesOrReactions(t *testing.T) {
	now := at(t, "2026-10-14T12:00:00Z")
	feed := newFeed(now, newEvent("reset_completed", "2026-10-14T11:00:00Z"))
	var status, forbidden atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/status.json" || r.Header.Get("Cookie") != "" {
			forbidden.Add(1)
		}
		status.Add(1)
		w.Header().Set("Set-Cookie", "visitor=must-not-store; Path=/")
		_, _ = w.Write([]byte(feed.JSON()))
	}))
	defer srv.Close()
	jar, _ := cookiejar.New(nil)
	u, _ := url.Parse(srv.URL)
	jar.SetCookies(u, []*http.Cookie{{Name: "old", Value: "must-not-send", Path: "/"}})
	httpClient := srv.Client()
	httpClient.Jar = jar
	c := Client{HTTP: httpClient, StatusURL: srv.URL + "/api/status.json"}
	for i := 0; i < 3; i++ {
		if _, err := c.FetchStatus(context.Background(), now, time.UTC); err != nil {
			t.Fatal(err)
		}
	}
	if status.Load() != 3 || forbidden.Load() != 0 {
		t.Fatal("public status boundary violated")
	}
	if cookies := jar.Cookies(u); len(cookies) != 1 || cookies[0].Name != "old" {
		t.Fatal("response visitor cookie was stored")
	}
}
