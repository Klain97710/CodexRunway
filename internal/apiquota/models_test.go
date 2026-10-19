package apiquota

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"
)

const modelsBody = `{"object":"list","data":[{"id":"gpt-5","owned_by":"openai"},{"id":"claude-sonnet"},{"id":" gpt-5 "},{"id":"a-model"},{"id":""},{"owned_by":"x"},{"id":7},"junk"]}`

func TestFetchModelsOpenAIStyle(t *testing.T) {
	site := newFakeSite(t, map[string]reply{"/v1/models": {200, modelsBody}})
	got, err := FetchModels(context.Background(), site.client(), site.url(), " sk-test ", ModelStyleOpenAI)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"a-model", "claude-sonnet", "gpt-5"}; !slices.Equal(got, want) {
		t.Fatalf("got %v", got)
	}
	reqs := site.all()
	if len(reqs) != 1 || reqs[0].auth != "Bearer sk-test" || reqs[0].apiKey != "" || reqs[0].path != "/v1/models" {
		t.Fatalf("%+v", reqs)
	}
}

func TestFetchModelsUsesNormalizedModelsURL(t *testing.T) {
	site := newFakeSite(t, map[string]reply{"/proxy/v1/models": {200, `{"data":[{"id":"m"}]}`}})
	for _, base := range []string{site.url() + "/proxy", site.url() + "/proxy/v1/", site.url() + "/proxy/v1/messages?x=1"} {
		got, err := FetchModels(context.Background(), site.client(), base, "sk-test", ModelStyleOpenAI)
		if err != nil || !slices.Equal(got, []string{"m"}) {
			t.Fatalf("%s: %v %v", base, got, err)
		}
	}
}

func TestFetchModelsAnthropicStyleHeaders(t *testing.T) {
	site := newFakeSite(t, map[string]reply{"/v1/models": {200, `{"data":[{"id":"claude-x","display_name":"Claude X","type":"model"}],"has_more":false}`}})
	got, err := FetchModels(context.Background(), site.client(), site.url()+"/v1/messages", "sk-ant", ModelStyleAnthropic)
	if err != nil || !slices.Equal(got, []string{"claude-x"}) {
		t.Fatalf("%v %v", got, err)
	}
	reqs := site.all()
	if len(reqs) != 1 {
		t.Fatalf("%+v", reqs)
	}
	if r := reqs[0]; r.apiKey != "sk-ant" || r.version != "2023-06-01" || r.auth != "" {
		t.Fatalf("%+v", r)
	}
}

func TestFetchModelsAnthropicFallsBackToBearer(t *testing.T) {
	var calls []seen
	client := doerFunc(func(r *http.Request) (*http.Response, error) {
		calls = append(calls, seen{auth: r.Header.Get("Authorization"), apiKey: r.Header.Get("x-api-key"), version: r.Header.Get("anthropic-version")})
		status, body := 401, ""
		if r.Header.Get("Authorization") == "Bearer sk-ant" {
			status, body = 200, `{"data":[{"id":"claude-x"}]}`
		}
		return textResponse(status, body), nil
	})
	got, err := FetchModels(context.Background(), client, "https://relay.example.com", "sk-ant", ModelStyleAnthropic)
	if err != nil || !slices.Equal(got, []string{"claude-x"}) {
		t.Fatalf("%v %v", got, err)
	}
	if len(calls) != 2 || calls[0].apiKey != "sk-ant" || calls[0].auth != "" || calls[1].auth != "Bearer sk-ant" || calls[1].apiKey != "" || calls[1].version != "2023-06-01" {
		t.Fatalf("%+v", calls)
	}

	// Both rejected: one retry only, then unauthorized.
	calls = nil
	reject := doerFunc(func(r *http.Request) (*http.Response, error) {
		calls = append(calls, seen{})
		return textResponse(403, ""), nil
	})
	if _, err := FetchModels(context.Background(), reject, "https://relay.example.com", "sk-ant", ModelStyleAnthropic); !errors.Is(err, ErrUnauthorized) {
		t.Fatal(err)
	}
	if len(calls) != 2 {
		t.Fatalf("%d calls", len(calls))
	}
}

func textResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: readCloser{strings.NewReader(body)}, Header: http.Header{}}
}

type readCloser struct{ *strings.Reader }

func (readCloser) Close() error { return nil }

func TestFetchModelsStatusErrors(t *testing.T) {
	tests := []struct {
		status int
		body   string
		check  func(error) bool
	}{
		{401, "", func(err error) bool { return errors.Is(err, ErrUnauthorized) }},
		{403, "", func(err error) bool { return errors.Is(err, ErrUnauthorized) }},
		{404, "", func(err error) bool { return errors.Is(err, ErrModelsNotFound) }},
		{500, "boom", func(err error) bool {
			var se *HTTPStatusError
			return errors.As(err, &se) && se.Code == 500 && err.Error() == "HTTP 500"
		}},
		{200, "<!doctype html>", func(err error) bool { return errors.Is(err, ErrInvalidResponse) }},
		{200, `[]`, func(err error) bool { return errors.Is(err, ErrInvalidResponse) }},
		{200, `{"data":"x"}`, func(err error) bool { return errors.Is(err, ErrInvalidResponse) }},
		{200, `{"data":[]} trailing`, func(err error) bool { return errors.Is(err, ErrInvalidResponse) }},
	}
	for _, tt := range tests {
		site := newFakeSite(t, map[string]reply{"/v1/models": {tt.status, tt.body}})
		got, err := FetchModels(context.Background(), site.client(), site.url(), "sk-test", ModelStyleOpenAI)
		if err == nil || got != nil || !tt.check(err) {
			t.Fatalf("%d %q: %v %v", tt.status, tt.body, got, err)
		}
	}
}

func TestFetchModelsEmptyLists(t *testing.T) {
	for _, body := range []string{`{}`, `{"data":null}`, `{"data":[]}`} {
		site := newFakeSite(t, map[string]reply{"/v1/models": {200, body}})
		got, err := FetchModels(context.Background(), site.client(), site.url(), "sk-test", ModelStyleOpenAI)
		if err != nil || got == nil || len(got) != 0 {
			t.Fatalf("%s: %#v %v", body, got, err)
		}
	}
}

func TestFetchModelsInputErrors(t *testing.T) {
	site := newFakeSite(t, nil)
	if _, err := FetchModels(context.Background(), site.client(), "ftp://x", "k", ModelStyleOpenAI); !errors.Is(err, ErrInvalidBaseURL) {
		t.Fatal(err)
	}
	if _, err := FetchModels(context.Background(), site.client(), "", "k", ModelStyleOpenAI); !errors.Is(err, ErrEmptyBaseURL) {
		t.Fatal(err)
	}
	if _, err := FetchModels(context.Background(), nil, site.url(), "k", ModelStyleOpenAI); err == nil {
		t.Fatal("nil client accepted")
	}
	if n := len(site.all()); n != 0 {
		t.Fatalf("%d requests", n)
	}
}

func TestFetchModelsEmptyKeyHasNoAuthHeader(t *testing.T) {
	site := newFakeSite(t, map[string]reply{"/v1/models": {200, `{"data":[{"id":"m"}]}`}})
	if _, err := FetchModels(context.Background(), site.client(), site.url(), "  ", ModelStyleOpenAI); err != nil {
		t.Fatal(err)
	}
	if r := site.all()[0]; r.auth != "" || r.apiKey != "" {
		t.Fatalf("%+v", r)
	}
}

func TestFetchModelsTransportErrorsAreSanitized(t *testing.T) {
	const key = "sk-live-0123456789abcdef"
	failing := doerFunc(func(r *http.Request) (*http.Response, error) {
		return nil, &url.Error{Op: "Get", URL: r.URL.String(), Err: fmt.Errorf("handshake failed for %s", key)}
	})
	_, err := FetchModels(context.Background(), failing, "https://relay.example.com", key, ModelStyleOpenAI)
	if err == nil || strings.Contains(err.Error(), key) || !strings.Contains(err.Error(), "<redacted>") {
		t.Fatalf("%v", err)
	}

	blocked := doerFunc(func(r *http.Request) (*http.Response, error) {
		<-r.Context().Done()
		return nil, &url.Error{Op: "Get", URL: r.URL.String(), Err: r.Context().Err()}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if _, err := FetchModels(ctx, blocked, "https://relay.example.com", key, ModelStyleOpenAI); !errors.Is(err, ErrTimeout) {
		t.Fatal(err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	cancel()
	if _, err := FetchModels(ctx, blocked, "https://relay.example.com", key, ModelStyleOpenAI); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestFetchModelsNeverEchoesKeyOrBody(t *testing.T) {
	const key = "sk-Secret0123456789abcdefXYZ"
	for _, status := range []int{401, 404, 500, 200} {
		site := newFakeSite(t, map[string]reply{"/v1/models": {status, `not json, key ` + key}})
		_, err := FetchModels(context.Background(), site.client(), site.url(), key, ModelStyleAnthropic)
		if err == nil || strings.Contains(err.Error(), key) || strings.Contains(err.Error(), "not json") {
			t.Fatalf("%d: %v", status, err)
		}
	}
}
