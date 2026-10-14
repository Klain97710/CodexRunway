package app

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/licoy/quosparr/internal/netx"
	"github.com/licoy/quosparr/internal/store"
)

// proxyAt starts a plain HTTP proxy that answers every request itself and
// counts them, and returns the configuration that uses it.
func proxyAt(t *testing.T) (store.NetworkProxyConfiguration, *atomic.Int32) {
	t.Helper()
	var seen atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.Add(1)
		_, _ = io.WriteString(w, "via proxy "+r.URL.Host)
	}))
	t.Cleanup(srv.Close)
	host, port, _ := net.SplitHostPort(srv.Listener.Addr().String())
	n, _ := strconv.Atoi(port)
	return store.NetworkProxyConfiguration{Mode: store.ProxyModeHTTP, Host: host, Port: n}, &seen
}

func getVia(t *testing.T, ctx context.Context, url string) (string, error) {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b), nil
}

func TestConfigureUpdaterHTTPSendsRequestsThroughTheNetworkContext(t *testing.T) {
	before := http.DefaultClient
	restore := configureUpdaterHTTP(netxUpdateClient)
	t.Cleanup(restore)
	if http.DefaultClient == before {
		t.Fatal("http.DefaultClient was not replaced")
	}
	if before.Transport != nil {
		t.Error("the previous client was modified in place")
	}

	cfg, seen := proxyAt(t)
	c, err := netx.NewContext(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	// The request context carries the cycle's network context, like the
	// update cycle's own context does after a settings change.
	ctx, cancel := context.WithCancel(netx.WithContext(context.Background(), c))
	defer cancel()
	// "check" and "install" of one cycle: same ctx, same proxy.
	for range 2 {
		body, err := getVia(t, ctx, "http://updates.example.invalid/feed.json")
		if err != nil || !strings.HasPrefix(body, "via proxy updates.example.invalid") {
			t.Fatalf("body %q, err %v", body, err)
		}
	}
	if seen.Load() != 2 {
		t.Errorf("proxy saw %d requests, want 2", seen.Load())
	}

	restore()
	if http.DefaultClient != before {
		t.Error("restore did not put the previous client back")
	}
}

func TestUpdaterTransportUsesNetxForAContextThatIsNoCycle(t *testing.T) {
	var asked atomic.Int32
	rt := updaterTransport{clients: newCycleClients(func(ctx context.Context) (*http.Client, error) {
		asked.Add(1)
		return &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("ok")), Header: http.Header{}}, nil
		})}, nil
	})}
	for range 2 {
		req, _ := http.NewRequest(http.MethodGet, "http://x.invalid/", nil) // Background: never ends
		resp, err := rt.RoundTrip(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	}
	if asked.Load() != 2 {
		t.Errorf("provider asked %d times, want once per request", asked.Load())
	}
}

func TestUpdaterTransportFailsInsteadOfConnectingDirectly(t *testing.T) {
	var reached atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached.Add(1) }))
	defer srv.Close()
	boom := errors.New("proxy settings are broken")
	rt := updaterTransport{clients: newCycleClients(func(context.Context) (*http.Client, error) { return nil, boom })}
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	if _, err := rt.RoundTrip(req); !errors.Is(err, boom) {
		t.Errorf("err = %v, want the provider's error", err)
	}
	// A client without a transport must not mean the default one.
	rt = updaterTransport{clients: newCycleClients(func(context.Context) (*http.Client, error) { return &http.Client{}, nil })}
	if _, err := rt.RoundTrip(req); !errors.Is(err, ErrUpdateNoTransport) {
		t.Errorf("err = %v, want ErrUpdateNoTransport", err)
	}
	if reached.Load() != 0 {
		t.Error("a request went out directly")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// TestNoBusinessCodeUsesTheGlobalHTTPClient: the global client belongs to
// mygo's updater (see updaterTransport). Every other request in the app takes
// an explicit client from internal/netx.
func TestNoBusinessCodeUsesTheGlobalHTTPClient(t *testing.T) {
	_, here, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(here), "..") // internal/
	banned := map[string]bool{
		"DefaultClient": true, "DefaultTransport": true,
		"Get": true, "Post": true, "Head": true, "PostForm": true,
	}
	// Known exceptions, to be removed: the fallback of a nil client in the
	// pricing fetcher (the app always passes a netx client).
	allowed := map[string]bool{}

	var found []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		if rel == filepath.Join("app", "update_http.go") || allowed[rel] {
			return nil
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		name := ""
		for _, imp := range f.Imports {
			if imp.Path.Value == `"net/http"` {
				name = "http"
				if imp.Name != nil {
					name = imp.Name.Name
				}
			}
		}
		if name == "" {
			return nil
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok {
				if id, ok := sel.X.(*ast.Ident); ok && id.Name == name && banned[sel.Sel.Name] {
					found = append(found, rel+": http."+sel.Sel.Name)
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(found) > 0 {
		t.Errorf("business code uses the global HTTP client (use internal/netx):\n%s", strings.Join(found, "\n"))
	}
}
