package store

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

const testUUID = "3F2504E0-4F89-41D3-9A0C-0305E82C3301"

func mustProxy(t *testing.T, doc string) NetworkProxyConfiguration {
	t.Helper()
	c, err := DecodeNetworkProxy([]byte(doc))
	if err != nil {
		t.Fatalf("DecodeNetworkProxy(%s): %v", doc, err)
	}
	return c
}

func TestProxySwiftFixtures(t *testing.T) {
	def := mustProxy(t, string(fixture(t, "swift_proxy_default.json")))
	if !reflect.DeepEqual(def, DefaultNetworkProxy()) {
		t.Errorf("default fixture decoded to %+v", def)
	}
	enc, err := EncodeNetworkProxy(DefaultNetworkProxy())
	if err != nil {
		t.Fatal(err)
	}
	sameJSON(t, enc, fixture(t, "swift_proxy_default.json"))

	auth := mustProxy(t, string(fixture(t, "swift_proxy_socks5_auth.json")))
	want := NetworkProxyConfiguration{Mode: ProxyModeSOCKS5, Host: "127.0.0.1", Port: 1080, UsesAuthentication: true, CredentialID: testUUID}
	if !reflect.DeepEqual(auth, want) {
		t.Errorf("auth fixture decoded to %+v", auth)
	}
	enc, err = EncodeNetworkProxy(want)
	if err != nil {
		t.Fatal(err)
	}
	sameJSON(t, enc, fixture(t, "swift_proxy_socks5_auth.json"))
}

func TestProxyDecodeRules(t *testing.T) {
	// credentialID is optional and null counts as absent; unknown keys are ignored by Swift.
	c := mustProxy(t, `{"mode":"http","host":"h","port":1,"usesAuthentication":false,"credentialID":null,"extra":1}`)
	if c.CredentialID != "" || c.Mode != ProxyModeHTTP {
		t.Errorf("got %+v", c)
	}
	for _, doc := range []string{
		``, `null`, `[]`, `{`,
		`{"mode":"http","host":"h","port":1}`,
		`{"mode":"http","host":"h","usesAuthentication":false}`,
		`{"mode":"http","port":1,"usesAuthentication":false}`,
		`{"host":"h","port":1,"usesAuthentication":false}`,
		`{"mode":"http","host":"h","port":1,"usesAuthentication":null}`,
		`{"mode":"pac","host":"h","port":1,"usesAuthentication":false}`,
		`{"mode":"http","host":"h","port":1.5,"usesAuthentication":false}`,
		`{"mode":"http","host":"h","port":"1","usesAuthentication":false}`,
		`{"mode":"http","host":1,"port":1,"usesAuthentication":false}`,
		`{"mode":"http","host":"h","port":1,"usesAuthentication":0}`,
		`{"mode":"http","host":"h","port":1,"usesAuthentication":false,"credentialID":5}`,
	} {
		if _, err := DecodeNetworkProxy([]byte(doc)); !errors.Is(err, ErrProxyInvalidConfiguration) {
			t.Errorf("%q: err = %v, want ErrProxyInvalidConfiguration", doc, err)
		}
	}
}

func TestProxyValidated(t *testing.T) {
	ok := func(c NetworkProxyConfiguration) NetworkProxyConfiguration {
		t.Helper()
		v, err := c.Validated()
		if err != nil {
			t.Fatalf("Validated(%+v): %v", c, err)
		}
		return v
	}
	http := func(host string, port int) NetworkProxyConfiguration {
		return NetworkProxyConfiguration{Mode: ProxyModeHTTP, Host: host, Port: port}
	}

	// System mode drops everything else.
	got := ok(NetworkProxyConfiguration{Mode: ProxyModeSystem, Host: "x", Port: 9, UsesAuthentication: true, CredentialID: testUUID})
	if !reflect.DeepEqual(got, DefaultNetworkProxy()) {
		t.Errorf("system mode kept fields: %+v", got)
	}
	for host, want := range map[string]string{
		"proxy.example.com":    "proxy.example.com",
		"  Proxy.Example.COM ": "proxy.example.com",
		"localhost":            "localhost",
		"example.com.":         "example.com.",
		"a-b.c1":               "a-b.c1",
		"127.0.0.1":            "127.0.0.1",
		"10.0.0.255":           "10.0.0.255",
		"::1":                  "::1",
		"[::1]":                "::1",
		"[2001:DB8::A]":        "2001:db8::a",
		"::ffff:1.2.3.4":       "::ffff:1.2.3.4",
	} {
		if got := ok(http(host, 8080)).Host; got != want {
			t.Errorf("host %q -> %q, want %q", host, got, want)
		}
	}
	for _, host := range []string{
		"", "   ", "[]", "[example.com]", "a b", "a/b", `a\b`, "user@host", "host?x", "host#x", "host%25", "a[b", "fe80::1%en0",
		"1.2.3", "256.1.1.1", "1.2.3.4.5", "1..2.3", ":::", "[::1", "-a.com", "a-.com", "a..com", "..",
		"under_score.com", "ünï.com", "bad\x00host", "tab\thost", strings.Repeat("a", 64) + ".com", strings.Repeat("a.", 127) + "a",
	} {
		if _, err := http(host, 8080).Validated(); !errors.Is(err, ErrProxyInvalidHost) {
			t.Errorf("host %q: err = %v, want ErrProxyInvalidHost", host, err)
		}
	}
	for _, port := range []int{-1, 0, 65_536, 100_000} {
		if _, err := http("h", port).Validated(); !errors.Is(err, ErrProxyInvalidPort) {
			t.Errorf("port %d: err = %v, want ErrProxyInvalidPort", port, err)
		}
	}
	for _, port := range []int{1, 80, 65_535} {
		ok(http("h", port))
	}

	// Credential reference: cleared without authentication, a UUID with it.
	c := http("h", 1)
	c.CredentialID = testUUID
	if got := ok(c); got.CredentialID != "" {
		t.Errorf("credential kept without authentication: %+v", got)
	}
	c.UsesAuthentication = true
	if got := ok(c); got.CredentialID != testUUID {
		t.Errorf("credential lost: %+v", got)
	}
	c.CredentialID = strings.ToLower(testUUID)
	ok(c)
	for _, id := range []string{"not-a-uuid", testUUID + "0", testUUID[:35], strings.Replace(testUUID, "-", "_", 1), "ZF2504E0-4F89-41D3-9A0C-0305E82C3301"} {
		c.CredentialID = id
		if _, err := c.Validated(); !errors.Is(err, ErrProxyInvalidConfiguration) {
			t.Errorf("credential %q: err = %v", id, err)
		}
	}
	if _, err := (NetworkProxyConfiguration{Mode: "pac", Port: 1, Host: "h"}).Validated(); !errors.Is(err, ErrProxyInvalidConfiguration) {
		t.Errorf("unknown mode: err = %v", err)
	}
}

func TestProxyEncode(t *testing.T) {
	c := NetworkProxyConfiguration{Mode: ProxyModeHTTP, Host: "Proxy.Example.com", Port: 3128}
	enc, err := EncodeNetworkProxy(c)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(enc); got != `{"host":"proxy.example.com","mode":"http","port":3128,"usesAuthentication":false}` {
		t.Errorf("encoded = %s (credentialID must be omitted when empty)", got)
	}
	c.UsesAuthentication = true
	if _, err := EncodeNetworkProxy(c); !errors.Is(err, ErrProxyCredentialsUnavailable) {
		t.Errorf("authentication without a credential reference: err = %v", err)
	}
	c.CredentialID = testUUID
	if enc, err = EncodeNetworkProxy(c); err != nil || !strings.Contains(string(enc), `"credentialID":"`+testUUID+`"`) {
		t.Errorf("encoded = %s, %v", enc, err)
	}
	if _, err := EncodeNetworkProxy(NetworkProxyConfiguration{Mode: ProxyModeHTTP, Host: "h", Port: 0}); !errors.Is(err, ErrProxyInvalidPort) {
		t.Errorf("err = %v", err)
	}
}

func TestProxyUnknownKeysRoundTrip(t *testing.T) {
	c := mustProxy(t, `{"mode":"http","host":"h","port":1,"usesAuthentication":false,"bypass":["a.com",{"x":1}]}`)
	c.Port = 2
	enc, err := EncodeNetworkProxy(c)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(enc), `"bypass":["a.com",{"x":1}]`) || !strings.Contains(string(enc), `"port":2`) {
		t.Errorf("encoded = %s", enc)
	}
}

func TestProxyHoldsNoSecretFields(t *testing.T) {
	typ := reflect.TypeOf(NetworkProxyConfiguration{})
	for i := 0; i < typ.NumField(); i++ {
		name := strings.ToLower(typ.Field(i).Name)
		if strings.Contains(name, "password") || strings.Contains(name, "username") || strings.Contains(name, "secret") {
			t.Errorf("NetworkProxyConfiguration must not hold %s", typ.Field(i).Name)
		}
	}
}
