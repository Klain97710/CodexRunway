package usage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/licoy/quosparr/internal/platform"
	"github.com/licoy/quosparr/internal/usage/decimal"
)

// PricingSourceURL is the official pricing page the catalog is read from.
const PricingSourceURL = "https://developers.openai.com/api/docs/pricing.md"

// DefaultPricingRefreshInterval is how long a downloaded catalog is trusted.
const DefaultPricingRefreshInterval = 24 * time.Hour

const pricingCacheSchema = 1

// PricingCacheFileName is the catalog cache inside ~/.quosparr.
const PricingCacheFileName = "openai-pricing-v1.json"

// Pricing catalog errors.
var (
	ErrPricingInvalidResponse = errors.New("usage: invalid pricing response")
	ErrPricingMarkdown        = errors.New("usage: invalid pricing markdown")
	ErrPricingEmpty           = errors.New("usage: empty pricing catalog")
)

// PricingStatusError reports an unusable HTTP status.
type PricingStatusError struct{ Status int }

func (e *PricingStatusError) Error() string { return fmt.Sprintf("usage: pricing status %d", e.Status) }

// PricingResponse is what a PricingFetcher returns.
type PricingResponse struct {
	StatusCode   int
	Body         []byte
	ETag         string
	LastModified string
}

// PricingFetcher downloads the pricing page. etag, when not empty, is the
// validator of the cached copy (If-None-Match).
type PricingFetcher func(ctx context.Context, etag string) (PricingResponse, error)

// errNoPricingClient is returned by a fetcher built without a client.
var errNoPricingClient = errors.New("usage: pricing fetcher has no HTTP client")

// HTTPDoer is the part of *http.Client the default fetcher needs, so the
// application can pass a client that honours its proxy settings.
type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// HTTPPricingFetcher fetches PricingSourceURL with client and a 10 second
// timeout. client must come from the app's network context; a nil client
// fails every fetch rather than connecting without the user's proxy.
func HTTPPricingFetcher(client HTTPDoer) PricingFetcher {
	return func(ctx context.Context, etag string) (PricingResponse, error) {
		if client == nil {
			return PricingResponse{}, errNoPricingClient
		}
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, PricingSourceURL, nil)
		if err != nil {
			return PricingResponse{}, err
		}
		req.Header.Set("Accept", "text/markdown")
		req.Header.Set("User-Agent", "Quosparr/1")
		if etag != "" {
			req.Header.Set("If-None-Match", etag)
		}
		resp, err := client.Do(req)
		if err != nil {
			return PricingResponse{}, err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		if err != nil {
			return PricingResponse{}, err
		}
		return PricingResponse{
			StatusCode:   resp.StatusCode,
			Body:         body,
			ETag:         resp.Header.Get("ETag"),
			LastModified: resp.Header.Get("Last-Modified"),
		}, nil
	}
}

const standardHeading = "### Standard pricing data"

func isNewlineRune(r rune) bool {
	switch r {
	case '\n', '\r', '\v', '\f', 0x85, 0x2028, 0x2029:
		return true
	}
	return false
}

// ParseStandardPrices reads the "Standard pricing data" table of the official
// pricing Markdown and returns the Codex-compatible models (gpt-*, codex-*).
func ParseStandardPrices(markdown string) (map[string]Price, error) {
	inside, sawHeader := false, false
	result := map[string]Price{}
	for _, raw := range strings.FieldsFunc(markdown, isNewlineRune) {
		line := strings.TrimFunc(raw, isSwiftWhitespace)
		if line == standardHeading {
			inside = true
			continue
		}
		if !inside {
			continue
		}
		if strings.HasPrefix(line, "### ") {
			break
		}
		if !strings.HasPrefix(line, "|") {
			continue
		}
		cells := tableCells(line)
		if len(cells) != 9 {
			continue
		}
		if cells[0] == "Model" {
			sawHeader = true
			continue
		}
		if strings.Trim(cells[0], "-:") == "" {
			continue
		}
		if !sawHeader {
			continue
		}
		model := catalogModelID(cells[0])
		if !strings.HasPrefix(model, "gpt-") && !strings.HasPrefix(model, "codex-") {
			continue
		}
		input, ok1 := catalogDecimal(cells[1])
		output, ok2 := catalogDecimal(cells[4])
		if !ok1 || !ok2 {
			continue
		}
		cached, ok := catalogDecimal(cells[2])
		if !ok {
			cached = input
		}
		p := Price{InputPerMillion: input, CachedInputPerMillion: cached, OutputPerMillion: output}
		p.CacheWritePerMillion = catalogOptional(cells[3])
		p.LongContextInputPerMillion = catalogOptional(cells[5])
		p.LongContextCachedInputPerMillion = catalogOptional(cells[6])
		p.LongContextCacheWritePerMillion = catalogOptional(cells[7])
		p.LongContextOutputPerMillion = catalogOptional(cells[8])
		if !validPrice(p) {
			return nil, ErrPricingMarkdown
		}
		result[model] = p
	}
	if !inside || !sawHeader {
		return nil, ErrPricingMarkdown
	}
	if len(result) == 0 {
		return nil, ErrPricingEmpty
	}
	return result, nil
}

func tableCells(line string) []string {
	parts := strings.Split(line, "|")
	if len(parts) < 2 {
		return nil
	}
	parts = parts[1 : len(parts)-1]
	out := make([]string, len(parts))
	for i, p := range parts {
		out[i] = strings.TrimFunc(p, isSwiftWhitespace)
	}
	return out
}

// catalogModelID keeps the first space-separated word ("gpt-5.5 (<272K
// context length)" is just gpt-5.5), lower-cased.
func catalogModelID(v string) string {
	word := strings.TrimLeft(v, " ")
	if i := strings.IndexByte(word, ' '); i >= 0 {
		word = word[:i]
	}
	if word == "" {
		word = v
	}
	return strings.ToLower(word)
}

func catalogDecimal(v string) (decimal.Decimal, bool) {
	v = strings.ReplaceAll(strings.ReplaceAll(v, "$", ""), ",", "")
	v = strings.TrimFunc(v, isSwiftWhitespace)
	if v == "" || v == "-" {
		return decimal.Decimal{}, false
	}
	d, err := decimal.Parse(v)
	return d, err == nil
}

func catalogOptional(v string) *decimal.Decimal {
	d, ok := catalogDecimal(v)
	if !ok {
		return nil
	}
	return &d
}

func validPrice(p Price) bool {
	limit := decimal.FromInt(10_000)
	check := func(d decimal.Decimal) bool { return d.Sign() >= 0 && !limit.Less(d) }
	for _, d := range []*decimal.Decimal{&p.InputPerMillion, &p.CachedInputPerMillion, p.CacheWritePerMillion,
		&p.OutputPerMillion, p.LongContextInputPerMillion, p.LongContextCachedInputPerMillion,
		p.LongContextCacheWritePerMillion, p.LongContextOutputPerMillion} {
		if d != nil && !check(*d) {
			return false
		}
	}
	return true
}

// --- cache file -------------------------------------------------------------

// pricingCache is the last-known-good catalog (openai-pricing-v1.json). The
// file layout is the one the Swift app writes, so each can read the other's.
type pricingCache struct {
	FetchedAt    time.Time
	ETag         *string
	LastModified *string
	Prices       map[string]Price
}

func (c pricingCache) version() string {
	var source string
	switch {
	case c.ETag != nil:
		source = *c.ETag
	case c.LastModified != nil:
		source = *c.LastModified
	default:
		source = c.FetchedAt.UTC().Format("2006-01-02T15:04:05Z")
	}
	var b strings.Builder
	dash := true // collapses runs and trims the front
	for _, r := range source {
		if unicode.IsLetter(r) || unicode.IsMark(r) || unicode.IsNumber(r) {
			b.WriteRune(r)
			dash = false
		} else if !dash {
			b.WriteByte('-')
			dash = true
		}
	}
	compact := strings.TrimRight(b.String(), "-")
	if utf8.RuneCountInString(compact) > 48 {
		compact = string([]rune(compact)[:48])
	}
	return onlinePricingVersionPrefix + compact
}

func (c pricingCache) priceBook() PriceBook {
	merged := BuiltInPrices()
	for k, v := range c.Prices {
		merged[k] = v
	}
	equivalent, ok := LookupPrice(merged, "gpt-5.6-sol")
	if !ok {
		equivalent = EquivalentPrice()
	}
	return NewPriceBook(c.version(), merged, equivalent)
}

func priceMap(p Price) map[string]any {
	m := map[string]any{
		"inputPerMillion":       p.InputPerMillion,
		"cachedInputPerMillion": p.CachedInputPerMillion,
		"outputPerMillion":      p.OutputPerMillion,
	}
	for k, v := range map[string]*decimal.Decimal{
		"cacheWritePerMillion":             p.CacheWritePerMillion,
		"longContextInputPerMillion":       p.LongContextInputPerMillion,
		"longContextCachedInputPerMillion": p.LongContextCachedInputPerMillion,
		"longContextCacheWritePerMillion":  p.LongContextCacheWritePerMillion,
		"longContextOutputPerMillion":      p.LongContextOutputPerMillion,
	} {
		if v != nil {
			m[k] = *v
		}
	}
	return m
}

// encode writes the cache with sorted keys and escaped slashes, as
// Foundation's JSONEncoder with .sortedKeys does.
func (c pricingCache) encode() ([]byte, error) {
	prices := make(map[string]any, len(c.Prices))
	for k, v := range c.Prices {
		prices[k] = priceMap(v)
	}
	doc := map[string]any{
		"schemaVersion": pricingCacheSchema,
		"sourceURL":     PricingSourceURL,
		"fetchedAt":     c.FetchedAt.UTC().Format("2006-01-02T15:04:05Z"),
		"prices":        prices,
	}
	if c.ETag != nil {
		doc["eTag"] = *c.ETag
	}
	if c.LastModified != nil {
		doc["lastModified"] = *c.LastModified
	}
	return marshalSwiftJSON(doc)
}

// marshalSwiftJSON marshals v compactly without HTML escaping, and escapes
// "/" as "\/" like Foundation.
func marshalSwiftJSON(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	out := bytes.TrimSuffix(buf.Bytes(), []byte("\n"))
	return escapeSlashes(out), nil
}

// escapeSlashes rewrites "/" inside JSON strings; outside strings it never
// occurs, so a plain replace is exact.
func escapeSlashes(b []byte) []byte { return bytes.ReplaceAll(b, []byte("/"), []byte(`\/`)) }

var iso8601Seconds = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(Z|[+-]\d{2}:\d{2})$`)

type priceFile struct {
	Input         *decimal.Decimal `json:"inputPerMillion"`
	Cached        *decimal.Decimal `json:"cachedInputPerMillion"`
	CacheWrite    *decimal.Decimal `json:"cacheWritePerMillion"`
	Output        *decimal.Decimal `json:"outputPerMillion"`
	LongInput     *decimal.Decimal `json:"longContextInputPerMillion"`
	LongCached    *decimal.Decimal `json:"longContextCachedInputPerMillion"`
	LongCacheWrte *decimal.Decimal `json:"longContextCacheWritePerMillion"`
	LongOutput    *decimal.Decimal `json:"longContextOutputPerMillion"`
}

type cacheFile struct {
	SchemaVersion *int                  `json:"schemaVersion"`
	SourceURL     *string               `json:"sourceURL"`
	FetchedAt     *string               `json:"fetchedAt"`
	ETag          *string               `json:"eTag"`
	LastModified  *string               `json:"lastModified"`
	Prices        map[string]*priceFile `json:"prices"`
}

// decodePricingCache validates a cache file. Anything unexpected (wrong
// schema, another source, no prices, a missing required price) rejects the
// whole file, so a damaged cache is never half used.
func decodePricingCache(data []byte) (pricingCache, bool) {
	var f cacheFile
	if err := json.Unmarshal(data, &f); err != nil {
		return pricingCache{}, false
	}
	if f.SchemaVersion == nil || *f.SchemaVersion != pricingCacheSchema ||
		f.SourceURL == nil || *f.SourceURL != PricingSourceURL ||
		f.FetchedAt == nil || !iso8601Seconds.MatchString(*f.FetchedAt) || len(f.Prices) == 0 {
		return pricingCache{}, false
	}
	at, err := time.Parse(time.RFC3339, *f.FetchedAt)
	if err != nil {
		return pricingCache{}, false
	}
	prices := make(map[string]Price, len(f.Prices))
	for model, p := range f.Prices {
		if p == nil || p.Input == nil || p.Cached == nil || p.Output == nil {
			return pricingCache{}, false
		}
		prices[model] = Price{
			InputPerMillion: *p.Input, CachedInputPerMillion: *p.Cached, CacheWritePerMillion: p.CacheWrite,
			OutputPerMillion:           *p.Output,
			LongContextInputPerMillion: p.LongInput, LongContextCachedInputPerMillion: p.LongCached,
			LongContextCacheWritePerMillion: p.LongCacheWrte, LongContextOutputPerMillion: p.LongOutput,
		}
	}
	return pricingCache{FetchedAt: at, ETag: f.ETag, LastModified: f.LastModified, Prices: prices}, true
}

// --- provider ---------------------------------------------------------------

// PricingCatalogConfig configures a PricingCatalogProvider.
type PricingCatalogConfig struct {
	// CachePath is the cache file (normally ~/.quosparr/openai-pricing-v1.json).
	CachePath string
	// RefreshInterval defaults to DefaultPricingRefreshInterval. A negative
	// value always refreshes.
	RefreshInterval time.Duration
	// Fetch should be HTTPPricingFetcher with a client from the app's network
	// context. When nil, nothing is fetched: the cache and built-in table serve.
	Fetch PricingFetcher
	// Now defaults to time.Now.
	Now func() time.Time
}

// PricingCatalogProvider serves the price book: the downloaded catalog laid
// over the built-in table, refreshed at most once per interval, falling back
// to the last cache and finally to the built-in table. It never fails: an
// unreachable or unparsable page is not an error for the caller. It
// implements PriceSource.
type PricingCatalogProvider struct {
	cfg PricingCatalogConfig

	mu    sync.Mutex
	cache *pricingCache
}

// NewPricingCatalogProvider loads the cache file, if any and valid.
func NewPricingCatalogProvider(cfg PricingCatalogConfig) *PricingCatalogProvider {
	if cfg.RefreshInterval == 0 {
		cfg.RefreshInterval = DefaultPricingRefreshInterval
	}
	if cfg.Fetch == nil {
		cfg.Fetch = HTTPPricingFetcher(nil)
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	p := &PricingCatalogProvider{cfg: cfg}
	if cfg.CachePath != "" {
		if data, err := os.ReadFile(cfg.CachePath); err == nil {
			if c, ok := decodePricingCache(data); ok {
				p.cache = &c
			}
		}
	}
	return p
}

// PriceBook implements PriceSource using the configured clock.
func (p *PricingCatalogProvider) PriceBook(ctx context.Context) PriceBook {
	return p.PriceBookAt(ctx, p.cfg.Now())
}

// PriceBookAt returns the price book as of now.
func (p *PricingCatalogProvider) PriceBookAt(ctx context.Context, now time.Time) PriceBook {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cache != nil && p.cfg.RefreshInterval > 0 && now.Sub(p.cache.FetchedAt) < p.cfg.RefreshInterval {
		return p.cache.priceBook()
	}
	if book, err := p.refresh(ctx, now); err == nil {
		return book
	}
	if p.cache != nil {
		return p.cache.priceBook()
	}
	return CurrentPriceBook()
}

func (p *PricingCatalogProvider) refresh(ctx context.Context, now time.Time) (PriceBook, error) {
	etag := ""
	if p.cache != nil && p.cache.ETag != nil {
		etag = *p.cache.ETag
	}
	resp, err := p.cfg.Fetch(ctx, etag)
	if err != nil {
		return PriceBook{}, err
	}
	if resp.StatusCode == http.StatusNotModified && p.cache != nil {
		c := *p.cache
		c.FetchedAt = now
		if err := p.save(c); err != nil {
			return PriceBook{}, err
		}
		p.cache = &c
		return c.priceBook(), nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return PriceBook{}, &PricingStatusError{Status: resp.StatusCode}
	}
	if !utf8.Valid(resp.Body) {
		return PriceBook{}, ErrPricingMarkdown
	}
	prices, err := ParseStandardPrices(string(resp.Body))
	if err != nil {
		return PriceBook{}, err
	}
	c := pricingCache{FetchedAt: now, Prices: prices}
	if resp.ETag != "" {
		c.ETag = &resp.ETag
	}
	if resp.LastModified != "" {
		c.LastModified = &resp.LastModified
	}
	if err := p.save(c); err != nil {
		return PriceBook{}, err
	}
	p.cache = &c
	return c.priceBook(), nil
}

func (p *PricingCatalogProvider) save(c pricingCache) error {
	if p.cfg.CachePath == "" {
		return nil
	}
	data, err := c.encode()
	if err != nil {
		return err
	}
	return platform.WriteFileAtomic(p.cfg.CachePath, data, 0o600)
}
