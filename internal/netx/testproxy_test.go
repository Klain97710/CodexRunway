package netx

import (
	"bufio"
	"encoding/base64"
	"encoding/binary"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// httpProxy is a minimal forward proxy for tests: absolute-URI requests and
// CONNECT tunnels, with optional Basic proxy authentication.
type httpProxy struct {
	srv         *httptest.Server
	user, pass  string
	connects    atomic.Int32
	forwarded   atomic.Int32
	authSeen    atomic.Int32
	rejected    atomic.Int32
	mu          sync.Mutex
	lastRequest string // request-target of the last forwarded request
}

func newHTTPProxy(t *testing.T, user, pass string) *httpProxy {
	t.Helper()
	p := &httpProxy{user: user, pass: pass}
	p.srv = httptest.NewServer(http.HandlerFunc(p.handle))
	t.Cleanup(p.srv.Close)
	return p
}

func (p *httpProxy) addr() (string, int) {
	host, port, _ := net.SplitHostPort(p.srv.Listener.Addr().String())
	n, _ := strconv.Atoi(port)
	return host, n
}

func (p *httpProxy) authorized(r *http.Request) bool {
	if p.user == "" {
		return true
	}
	h := r.Header.Get("Proxy-Authorization")
	if h == "" {
		return false
	}
	p.authSeen.Add(1)
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte(p.user+":"+p.pass))
	return h == want
}

func (p *httpProxy) handle(w http.ResponseWriter, r *http.Request) {
	if !p.authorized(r) {
		p.rejected.Add(1)
		w.Header().Set("Proxy-Authenticate", `Basic realm="test"`)
		w.WriteHeader(http.StatusProxyAuthRequired)
		return
	}
	if r.Method == http.MethodConnect {
		p.connects.Add(1)
		p.tunnel(w, r)
		return
	}
	p.forwarded.Add(1)
	p.mu.Lock()
	p.lastRequest = r.RequestURI
	p.mu.Unlock()
	out := r.Clone(r.Context())
	out.RequestURI = ""
	out.Header.Del("Proxy-Authorization")
	resp, err := http.DefaultTransport.RoundTrip(out)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	for k, v := range resp.Header {
		w.Header()[k] = v
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func (p *httpProxy) tunnel(w http.ResponseWriter, r *http.Request) {
	upstream, err := net.Dial("tcp", r.Host)
	if err != nil {
		http.Error(w, "bad gateway", http.StatusBadGateway)
		return
	}
	hj := w.(http.Hijacker)
	client, _, err := hj.Hijack()
	if err != nil {
		upstream.Close()
		return
	}
	_, _ = client.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
	pipe(client, upstream)
}

func pipe(a, b net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)
	cp := func(dst, src net.Conn) {
		defer wg.Done()
		_, _ = io.Copy(dst, src)
		if c, ok := dst.(*net.TCPConn); ok {
			_ = c.CloseWrite()
		}
	}
	go cp(a, b)
	go cp(b, a)
	wg.Wait()
	a.Close()
	b.Close()
}

// socksProxy is a minimal SOCKS5 server with optional RFC 1929 authentication.
type socksProxy struct {
	ln          net.Listener
	user, pass  string
	connects    atomic.Int32
	authFailed  atomic.Int32
	lastAddress atomic.Value // string
}

func newSOCKSProxy(t *testing.T, user, pass string) *socksProxy {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &socksProxy{ln: ln, user: user, pass: pass}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go s.serve(conn)
		}
	}()
	return s
}

func (s *socksProxy) addr() (string, int) {
	host, port, _ := net.SplitHostPort(s.ln.Addr().String())
	n, _ := strconv.Atoi(port)
	return host, n
}

func (s *socksProxy) serve(conn net.Conn) {
	defer conn.Close()
	r := bufio.NewReader(conn)
	head := make([]byte, 2)
	if _, err := io.ReadFull(r, head); err != nil || head[0] != 5 {
		return
	}
	methods := make([]byte, head[1])
	if _, err := io.ReadFull(r, methods); err != nil {
		return
	}
	want := byte(0)
	if s.user != "" {
		want = 2
	}
	found := false
	for _, m := range methods {
		found = found || m == want
	}
	if !found {
		_, _ = conn.Write([]byte{5, 0xff})
		s.authFailed.Add(1)
		return
	}
	_, _ = conn.Write([]byte{5, want})
	if want == 2 {
		ver, _ := r.ReadByte()
		ulen, _ := r.ReadByte()
		user := make([]byte, ulen)
		_, _ = io.ReadFull(r, user)
		plen, _ := r.ReadByte()
		pass := make([]byte, plen)
		_, _ = io.ReadFull(r, pass)
		if ver != 1 || string(user) != s.user || string(pass) != s.pass {
			_, _ = conn.Write([]byte{1, 1})
			s.authFailed.Add(1)
			return
		}
		_, _ = conn.Write([]byte{1, 0})
	}
	req := make([]byte, 4)
	if _, err := io.ReadFull(r, req); err != nil || req[1] != 1 {
		return
	}
	var host string
	switch req[3] {
	case 1:
		b := make([]byte, 4)
		_, _ = io.ReadFull(r, b)
		host = net.IP(b).String()
	case 3:
		n, _ := r.ReadByte()
		b := make([]byte, n)
		_, _ = io.ReadFull(r, b)
		host = string(b)
	case 4:
		b := make([]byte, 16)
		_, _ = io.ReadFull(r, b)
		host = net.IP(b).String()
	default:
		return
	}
	pb := make([]byte, 2)
	_, _ = io.ReadFull(r, pb)
	target := net.JoinHostPort(host, strconv.Itoa(int(binary.BigEndian.Uint16(pb))))
	s.lastAddress.Store(target)
	upstream, err := net.Dial("tcp", target)
	if err != nil {
		_, _ = conn.Write([]byte{5, 5, 0, 1, 0, 0, 0, 0, 0, 0})
		return
	}
	s.connects.Add(1)
	_, _ = conn.Write([]byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0})
	// Anything the client already buffered behind the request goes upstream.
	if n := r.Buffered(); n > 0 {
		b, _ := r.Peek(n)
		_, _ = upstream.Write(b)
	}
	pipe(conn, upstream)
}

func okHandler(body string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, body)
	})
}

func hostPort(t *testing.T, serverURL string) string {
	t.Helper()
	return strings.TrimPrefix(strings.TrimPrefix(serverURL, "https://"), "http://")
}

func hostPortOf(addr string) (string, int) {
	h, p, _ := net.SplitHostPort(addr)
	n, _ := strconv.Atoi(p)
	return h, n
}

func net_JoinHostPort(host string, port int) string {
	return net.JoinHostPort(host, strconv.Itoa(port))
}
