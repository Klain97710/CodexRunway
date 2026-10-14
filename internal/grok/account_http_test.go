package grok

import (
	"net/http"
	"net/http/httptest"
)

// handlerTransport serves every request from an in-process handler, so tests
// can use the real https://auth.x.ai URLs without any network access.
type handlerTransport struct{ handler http.Handler }

func (h handlerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := req.Context().Err(); err != nil {
		return nil, err
	}
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, req)
	resp := rec.Result()
	resp.Request = req
	return resp, nil
}

func handlerClient(h http.Handler) *http.Client {
	return &http.Client{Transport: handlerTransport{h}}
}

// timeoutTransport fails every request with a timeout error.
type timeoutTransport struct{}

type timeoutError struct{}

func (timeoutError) Error() string   { return "i/o timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

func (timeoutTransport) RoundTrip(*http.Request) (*http.Response, error) { return nil, timeoutError{} }
