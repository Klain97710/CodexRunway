package app

import (
	"context"
	"errors"
	"github.com/licoy/quosparr/internal/personal"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestPersonalUpdaterRedirectsAndCredentials(t *testing.T) {
	origin := personal.Homepage + "/releases/download/personal-v0.2.0/update-darwin-arm64.json"
	cdn := "https://release-assets.githubusercontent.com/github-production-release-asset/" + personal.RepositoryID + "/asset-uuid?sig=temporary"
	for _, target := range []string{cdn, "https://github.com/Licoy/Quosparr/releases/latest/download/a.zip", "http://github.com/" + personal.Repository + "/releases/latest/download/a.zip"} {
		calls := 0
		restore := configureUpdaterHTTP(func(context.Context) (*http.Client, error) {
			return &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				if req.Header.Get("Authorization") != "" || req.Header.Get("Cookie") != "" {
					t.Fatal("credentials reached update origin/CDN")
				}
				header := http.Header{}
				code := 200
				if req.URL.String() == origin {
					code = 302
					header.Set("Location", target)
				}
				return &http.Response{StatusCode: code, Header: header, Body: io.NopCloser(strings.NewReader("ok")), Request: req}, nil
			})}, nil
		})
		req, _ := http.NewRequest("GET", origin, nil)
		req.Header.Set("Authorization", "Bearer synthetic")
		req.Header.Set("Cookie", "synthetic=1")
		resp, err := http.DefaultClient.Do(req)
		if resp != nil {
			resp.Body.Close()
		}
		restore()
		if target == cdn {
			if err != nil || calls != 2 {
				t.Fatalf("CDN redirect blocked: %v calls=%d", err, calls)
			}
		} else if !errors.Is(err, ErrUpdateURL) || calls != 1 {
			t.Fatalf("unsafe redirect escaped guard: %v calls=%d", err, calls)
		}
	}
}
