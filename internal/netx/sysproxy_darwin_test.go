//go:build darwin

package netx

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
)

const testPAC = `function FindProxyForURL(url, host) {
  if (host == "direct.example") return "DIRECT";
  if (host == "socks.example") return "SOCKS 127.0.0.1:1080";
  return "PROXY 127.0.0.1:18080; DIRECT";
}`

func servePAC(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ns-proxy-autoconfig")
		_, _ = w.Write([]byte(testPAC))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestPACEvaluationThroughCFNetwork(t *testing.T) {
	pac := servePAC(t)
	cases := []struct{ target, want string }{
		{"https://api.example.com/v1", "http://127.0.0.1:18080"},
		{"https://direct.example/v1", ""},
		{"http://socks.example/v1", "socks5://127.0.0.1:1080"},
	}
	for _, tc := range cases {
		got, err := evaluatePACURL(pac.URL+"/proxy.pac", mustURL(t, tc.target))
		if err != nil {
			t.Fatalf("%s: %v", tc.target, err)
		}
		if urlString(got) != tc.want {
			t.Fatalf("%s: proxy %q, want %q", tc.target, urlString(got), tc.want)
		}
	}
}

func TestPACEvaluationConcurrent(t *testing.T) {
	pac := servePAC(t)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := evaluatePACURL(pac.URL+"/proxy.pac", &url.URL{Scheme: "https", Host: "api.example.com"})
			if err != nil || urlString(got) != "http://127.0.0.1:18080" {
				t.Errorf("proxy %q err %v", urlString(got), err)
			}
		}()
	}
	wg.Wait()
}

func TestPACFailureIsAnErrorNotDirect(t *testing.T) {
	bad := httptest.NewServer(http.NotFoundHandler())
	defer bad.Close()
	got, err := evaluatePACURL(bad.URL+"/missing.pac", mustURL(t, "https://api.example.com"))
	if err == nil {
		t.Fatalf("a broken PAC returned %q instead of an error", urlString(got))
	}
}

func TestCFNetworkSystemSettingsAreReadable(t *testing.T) {
	// The result depends on the machine; the call itself must work and a
	// failure must be an error, never a panic.
	proxy, err := osSystemProxy(mustURL(t, "https://example.com"))
	t.Logf("system proxy for https://example.com: %q err=%v", urlString(proxy), err)
}
