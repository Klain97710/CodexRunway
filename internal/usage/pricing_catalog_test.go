package usage

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/platform"
)

const catalogMarkdown = `# Pricing

### Standard pricing data

| Model | Short context input | Short context cached input | Short context cache writes | Short context output | Long context input | Long context cached input | Long context cache writes | Long context output |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| gpt-6-astra | $10.00 | $1.00 | $12.50 | $50.00 | $20.00 | $2.00 | $25.00 | $75.00 |
| gpt-6-sol | $2.00 | $0.20 | $2.50 | $10.00 | $4.00 | $0.40 | $5.00 | $15.00 |
| gpt-6-luna | $0.10 | $0.01 | $0.125 | $0.50 | $0.20 | $0.02 | $0.25 | $0.75 |
| gpt-5.6-sol | $5.00 | $0.50 | $6.25 | $30.00 | $10.00 | $1.00 | $12.50 | $45.00 |
| gpt-5.6-terra | $2.00 | $0.20 | $2.50 | $12.00 | $4.00 | $0.40 | $5.00 | $18.00 |
| unrelated-provider | $1.00 | $0.10 | - | $2.00 | - | - | - | - |

### Batch pricing data

| Model | Short context input | Short context cached input | Short context cache writes | Short context output | Long context input | Long context cached input | Long context cache writes | Long context output |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| gpt-6-astra | $5.00 | $0.50 | $6.25 | $25.00 | $10.00 | $1.00 | $12.50 | $37.50 |
| gpt-5.6-sol | $2.50 | $0.25 | $3.125 | $15.00 | $5.00 | $0.50 | $6.25 | $22.50 |
`

func fixedFetch(markdown, etag string) PricingFetcher {
	return func(context.Context, string) (PricingResponse, error) {
		return PricingResponse{StatusCode: 200, Body: []byte(markdown), ETag: etag, LastModified: "Thu, 13 Aug 2026 04:57:43 GMT"}, nil
	}
}

func offline(context.Context, string) (PricingResponse, error) {
	return PricingResponse{}, errors.New("offline")
}

func TestParseStandardPricingTable(t *testing.T) {
	prices, err := ParseStandardPrices(catalogMarkdown)
	if err != nil {
		t.Fatal(err)
	}
	a := prices["gpt-6-astra"]
	wantDec(t, "in", &a.InputPerMillion, "10")
	wantDec(t, "cached", &a.CachedInputPerMillion, "1")
	wantDec(t, "write", a.CacheWritePerMillion, "12.5")
	wantDec(t, "out", &a.OutputPerMillion, "50")
	wantDec(t, "lc in", a.LongContextInputPerMillion, "20")
	wantDec(t, "lc cached", a.LongContextCachedInputPerMillion, "2")
	wantDec(t, "lc write", a.LongContextCacheWritePerMillion, "25")
	wantDec(t, "lc out", a.LongContextOutputPerMillion, "75")
	builtin, _ := PriceFor("gpt-6-sol")
	if !prices["gpt-6-sol"].Equal(builtin) || !prices["gpt-6-luna"].Equal(mustPrice("gpt-6-luna")) {
		t.Fatal("the bundled GPT-6 prices must equal the catalog's")
	}
	sol := prices["gpt-5.6-sol"]
	wantDec(t, "sol in", &sol.InputPerMillion, "5")
	wantDec(t, "sol cached", &sol.CachedInputPerMillion, "0.5")
	wantDec(t, "sol write", sol.CacheWritePerMillion, "6.25")
	wantDec(t, "sol lc out", sol.LongContextOutputPerMillion, "45")
	terra := prices["gpt-5.6-terra"]
	wantDec(t, "terra", &terra.InputPerMillion, "2")
	if _, ok := prices["unrelated-provider"]; ok {
		t.Fatal("non-Codex models are skipped")
	}
}

func mustPrice(m string) Price {
	p, ok := PriceFor(m)
	if !ok {
		panic(m)
	}
	return p
}

func TestParseStandardPricesErrors(t *testing.T) {
	for name, md := range map[string]string{
		"no heading": "# nothing\n",
		"no header":  "### Standard pricing data\n| a | b |\n",
		"empty":      "### Standard pricing data\n| Model | a | b | c | d | e | f | g | h |\n| --- | - | - | - | - | - | - | - | - |\n",
		"too high":   "### Standard pricing data\n| Model | a | b | c | d | e | f | g | h |\n| gpt-x | $10001 | - | - | $1 | - | - | - | - |\n",
	} {
		if _, err := ParseStandardPrices(md); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	// A row that is not a Codex model, lacks input/output, or has a context
	// note is handled without failing the whole table.
	md := "### Standard pricing data\n| Model | a | b | c | d | e | f | g | h |\n" +
		"| gpt-9 (<272K context length) | $1,000.50 | - | - | $2 | - | - | - | - |\n" +
		"| gpt-10 | - | - | - | $2 | - | - | - | - |\n"
	prices, err := ParseStandardPrices(md)
	if err != nil {
		t.Fatal(err)
	}
	if len(prices) != 1 {
		t.Fatalf("%v", prices)
	}
	p := prices["gpt-9"]
	wantDec(t, "thousands separator", &p.InputPerMillion, "1000.5")
	wantDec(t, "cached defaults to input", &p.CachedInputPerMillion, "1000.5")
}

func TestCatalogDownloadAndCache(t *testing.T) {
	for _, c := range []struct{ model, bundled string }{{"gpt-6-astra", "10.00"}, {"gpt-6-sol", "2.00"}, {"gpt-6-luna", "0.10"}} {
		t.Run(c.model, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "sub", "pricing.json")
			now := time.Unix(1_786_579_200, 0)
			md := strings.Replace(catalogMarkdown, "| "+c.model+" | $"+c.bundled+" |", "| "+c.model+" | $12.00 |", 1)
			p := NewPricingCatalogProvider(PricingCatalogConfig{CachePath: path, RefreshInterval: -1, Fetch: fixedFetch(md, `"official-etag"`)})
			book := p.PriceBookAt(context.Background(), now)
			million := Totals{TotalTokens: 1_000_000, UncachedInputTokens: 1_000_000}
			if !strings.HasPrefix(book.Version(), "openai-docs-") {
				t.Fatal(book.Version())
			}
			eq(t, "version", book.Version(), "openai-docs-official-etag")
			if cost, ok := book.Cost("gpt-5.6-terra", million); !ok || cost.String() != "2" {
				t.Fatalf("terra %v %v", cost, ok)
			}
			if cost, ok := book.Cost(c.model, million); !ok || cost.String() != "12" {
				t.Fatalf("%s %v %v", c.model, cost, ok)
			}
			if err := platform.VerifyOwnerOnly(path, 0o600); err != nil {
				t.Fatalf("cache permissions: %v", err)
			}
		})
	}
}

func TestBundledGPT6WorksOfflineWithNoCacheOrOlderCache(t *testing.T) {
	for _, older := range []bool{false, true} {
		path := filepath.Join(t.TempDir(), "pricing.json")
		fetchedAt := time.Unix(1_786_579_200, 0)
		if older {
			var kept []string
			for _, l := range strings.Split(catalogMarkdown, "\n") {
				if !strings.Contains(l, "| gpt-6-") {
					kept = append(kept, l)
				}
			}
			p := NewPricingCatalogProvider(PricingCatalogConfig{CachePath: path, Fetch: fixedFetch(strings.Join(kept, "\n"), "")})
			p.PriceBookAt(context.Background(), fetchedAt)
			if _, err := os.Stat(path); err != nil {
				t.Fatal(err)
			}
		}
		p := NewPricingCatalogProvider(PricingCatalogConfig{CachePath: path, Fetch: offline})
		book := p.PriceBookAt(context.Background(), fetchedAt.Add(24*time.Hour))
		totals := Totals{TotalTokens: 11_000, UncachedInputTokens: 8_000, CachedInputTokens: 2_000, OutputTokens: 1_000, Turns: 1, Threads: 1}
		for model, want := range map[string]string{"gpt-6-astra": "0.132", "gpt-6-sol": "0.0264", "gpt-6-luna": "0.00132"} {
			c, ok := book.Cost(model, totals)
			if !ok || c.String() != want {
				t.Errorf("older=%v %s: %v %v, want %s", older, model, c, ok, want)
			}
		}
		if !older {
			eq(t, "no cache falls back to the built-in table", book.Version(), PricingVersion)
		}
	}
}

func TestStaleCacheFallbackWhenRefreshFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pricing.json")
	at := time.Unix(1_786_579_200, 0)
	NewPricingCatalogProvider(PricingCatalogConfig{CachePath: path, RefreshInterval: -1, Fetch: fixedFetch(catalogMarkdown, `"cached-etag"`)}).
		PriceBookAt(context.Background(), at)
	p := NewPricingCatalogProvider(PricingCatalogConfig{CachePath: path, RefreshInterval: -1, Fetch: offline})
	book := p.PriceBookAt(context.Background(), at.Add(24*time.Hour))
	if !strings.HasPrefix(book.Version(), "openai-docs-") {
		t.Fatal(book.Version())
	}
	pr, ok := book.Price("gpt-5.6-sol")
	if !ok {
		t.Fatal("missing")
	}
	wantDec(t, "sol input", &pr.InputPerMillion, "5")
}

func TestCatalogFreshCacheSkipsFetchAndNotModifiedRenewsIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pricing.json")
	at := time.Unix(1_786_579_200, 0)
	var calls atomic.Int32
	var gotETag atomic.Value
	fetch := func(_ context.Context, etag string) (PricingResponse, error) {
		calls.Add(1)
		gotETag.Store(etag)
		if etag != "" {
			return PricingResponse{StatusCode: 304}, nil
		}
		return PricingResponse{StatusCode: 200, Body: []byte(catalogMarkdown), ETag: `"v1"`}, nil
	}
	p := NewPricingCatalogProvider(PricingCatalogConfig{CachePath: path, Fetch: fetch})
	ctx := context.Background()
	p.PriceBookAt(ctx, at)
	p.PriceBookAt(ctx, at.Add(time.Hour))
	eq(t, "calls within the interval", calls.Load(), 1)
	book := p.PriceBookAt(ctx, at.Add(25*time.Hour))
	eq(t, "calls after the interval", calls.Load(), 2)
	eq(t, "etag sent", gotETag.Load().(string), `"v1"`)
	eq(t, "version kept on 304", book.Version(), "openai-docs-v1")
	// the 304 moved fetchedAt forward, so one more hour needs no request
	p.PriceBookAt(ctx, at.Add(26*time.Hour))
	eq(t, "calls", calls.Load(), 2)
	// a new process reads the same file
	again := NewPricingCatalogProvider(PricingCatalogConfig{CachePath: path, Fetch: offline})
	eq(t, "persisted", again.PriceBookAt(ctx, at.Add(26*time.Hour)).Version(), "openai-docs-v1")
}

func TestCatalogRejectsBadResponses(t *testing.T) {
	at := time.Unix(1_786_579_200, 0)
	for name, resp := range map[string]PricingResponse{
		"500":         {StatusCode: 500, Body: []byte(catalogMarkdown)},
		"304 no body": {StatusCode: 304},
		"not utf8":    {StatusCode: 200, Body: []byte{0xff, 0xfe}},
		"no table":    {StatusCode: 200, Body: []byte("# hi")},
	} {
		path := filepath.Join(t.TempDir(), "p.json")
		p := NewPricingCatalogProvider(PricingCatalogConfig{CachePath: path, Fetch: func(context.Context, string) (PricingResponse, error) { return resp, nil }})
		eq(t, name, p.PriceBookAt(context.Background(), at).Version(), PricingVersion)
		if _, err := os.Stat(path); err == nil {
			t.Errorf("%s: a cache file was written", name)
		}
	}
}

func TestCatalogVersionToken(t *testing.T) {
	long := strings.Repeat("a", 60)
	etag, lm := `"W/abc--def"`, "Thu, 13 Aug 2026"
	cases := []struct {
		c    pricingCache
		want string
	}{
		{pricingCache{ETag: &etag}, "openai-docs-W-abc-def"},
		{pricingCache{LastModified: &lm}, "openai-docs-Thu-13-Aug-2026"},
		{pricingCache{ETag: &long}, "openai-docs-" + strings.Repeat("a", 48)},
		{pricingCache{FetchedAt: time.Unix(1_786_579_200, 0)}, "openai-docs-2026-08-13T00-00-00Z"},
	}
	for _, c := range cases {
		eq(t, c.want, c.c.version(), c.want)
	}
}

func TestPricingCacheMatchesSwiftFile(t *testing.T) {
	golden, err := os.ReadFile("testdata/pricing_cache.json")
	if err != nil {
		t.Fatal(err)
	}
	c, ok := decodePricingCache(golden)
	if !ok {
		t.Fatal("the Swift cache file must be accepted")
	}
	out, err := c.encode()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, golden) {
		t.Fatalf("encoded\n%s\nwant\n%s", out, golden)
	}
}

func TestPricingCacheRejections(t *testing.T) {
	good := `{"fetchedAt":"2026-06-30T12:00:00Z","prices":{"gpt-5.5":{"cachedInputPerMillion":0.5,"inputPerMillion":5,"outputPerMillion":30}},"schemaVersion":1,"sourceURL":"https:\/\/developers.openai.com\/api\/docs\/pricing.md"}`
	if _, ok := decodePricingCache([]byte(good)); !ok {
		t.Fatal("baseline must decode")
	}
	for name, bad := range map[string]string{
		"schema":     strings.Replace(good, `"schemaVersion":1`, `"schemaVersion":2`, 1),
		"source":     strings.Replace(good, "developers.openai.com", "example.com", 1),
		"empty":      strings.Replace(good, `"gpt-5.5":{"cachedInputPerMillion":0.5,"inputPerMillion":5,"outputPerMillion":30}`, "", 1),
		"no output":  strings.Replace(good, `,"outputPerMillion":30`, "", 1),
		"fractional": strings.Replace(good, "12:00:00Z", "12:00:00.5Z", 1),
		"string num": strings.Replace(good, `"inputPerMillion":5`, `"inputPerMillion":"5"`, 1),
		"garbage":    "{",
	} {
		if _, ok := decodePricingCache([]byte(bad)); ok {
			t.Errorf("%s: should be rejected", name)
		}
	}
}

func TestHTTPPricingFetcher(t *testing.T) {
	var gotAccept, gotINM, gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAccept, gotINM, gotUA = r.Header.Get("Accept"), r.Header.Get("If-None-Match"), r.Header.Get("User-Agent")
		w.Header().Set("ETag", `"e1"`)
		w.Write([]byte("body"))
	}))
	defer srv.Close()
	f := HTTPPricingFetcher(rewriteDoer{srv.URL})
	resp, err := f(context.Background(), `"old"`)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "status", resp.StatusCode, 200)
	eq(t, "body", string(resp.Body), "body")
	eq(t, "etag", resp.ETag, `"e1"`)
	eq(t, "accept", gotAccept, "text/markdown")
	eq(t, "if-none-match", gotINM, `"old"`)
	eq(t, "user agent", gotUA, "Quosparr/1")
}

// rewriteDoer sends every request to a test server instead.
type rewriteDoer struct{ base string }

func (d rewriteDoer) Do(r *http.Request) (*http.Response, error) {
	r2 := r.Clone(r.Context())
	u := *r.URL
	u.Scheme, u.Host = "http", strings.TrimPrefix(d.base, "http://")
	r2.URL = &u
	return http.DefaultClient.Do(r2)
}
