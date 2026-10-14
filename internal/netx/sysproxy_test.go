package netx

import (
	"errors"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

func clearProxyEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{"HTTP_PROXY", "http_proxy", "HTTPS_PROXY", "https_proxy", "ALL_PROXY", "all_proxy", "NO_PROXY", "no_proxy"} {
		t.Setenv(name, "")
	}
}

func mustURL(t *testing.T, s string) *url.URL {
	t.Helper()
	u, err := url.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestEnvironmentProxyHasPriorityOverSystem(t *testing.T) {
	clearProxyEnv(t)
	t.Setenv("HTTPS_PROXY", "proxy.env:3128")
	t.Setenv("ALL_PROXY", "socks5://socks.env:1080")
	var calls atomic.Int32
	resolve := systemProxyWithEnvironment(func(*url.URL) (*url.URL, error) { calls.Add(1); return nil, nil })

	got, err := resolve(mustURL(t, "https://example.com/x"))
	if err != nil || got.String() != "http://proxy.env:3128" {
		t.Fatalf("https: %v %v", got, err)
	}
	got, err = resolve(mustURL(t, "http://example.com/x"))
	if err != nil || got.String() != "socks5://socks.env:1080" {
		t.Fatalf("http falls back to ALL_PROXY: %v %v", got, err)
	}
	if got, _ := resolve(mustURL(t, "http://localhost:8080/")); got != nil {
		t.Fatalf("loopback must stay direct: %v", got)
	}
	if calls.Load() != 0 {
		t.Fatal("the system was consulted despite an environment proxy")
	}
}

func TestEnvironmentErrorsDoNotLeakAndDoNotFallBack(t *testing.T) {
	clearProxyEnv(t)
	t.Setenv("HTTPS_PROXY", "ftp://user:secret-pass@proxy.env:21")
	var calls atomic.Int32
	resolve := systemProxyWithEnvironment(func(*url.URL) (*url.URL, error) { calls.Add(1); return nil, nil })
	_, err := resolve(mustURL(t, "https://example.com"))
	if err == nil || calls.Load() != 0 {
		t.Fatalf("err=%v calls=%d", err, calls.Load())
	}
	if strings.Contains(err.Error(), "secret-pass") {
		t.Fatalf("password leaked: %v", err)
	}
}

func TestNOProxyWithoutEnvironmentProxySkipsTheSystem(t *testing.T) {
	tests := []struct {
		name, noProxy, target string
		bypass                bool
	}{
		{"wildcard", "*", "http://updates.example/file", true},
		{"CIDR", "10.0.0.0/8", "https://10.2.3.4/file", true},
		{"domain suffix", "example.com", "https://updates.example.com/file", true},
		{"exact domain", "example.com", "https://example.com/file", true},
		{"matching port", "example.com:8080", "http://example.com:8080/file", true},
		{"different port", "example.com:8080", "http://example.com:8081/file", false},
		{"IPv6", "[2001:db8::1]", "https://[2001:db8::1]/file", true},
		{"unrelated", "other.example", "https://example.com/file", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clearProxyEnv(t)
			t.Setenv("NO_PROXY", tc.noProxy)
			var calls atomic.Int32
			system := mustURL(t, "http://system-proxy.example:8080")
			resolve := systemProxyWithEnvironment(func(*url.URL) (*url.URL, error) { calls.Add(1); return system, nil })
			got, err := resolve(mustURL(t, tc.target))
			if err != nil {
				t.Fatal(err)
			}
			if tc.bypass && (got != nil || calls.Load() != 0) {
				t.Fatalf("got %v, system calls %d", got, calls.Load())
			}
			if !tc.bypass && (got == nil || calls.Load() != 1) {
				t.Fatalf("got %v, system calls %d", got, calls.Load())
			}
		})
	}
}

func TestSystemErrorsPropagate(t *testing.T) {
	clearProxyEnv(t)
	boom := errors.New("gsettings failed")
	resolve := systemProxyWithEnvironment(func(*url.URL) (*url.URL, error) { return nil, boom })
	if _, err := resolve(mustURL(t, "https://example.com")); !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
}
